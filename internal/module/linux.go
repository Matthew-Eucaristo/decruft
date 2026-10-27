package module

import (
	"context"
	"fmt"
	"strings"

	"github.com/Matthew-Eucaristo/decruft/internal/item"
)

// Linux: the package manager's own cleanups. apt and journald need root, so
// decruft shows the exact command and never runs it; unused Flatpak runtimes
// installed for the user are removed with flatpak itself.
func Linux() Module {
	return Module{ID: "system", Name: "System packages", Scan: scanLinux}
}

func scanLinux(ctx context.Context, env *Env) Result {
	var r Result
	sudo := func(it *item.Item, argv ...string) {
		it.Action = item.Action{Kind: item.Manual, Argv: append([]string{"sudo"}, argv...), How: "needs root: run it yourself"}
		it.Add("needs_sudo", item.Protected, "needs root; decruft shows the command and never runs sudo")
	}
	if apt, err := env.LookPath("apt-get"); err == nil {
		if it, ok := build(env, spec{source: "system", category: item.System, kind: "apt-cache", name: "apt downloaded packages",
			path: "/var/cache/apt/archives", explain: "Package files apt downloaded to install or upgrade."}); ok {
			sudo(&it, "apt-get", "clean")
			r.Items = append(r.Items, it)
		}
		if out, err := env.Output(ctx, nil, apt, "-s", "autoremove"); err == nil {
			var names []string
			for _, l := range strings.Split(string(out), "\n") {
				if f := strings.Fields(l); len(f) > 1 && f[0] == "Remv" {
					names = append(names, f[1])
				}
			}
			if len(names) > 0 {
				it, _ := build(env, spec{source: "system", category: item.System, kind: "apt-autoremove",
					name:    fmt.Sprintf("Unused dependencies (%d)", len(names)),
					explain: "Packages installed as dependencies that nothing needs any more: " + list(names, 6) + "."}, strings.Join(names, ","))
				sudo(&it, "apt-get", "autoremove")
				r.Items = append(r.Items, it)
			}
		}
	}
	if jc, err := env.LookPath("journalctl"); err == nil {
		if out, err := env.Output(ctx, nil, jc, "--disk-usage"); err == nil {
			// "Archived and active journals take up 1.2G in the file system."
			if f := strings.Fields(string(out)); len(f) > 0 {
				for i, w := range f {
					if w == "up" && i+1 < len(f) {
						if n := parseSize(f[i+1]); n > 0 {
							it, _ := build(env, spec{source: "system", category: item.System, kind: "journal", name: "System journal",
								explain: "systemd's logs. Vacuuming keeps the last two weeks."})
							it.Bytes = n
							sudo(&it, "journalctl", "--vacuum-time=2weeks")
							r.Items = append(r.Items, it)
						}
					}
				}
			}
		}
	}
	if fp, err := env.LookPath("flatpak"); err == nil {
		if out, err := env.Output(ctx, nil, fp, "list", "--user", "--runtime", "--columns=application"); err == nil && strings.TrimSpace(string(out)) != "" {
			it, _ := build(env, spec{source: "system", category: item.System, kind: "flatpak-unused", name: "Unused Flatpak runtimes (user)",
				argv: []string{fp, "uninstall", "--user", "--unused", "--noninteractive"}, level: item.Review,
				explain: "Runtimes and extensions no installed app uses any more. Size is unknown until Flatpak removes them."})
			r.Items = append(r.Items, it)
		}
	}
	return r
}
