package module

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Matthew-Eucaristo/decruft/internal/item"
)

// Go: the build cache and the module cache, located with `go env` and
// removed with `go clean`, which knows the module cache is read-only.
func Go() Module {
	return Module{ID: "go", Name: "Go", Scan: scanGo}
}

func scanGo(ctx context.Context, env *Env) Result {
	var r Result
	goBin, err := env.LookPath("go")
	if err != nil {
		return r
	}
	out, err := env.Output(ctx, nil, goBin, "env", "-json", "GOCACHE", "GOMODCACHE")
	if err != nil {
		r.Warnings = append(r.Warnings, "go env: "+err.Error())
		return r
	}
	var vars struct{ GOCACHE, GOMODCACHE string }
	if err := json.Unmarshal(out, &vars); err != nil {
		r.Warnings = append(r.Warnings, "go env: unreadable output; Go caches skipped")
		return r
	}
	if vars.GOCACHE != "" && vars.GOCACHE != "off" {
		if it, ok := build(env, spec{
			source: "go", category: item.Go, kind: "go-build-cache", name: "Go build cache", path: vars.GOCACHE,
			argv:    []string{goBin, "clean", "-cache"},
			explain: "Compiled packages and test results that go build and go test reuse (includes -testcache).",
			regen:   "the next go build (slower once)", regrows: true,
		}); ok {
			r.Items = append(r.Items, it)
		}
	}
	if vars.GOMODCACHE != "" {
		if it, ok := build(env, spec{
			source: "go", category: item.Go, kind: "go-mod-cache", name: "Go module cache", path: vars.GOMODCACHE,
			argv:    []string{goBin, "clean", "-modcache"},
			explain: "Downloaded module sources. Every project re-downloads what it needs.",
			regen:   "go mod download, or the next build (network)", regrows: true,
		}); ok {
			if strings.HasPrefix(vars.GOMODCACHE, env.Home) && strings.Contains(vars.GOMODCACHE, "/pkg/mod") {
				it.Note = joinNote(it.Note, "installed tools in ~/go/bin are kept")
			}
			r.Items = append(r.Items, it)
		}
	}
	if lint, err := env.LookPath("golangci-lint"); err == nil {
		if out, err := env.Output(ctx, nil, lint, "cache", "status"); err == nil {
			for _, l := range strings.Split(string(out), "\n") {
				if dir, ok := strings.CutPrefix(strings.TrimSpace(l), "Dir: "); ok {
					if it, ok := build(env, spec{
						source: "go", category: item.Go, kind: "golangci-lint-cache", name: "golangci-lint cache", path: dir,
						argv:    []string{lint, "cache", "clean"},
						explain: "Analysis results golangci-lint reuses between runs.",
						regen:   "the next golangci-lint run", regrows: true,
					}); ok {
						r.Items = append(r.Items, it)
					}
				}
			}
		}
	}
	return r
}
