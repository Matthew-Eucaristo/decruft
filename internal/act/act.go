// Package act carries out a plan, one step at a time: each built-in item is
// re-read and must match what the user saw, then its official command runs
// or its folder moves to the trash; family items go back to their tool as
// one plan. Free space is measured before and after every step.
package act

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Matthew-Eucaristo/decruft/internal/family"
	"github.com/Matthew-Eucaristo/decruft/internal/item"
	"github.com/Matthew-Eucaristo/decruft/internal/module"
	"github.com/Matthew-Eucaristo/decruft/internal/pathx"
	"github.com/Matthew-Eucaristo/decruft/internal/trash"
)

// Mode is how folders are removed. Official commands delete by their own
// design in every mode except DryRun.
type Mode int

const (
	Trash Mode = iota
	DryRun
	Permanent
)

func (m Mode) family() family.Mode {
	switch m {
	case DryRun:
		return family.DryRun
	case Permanent:
		return family.Permanent
	}
	return family.Trash
}

// How is what happened to an item.
type How string

const (
	Trashed   How = "trashed"   // moved to the trash: freed when it is emptied
	Deleted   How = "deleted"   // folder deleted (--permanent)
	Freed     How = "freed"     // an official command removed it
	Delegated How = "delegated" // its family tool removed it
	Dry       How = "dry_run"
	Refused   How = "refused" // rules said no, or it changed; nothing happened
	Failed    How = "failed"
)

// Outcome is one item's result.
type Outcome struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Source   string        `json:"source"`
	Category item.Category `json:"category"`
	How      How           `json:"how"`
	Kind     item.Kind     `json:"kind"`
	What     string        `json:"what"` // the command, or what the tool did
	Estimate int64         `json:"estimated_bytes"`
	// Freed is the measured change in free space, -1 when unknown. For a
	// family batch it is split by estimate across its items.
	Freed int64  `json:"freed_bytes_measured"`
	Error string `json:"error,omitempty"`
	// Claimed marks a command that reported success while the disk did not
	// gain what it promised (yet): Docker Desktop and simctl do this.
	Claimed bool `json:"claimed_not_freed,omitempty"`
}

// Step is one unit of work: a built-in item, or a family tool's batch.
type Step struct {
	Tool  string // family tool name, empty for built-in
	Items []item.Item
}

// Executor runs steps.
type Executor struct {
	Mode    Mode
	Home    string
	Env     *module.Env
	Modules []module.Module
	Family  *family.Family
	Tools   map[string]family.Tool
	// Run runs an official command; Free measures available space;
	// MoveToTrash defaults to the OS trash.
	Run         func(ctx context.Context, extraEnv []string, argv []string) error
	Free        func(path string) int64
	MoveToTrash func(path string) error
}

// Plan orders the selection into steps: built-in items by priority, then
// one batch per family tool. Items decruft must not act on are kept as
// their own steps so they are reported as refused, not silently dropped.
func Plan(items []item.Item) []Step {
	var builtin []item.Item
	byTool := map[string][]item.Item{}
	var tools []string
	for _, it := range items {
		if it.Action.Kind == item.Delegate && it.Delegate != nil {
			if byTool[it.Delegate.Tool] == nil {
				tools = append(tools, it.Delegate.Tool)
			}
			byTool[it.Delegate.Tool] = append(byTool[it.Delegate.Tool], it)
			continue
		}
		builtin = append(builtin, it)
	}
	sort.SliceStable(builtin, func(i, j int) bool { return builtin[i].Priority < builtin[j].Priority })
	var steps []Step
	for _, it := range builtin {
		steps = append(steps, Step{Items: []item.Item{it}})
	}
	for _, t := range tools {
		steps = append(steps, Step{Tool: t, Items: byTool[t]})
	}
	return steps
}

// Do carries out one step.
func (e *Executor) Do(ctx context.Context, s Step) []Outcome {
	if s.Tool != "" {
		return e.delegate(ctx, s)
	}
	return []Outcome{e.one(ctx, s.Items[0])}
}

func outcome(it item.Item, how How) Outcome {
	o := Outcome{ID: it.ID, Name: it.Name, Source: it.Source, Category: it.Category, How: how, Kind: it.Action.Kind,
		Estimate: max(it.Size(), 0), Freed: -1}
	switch {
	case len(it.Action.Argv) > 0:
		o.What = it.Action.CommandLine()
	case it.Delegate != nil:
		o.What = it.Delegate.Text
	case it.Path != "":
		o.What = it.Path
	}
	return o
}

func refuse(it item.Item, why string) Outcome {
	o := outcome(it, Refused)
	o.Error = why
	return o
}

// allowed says why decruft must not act on an item, or "".
func allowed(it item.Item) string {
	if !it.Selectable() || (it.Action.Kind != item.Command && it.Action.Kind != item.Trash) {
		return "decruft does not act on this item (" + string(it.Level) + ", " + string(it.Action.Kind) + ")"
	}
	if it.Action.Kind == item.Command && (len(it.Action.Argv) == 0 || filepath.Base(it.Action.Argv[0]) == "sudo") {
		return "commands that need sudo are shown, never run"
	}
	return ""
}

// check re-reads the item's module and returns the fresh copy, or why the
// item may not run. Callers act on the fresh copy only, so nothing in the
// item they were handed (its level, its command) is trusted.
func (e *Executor) check(ctx context.Context, it item.Item) (item.Item, string) {
	if why := allowed(it); why != "" {
		return it, why
	}
	var mod *module.Module
	for i := range e.Modules {
		if e.Modules[i].ID == it.Source {
			mod = &e.Modules[i]
		}
	}
	if mod == nil {
		return it, "no built-in module " + it.Source
	}
	// Re-read without sizing: identity and state are what the fingerprint
	// covers, and sizing again would make every step slow.
	env := *e.Env
	env.Measure = func(p string) module.Measured {
		if _, err := os.Lstat(p); err != nil {
			return module.Measured{Missing: true}
		}
		return module.Measured{Bytes: 1, Frees: 1}
	}
	for _, fresh := range mod.Scan(ctx, &env).Items {
		fresh.Seal()
		if fresh.ID != it.ID {
			continue
		}
		if fresh.Fingerprint != it.Fingerprint {
			if !fresh.Level.Allowed() && len(fresh.Reasons) > 0 {
				return it, "changed since the scan: " + fresh.Reasons[len(fresh.Reasons)-1].Detail
			}
			return it, "changed since the scan; scan again"
		}
		if why := allowed(fresh); why != "" {
			return it, why
		}
		return fresh, ""
	}
	return it, "no longer there"
}

func (e *Executor) one(ctx context.Context, it item.Item) Outcome {
	if ctx.Err() != nil {
		return refuse(it, "cancelled")
	}
	fresh, why := e.check(ctx, it)
	if why != "" {
		return refuse(it, why)
	}
	// Act on what the module says now; keep the size the user saw.
	fresh.Bytes, fresh.Frees, fresh.UpTo = it.Bytes, it.Frees, it.UpTo
	it = fresh
	if it.Action.Kind == item.Trash {
		if err := e.guard(it.Path); err != nil {
			return refuse(it, err.Error())
		}
	}
	if e.Mode == DryRun {
		return outcome(it, Dry)
	}
	before := e.Free(e.Home)
	var o Outcome
	switch it.Action.Kind {
	case item.Command:
		o = outcome(it, Freed)
		if err := e.Run(ctx, it.Action.Env, it.Action.Argv); err != nil {
			o.How, o.Error = Failed, err.Error()
		}
	case item.Trash:
		o = outcome(it, Trashed)
		var err error
		if e.Mode == Permanent {
			o.How = Deleted
			err = os.RemoveAll(it.Path)
		} else if e.MoveToTrash != nil {
			err = e.MoveToTrash(it.Path)
		} else {
			err = trash.Move(it.Path)
		}
		if err != nil {
			o.How, o.Error = Failed, err.Error()
		}
	}
	if after := e.Free(e.Home); before >= 0 && after >= 0 {
		o.Freed = after - before
	}
	if o.How == Freed && o.Estimate > 50e6 && o.Freed >= 0 && o.Freed < o.Estimate/10 {
		o.Claimed = true
	}
	return o
}

// guard refuses folder moves outside the home folder, of the home folder or
// a top-level folder in it, and of symlinks.
func (e *Executor) guard(path string) error {
	if path == "" || !filepath.IsAbs(path) {
		return errors.New("not an absolute path")
	}
	if e.Home == "" || !pathx.Within(e.Home, path) {
		return errors.New("outside the home folder")
	}
	rel, err := filepath.Rel(e.Home, path)
	if err != nil || len(strings.Split(rel, string(filepath.Separator))) < 3 {
		return errors.New("too close to the home folder to move")
	}
	fi, err := os.Lstat(path)
	switch {
	case err != nil:
		return errors.New("no longer there")
	case fi.Mode()&os.ModeSymlink != 0:
		return errors.New("is a symlink")
	case !fi.IsDir():
		return errors.New("not a folder")
	}
	return nil
}

func (e *Executor) delegate(ctx context.Context, s Step) []Outcome {
	var outs []Outcome
	t, ok := e.Tools[s.Tool]
	if !ok || !t.Contract {
		for _, it := range s.Items {
			outs = append(outs, refuse(it, s.Tool+" is not available to hand this to"))
		}
		return outs
	}
	if ctx.Err() != nil {
		for _, it := range s.Items {
			outs = append(outs, refuse(it, "cancelled"))
		}
		return outs
	}
	before := e.Free(e.Home)
	results, err := e.Family.Apply(ctx, t, s.Items, e.Mode.family())
	after := e.Free(e.Home)
	byID := map[string]family.Result{}
	for _, r := range results {
		byID[r.ID] = r
	}
	var doneEst int64
	for _, it := range s.Items {
		o := outcome(it, Delegated)
		r, found := byID[it.Delegate.ID]
		switch {
		case err != nil:
			o.How, o.Error = Failed, err.Error()
		case !found:
			o.How, o.Error = Failed, s.Tool+" did not report this item"
		case r.Status == "done":
			if r.Estimated > 0 {
				o.Estimate = r.Estimated
			}
			doneEst += o.Estimate
		case r.Status == "dry_run":
			o.How = Dry
		case r.Status == "refused":
			o.How, o.Error = Refused, r.Error
		default:
			o.How, o.Error = Failed, r.Error
		}
		if o.Error == "" && o.How != Delegated && o.How != Dry {
			o.Error = strings.Join(r.Reasons, ", ")
		}
		outs = append(outs, o)
	}
	// Split the measured change across the done items by their estimates.
	if before >= 0 && after >= 0 && doneEst > 0 {
		for i := range outs {
			if outs[i].How == Delegated {
				outs[i].Freed = (after - before) * outs[i].Estimate / doneEst
			}
		}
	}
	return outs
}

// RunCommand runs an official cleanup command without a terminal: stdin is
// empty so nothing can prompt, and the tail of its output explains a failure.
func RunCommand(ctx context.Context, extra []string, argv []string) error {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env = append(append(os.Environ(), module.Quiet...), extra...)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		tail := strings.TrimSpace(out.String())
		if lines := strings.Split(tail, "\n"); len(lines) > 2 {
			tail = strings.Join(lines[len(lines)-2:], " / ")
		}
		if tail != "" {
			return fmt.Errorf("%s: %s", filepath.Base(argv[0]), tail)
		}
		return fmt.Errorf("%s: %w", filepath.Base(argv[0]), err)
	}
	return nil
}
