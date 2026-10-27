// Command decruft shows everything a developer's machine accumulates, explains
// what macOS hides, and frees space only through each tool's official
// command or the family's own cleaners. Run `decruft -h` for usage.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"

	"github.com/Matthew-Eucaristo/decruft/internal/act"
	"github.com/Matthew-Eucaristo/decruft/internal/disk"
	"github.com/Matthew-Eucaristo/decruft/internal/family"
	"github.com/Matthew-Eucaristo/decruft/internal/format"
	"github.com/Matthew-Eucaristo/decruft/internal/inventory"
	"github.com/Matthew-Eucaristo/decruft/internal/item"
	"github.com/Matthew-Eucaristo/decruft/internal/module"
	"github.com/Matthew-Eucaristo/decruft/internal/report"
	"github.com/Matthew-Eucaristo/decruft/internal/ui"
)

var version = "" // set by -ldflags at release time

const usage = `decruft - see and reclaim what a developer's machine accumulates.

Usage:
  decruft [flags] [root...]     roots are passed to fell, nodekill and flutterkill
                                (default: their own, ~/dev)

Interactive by default. Folders go to the Trash unless --permanent; caches are
removed by their own tool's official command; family items are handed back to
fell, nodekill or flutterkill. --list, --json and --dry-run never change anything.

Flags:
`

// Exit codes, shared with the family.
const (
	exitUsage   = 2
	exitRefused = 3
	exitPartial = 4
)

type exitError struct{ code int }

func (e exitError) Error() string { return fmt.Sprintf("exit %d", e.code) }

func main() {
	err := run(os.Args[1:], os.Stdout, os.Stderr)
	var ee exitError
	switch {
	case errors.As(err, &ee):
		os.Exit(ee.code)
	case errors.Is(err, errUsage):
		os.Exit(exitUsage)
	case err != nil:
		fmt.Fprintln(os.Stderr, "decruft:", err)
		os.Exit(1)
	}
}

var errUsage = errors.New("usage")

// deps are the machine decruft talks to; tests replace them.
type deps struct {
	env    *module.Env
	fam    *family.Family
	run    func(ctx context.Context, extra []string, argv []string) error
	free   func(string) int64
	disk   func(ctx context.Context, home string) disk.Info
	tty    bool
	modset []module.Module
}

func systemDeps() deps {
	env := module.System()
	return deps{
		env:    env,
		fam:    &family.Family{Run: family.Exec, LookPath: env.LookPath, Home: env.Home},
		run:    act.RunCommand,
		free:   disk.Free,
		disk:   disk.Read,
		tty:    term.IsTerminal(os.Stdout.Fd()) && term.IsTerminal(os.Stdin.Fd()),
		modset: module.All(),
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	return runWith(args, stdout, stderr, systemDeps())
}

func runWith(args []string, stdout, stderr io.Writer, d deps) error {
	flags := flag.NewFlagSet("decruft", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var (
		dryRun    = flags.Bool("dry-run", false, "never change anything; show what would be freed")
		permanent = flags.Bool("permanent", false, "delete folders permanently instead of moving them to the Trash")
		jsonOut   = flags.Bool("json", false, "print the scan as JSON and exit (never changes anything)")
		list      = flags.Bool("list", false, "print the scan as a table and exit (default when not a terminal)")
		yes       = flags.Bool("yes", false, "non-interactive: clean every safe item (review items are never included)")
		noFamily  = flags.Bool("no-family", false, "skip fell, nodekill and flutterkill")
		showVer   = flags.Bool("version", false, "print version")
	)
	flags.Usage = func() {
		fmt.Fprint(flags.Output(), usage)
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return errUsage
	}
	if *showVer {
		fmt.Fprintln(stdout, "decruft", buildVersion())
		return nil
	}
	if *dryRun && *permanent {
		fmt.Fprintln(stderr, "decruft: --dry-run and --permanent are mutually exclusive")
		return errUsage
	}
	var roots []string
	for _, r := range flags.Args() {
		abs, err := filepath.Abs(r)
		if err != nil {
			return err
		}
		if fi, err := os.Stat(abs); err != nil || !fi.IsDir() {
			return fmt.Errorf("%s is not a directory", r)
		}
		roots = append(roots, abs)
	}

	scanner := &inventory.Scanner{Env: d.env, Family: d.fam, Modules: d.modset, Roots: roots, NoFamily: *noFamily}
	mode := act.Trash
	switch {
	case *dryRun:
		mode = act.DryRun
	case *permanent:
		mode = act.Permanent
	}
	exec := &act.Executor{Mode: mode, Home: d.env.Home, Env: d.env, Modules: d.modset, Family: d.fam,
		Tools: map[string]family.Tool{}, Run: d.run, Free: d.free}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch {
	case *jsonOut || *list || (!d.tty && !*yes):
		s := collect(ctx, scanner, d)
		if *jsonOut {
			return report.JSON(stdout, s)
		}
		report.List(stdout, s)
		return nil
	case *yes:
		return cleanSafe(ctx, stdout, scanner, exec, d)
	}

	m := ui.New(ui.Options{Scanner: scanner, Exec: exec, Home: d.env.Home, Version: buildVersion(), Disk: d.disk})
	_, err := tea.NewProgram(m).Run()
	// The receipt stays in the terminal after the full-screen view closes.
	m.Receipt().Print(stdout)
	return err
}

func collect(ctx context.Context, sc *inventory.Scanner, d deps) report.Scan {
	sources, items, warns := sc.Collect(ctx)
	return report.Scan{Disk: d.disk(ctx, d.env.Home), Sources: sources, Items: items, Warnings: warns,
		Home: d.env.Home, Version: buildVersion()}
}

// cleanSafe removes every safe item without asking, one step at a time.
// Cancelling (ctrl+c) stops before the next step.
func cleanSafe(ctx context.Context, w io.Writer, sc *inventory.Scanner, ex *act.Executor, d deps) error {
	start := time.Now()
	sources, items, warns := sc.Collect(ctx)
	for _, s := range sources {
		if s.Tool != nil {
			ex.Tools[s.Name] = *s.Tool
		}
	}
	for _, warn := range warns {
		fmt.Fprintln(w, "warning: "+warn)
	}
	var pick []item.Item
	for _, it := range items {
		if it.Selectable() && it.Level == item.Safe {
			pick = append(pick, it)
		}
	}
	if len(pick) == 0 {
		fmt.Fprintln(w, "Nothing safe to clean.")
		return nil
	}
	r := report.Receipt{Before: d.free(d.env.Home)}
	for _, st := range act.Plan(pick) {
		for _, o := range ex.Do(ctx, st) {
			fmt.Fprintf(w, "%-10s %9s  %s\n", o.How, format.Bytes(o.Estimate), o.Name)
			r.Outcomes = append(r.Outcomes, o)
		}
		for _, it := range st.Items {
			r.Regrows = r.Regrows || it.Regrows
		}
	}
	r.After, r.Took = d.free(d.env.Home), time.Since(start)
	fmt.Fprintln(w)
	r.Print(w)
	_, _, _, _, _, refused, failed, _ := r.Buckets()
	switch {
	case len(failed) > 0:
		return exitError{exitPartial}
	case len(refused) > 0:
		return exitError{exitRefused}
	}
	return nil
}

func buildVersion() string {
	if version != "" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "dev"
}
