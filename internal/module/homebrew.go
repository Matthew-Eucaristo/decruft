package module

import (
	"context"
	"fmt"
	"os"
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
		r.Items = append(r.Items, autoremoveItem(ctx, env, brew, names))
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

// autoremoveItem offers `brew autoremove` only when every formula it would
// uninstall is a library. Homebrew counts a formula as unneeded once nothing
// installed on request depends on it, even when its command is the one your
// PATH runs (a runtime such as bun pulled in by another formula, say), and
// autoremove would take that command away. So when a candidate provides a
// command on PATH, the bulk command is withheld: the item is protected,
// names those commands, and shows the uninstall of the others for you to
// run. When decruft cannot tell (no Cellar), it is protected too. Access
// times are not used: macOS does not update them on every run and bulk
// reads move them, so they would guess.
func autoremoveItem(ctx context.Context, env *Env, brew string, names []string) item.Item {
	cellar := ""
	if c, err := env.Output(ctx, nil, brew, "--cellar"); err == nil {
		cellar = firstLine(c)
	}
	var bytes, frees int64
	var used, libs []string
	for _, n := range names {
		if cellar == "" {
			bytes, frees = -1, -1
			continue
		}
		keg := filepath.Join(cellar, n)
		m := env.Measure(keg)
		bytes, frees = bytes+m.Bytes, frees+m.Frees
		if cmds := commandsOnPath(env, keg); len(cmds) > 0 {
			used = append(used, n+" ("+list(cmds, 3)+")")
		} else {
			libs = append(libs, n)
		}
	}
	it, _ := build(env, spec{
		source: "homebrew", category: item.Homebrew, kind: "brew-autoremove",
		name: fmt.Sprintf("Unused dependencies (%d)", len(names)),
		argv: []string{brew, "autoremove"}, env: Quiet, level: item.Review,
		explain: "Formulae installed only as dependencies that nothing installed needs any more: " + list(names, 6) + ".",
		regen:   "brew install <name>, if you used one directly", priority: -1,
	}, strings.Join(names, ","), "commands:"+strings.Join(used, ","), "cellar:"+fmt.Sprint(cellar != ""))
	it.Bytes, it.Frees = bytes, frees
	switch {
	case cellar == "":
		it.Add("unverified", item.Protected, "decruft could not find Homebrew's Cellar to check which of them you may run yourself, so it does not run brew autoremove")
		it.Action = item.Action{Kind: item.Manual, How: "check `brew autoremove -n` and uninstall what you do not use with `brew uninstall <name>`"}
	case len(used) > 0:
		it.Add("provides_commands", item.Protected, "your PATH runs commands from "+list(used, 4)+"; brew autoremove would uninstall them with the rest. Keep them with `brew install <name>`, or uninstall them yourself")
		it.Action = item.Action{Kind: item.Manual, How: "nothing else to remove"}
		if len(libs) > 0 {
			it.Action = item.Action{Kind: item.Manual, Argv: append([]string{brew, "uninstall"}, libs...), Env: Quiet}
			it.Note = joinNote(it.Note, "uninstalls only the libraries; brew refuses any that something still needs")
		}
	default:
		it.Add("uninstalls", item.Review, "uninstalls libraries nothing installed needs any more; none provides a command on your PATH")
	}
	return it
}

// commandsOnPath lists the executables of a keg (any installed version's
// bin and sbin) that are what PATH runs under their name.
func commandsOnPath(env *Env, keg string) []string {
	var out []string
	seen := map[string]bool{}
	if r, err := filepath.EvalSymlinks(keg); err == nil {
		keg = r
	}
	for _, version := range subdirs(keg) {
		for _, sub := range []string{"bin", "sbin"} {
			entries, err := os.ReadDir(filepath.Join(version, sub))
			if err != nil {
				continue
			}
			for _, e := range entries {
				name := e.Name()
				if seen[name] {
					continue
				}
				p, err := env.LookPath(name)
				if err != nil {
					continue
				}
				if real, err := filepath.EvalSymlinks(p); err == nil && strings.HasPrefix(real, keg+string(filepath.Separator)) {
					seen[name] = true
					out = append(out, name)
				}
			}
		}
	}
	return out
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
