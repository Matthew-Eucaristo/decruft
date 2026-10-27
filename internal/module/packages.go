package module

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/Matthew-Eucaristo/decruft/internal/item"
)

// Packages: download caches with an official clean command: CocoaPods
// (`pod cache clean --all`), uv (`uv cache prune`) and pip (`pip cache purge`).
// Hand-deleting uv's cache leaves it inconsistent, so only the tools act.
func Packages() Module {
	return Module{ID: "packages", Name: "Package caches", Scan: scanPackages}
}

func scanPackages(ctx context.Context, env *Env) Result {
	var r Result
	if pod, err := env.LookPath("pod"); err == nil {
		dir := env.Getenv("CP_CACHE_DIR")
		if dir == "" {
			dir = filepath.Join(env.Home, "Library", "Caches", "CocoaPods")
			if env.OS != "darwin" {
				dir = filepath.Join(env.Home, ".cache", "CocoaPods")
			}
		}
		if it, ok := build(env, spec{
			source: "packages", category: item.Packages, kind: "cocoapods-cache", name: "CocoaPods cache", path: dir,
			argv: []string{pod, "cache", "clean", "--all"}, env: Quiet,
			explain: "Downloaded pod sources CocoaPods copies into projects.",
			regen:   "pod install (network)",
		}); ok {
			r.Items = append(r.Items, it)
		}
	}
	if uv, err := env.LookPath("uv"); err == nil {
		if out, err := env.Output(ctx, nil, uv, "cache", "dir"); err != nil {
			r.Warnings = append(r.Warnings, "uv cache dir: "+err.Error())
		} else if dir := firstLine(out); dir != "" {
			if it, ok := build(env, spec{
				source: "packages", category: item.Packages, kind: "uv-cache", name: "uv cache", path: dir,
				argv: []string{uv, "cache", "prune"}, upTo: true,
				explain: "Wheels, sources and environments uv reuses. prune removes only unused entries; `uv cache clean` would remove everything.",
				regen:   "uv sync / uv pip install (network)",
			}); ok {
				r.Items = append(r.Items, it)
			}
		}
	}
	for _, name := range []string{"pip3", "pip"} {
		pip, err := env.LookPath(name)
		if err != nil {
			continue
		}
		out, err := env.Output(ctx, nil, pip, "cache", "dir")
		if err != nil {
			if !strings.Contains(err.Error(), "disabled") {
				r.Warnings = append(r.Warnings, name+" cache dir: "+err.Error())
			}
			break
		}
		if dir := firstLine(out); dir != "" {
			if it, ok := build(env, spec{
				source: "packages", category: item.Packages, kind: "pip-cache", name: "pip cache", path: dir,
				argv: []string{pip, "cache", "purge"}, env: Quiet,
				explain: "Wheels and HTTP responses pip keeps to avoid downloading again.",
				regen:   "pip install (network)",
			}); ok {
				r.Items = append(r.Items, it)
			}
		}
		break
	}
	return r
}
