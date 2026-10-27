package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Matthew-Eucaristo/decruft/internal/disk"
	"github.com/Matthew-Eucaristo/decruft/internal/family"
	"github.com/Matthew-Eucaristo/decruft/internal/item"
	"github.com/Matthew-Eucaristo/decruft/internal/module"
)

// testDeps is a machine with one safe command item, one review item and
// one folder, and no family tools.
func testDeps(t *testing.T) (deps, *[]string) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Caches", "tool")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var ran []string
	env := &module.Env{Home: home, OS: "test", Getenv: func(string) string { return "" },
		LookPath: func(string) (string, error) { return "", errors.New("no") },
		Running:  func(string) bool { return false },
		Measure:  func(string) module.Measured { return module.Measured{Bytes: 1e9, Frees: 1e9} }}
	mod := module.Module{ID: "t", Scan: func(context.Context, *module.Env) module.Result {
		return module.Result{Items: []item.Item{
			{ID: "safe", Source: "t", Name: "safe cache", Path: dir, Bytes: 1e9, Frees: 1e9, Level: item.Safe,
				Action: item.Action{Kind: item.Command, Argv: []string{"/bin/tool", "clean"}}},
			{ID: "rev", Source: "t", Name: "review thing", Bytes: 2e9, Frees: 2e9, Level: item.Review,
				Action: item.Action{Kind: item.Command, Argv: []string{"/bin/tool", "nuke"}}},
		}}
	}}
	free := int64(5e9)
	d := deps{
		env: env, modset: []module.Module{mod},
		fam: &family.Family{Home: home, LookPath: env.LookPath},
		run: func(_ context.Context, _ []string, argv []string) error {
			ran = append(ran, strings.Join(argv, " "))
			free += 1e9
			return nil
		},
		free: func(string) int64 { return free },
		disk: func(context.Context, string) disk.Info {
			return disk.Info{Name: "Test", Total: 100e9, Free: free, Purgeable: -1, Swap: -1, Trash: -1}
		},
	}
	return d, &ran
}

func TestNonTTYPrintsListAndActsOnNothing(t *testing.T) {
	d, ran := testDeps(t)
	var out bytes.Buffer
	if err := runWith(nil, &out, &out, d); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "safe cache") || !strings.Contains(out.String(), "runs tool clean") || len(*ran) != 0 {
		t.Fatalf("ran %v\n%s", *ran, out.String())
	}
}

func TestJSONActsOnNothing(t *testing.T) {
	d, ran := testDeps(t)
	var out bytes.Buffer
	if err := runWith([]string{"--json", "--no-family"}, &out, &out, d); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"schema_version": 1`) || len(*ran) != 0 || strings.Contains(out.String(), d.env.Home) {
		t.Fatalf("ran %v\n%s", *ran, out.String())
	}
}

func TestYesCleansOnlySafeItems(t *testing.T) {
	d, ran := testDeps(t)
	var out bytes.Buffer
	if err := runWith([]string{"--yes", "--no-family"}, &out, &out, d); err != nil {
		t.Fatal(err)
	}
	if len(*ran) != 1 || (*ran)[0] != "/bin/tool clean" {
		t.Fatalf("ran %v", *ran)
	}
	if !strings.Contains(out.String(), "Freed now") || !strings.Contains(out.String(), "+1.0 GB measured") {
		t.Fatal(out.String())
	}
}

func TestYesDryRunRunsNothing(t *testing.T) {
	d, ran := testDeps(t)
	var out bytes.Buffer
	if err := runWith([]string{"--yes", "--dry-run", "--no-family"}, &out, &out, d); err != nil {
		t.Fatal(err)
	}
	if len(*ran) != 0 || !strings.Contains(out.String(), "Would free") {
		t.Fatalf("ran %v\n%s", *ran, out.String())
	}
}

func TestUsageErrors(t *testing.T) {
	d, _ := testDeps(t)
	var out bytes.Buffer
	for _, args := range [][]string{{"--dry-run", "--permanent"}, {"--nope"}} {
		if err := runWith(args, &out, &out, d); !errors.Is(err, errUsage) {
			t.Fatalf("%v: %v", args, err)
		}
	}
	if err := runWith([]string{"/definitely/not/here"}, &out, &out, d); err == nil {
		t.Fatal("a missing root is an error")
	}
}
