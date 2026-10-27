package module

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Matthew-Eucaristo/decruft/internal/item"
)

// Homebrew: `brew autoremove` (formulae installed only as dependencies that
// nothing needs any more), then `brew cleanup --prune=all` (old versions,
// downloads and logs). Both are measured with their own -n dry run.
func Homebrew() Module {
	return Module{ID: "homebrew", Name: "Homebrew", Scan: scanHomebrew}
}

var brewFrees = regexp.MustCompile(`would free approximately ([0-9.,]+\s*[KMGT]?i?B)`)

func scanHomebrew(ctx context.Context, env *Env) Result {
	var r Result
	brew, err := env.LookPath("brew")
	if err != nil {
		return r
	}
	if out, err := env.Output(ctx, nil, brew, "autoremove", "-n"); err != nil {
		r.Warnings = append(r.Warnings, "brew autoremove -n: "+err.Error())
	} else if names := autoremoveNames(string(out)); len(names) > 0 {
		cellar := ""
		if c, err := env.Output(ctx, nil, brew, "--cellar"); err == nil {
			cellar = firstLine(c)
		}
		var bytes, frees int64
		for _, n := range names {
			if cellar == "" {
				bytes, frees = -1, -1
				break
			}
			m := env.Measure(filepath.Join(cellar, n))
			bytes, frees = bytes+m.Bytes, frees+m.Frees
		}
		it, _ := build(env, spec{
			source: "homebrew", category: item.Homebrew, kind: "brew-autoremove",
			name: fmt.Sprintf("Unused dependencies (%d)", len(names)),
			argv: []string{brew, "autoremove"}, env: Quiet, level: item.Review,
			explain: "Formulae installed only as dependencies that nothing installed needs any more: " + list(names, 6) + ".",
			regen:   "brew install <name>, if you used one directly", priority: -1,
		}, strings.Join(names, ","))
		it.Bytes, it.Frees = bytes, frees
		it.Add("uninstalls", item.Review, "uninstalls formulae; check the list in case you use one directly")
		r.Items = append(r.Items, it)
	}
	out, err := env.Output(ctx, nil, brew, "cleanup", "--prune=all", "-n")
	if err != nil {
		r.Warnings = append(r.Warnings, "brew cleanup -n: "+err.Error())
		return r
	}
	n := strings.Count(string(out), "Would remove:")
	m := brewFrees.FindStringSubmatch(string(out))
	if n == 0 || m == nil {
		return r
	}
	it, _ := build(env, spec{
		source: "homebrew", category: item.Homebrew, kind: "brew-cleanup",
		name: "Old versions, downloads and logs",
		argv: []string{brew, "cleanup", "--prune=all"}, env: Quiet,
		explain: fmt.Sprintf("%d old formula versions, cached downloads and logs Homebrew no longer needs.", n),
		regen:   "brew downloads a bottle again when you reinstall or upgrade", regrows: true,
	})
	it.Bytes = parseSize(m[1])
	it.Frees = it.Bytes
	it.Note = "size is Homebrew's own estimate"
	r.Items = append(r.Items, it)
	return r
}

// autoremoveNames reads `brew autoremove -n`:
//
//	==> Would autoremove 2 unneeded formulae:
//	icu4c@76
//	python@3.12
func autoremoveNames(out string) []string {
	var names []string
	in := false
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(l, "==> Would autoremove"):
			in = true
		case strings.HasPrefix(l, "==>"):
			in = false
		case in && l != "" && !strings.ContainsAny(l, " /"):
			names = append(names, l)
		}
	}
	return names
}

// list joins the first n names and says how many more there are.
func list(names []string, n int) string {
	if len(names) <= n {
		return strings.Join(names, ", ")
	}
	return strings.Join(names[:n], ", ") + fmt.Sprintf(" and %d more", len(names)-n)
}
