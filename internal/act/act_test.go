package act

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Matthew-Eucaristo/decruft/internal/family"
	"github.com/Matthew-Eucaristo/decruft/internal/item"
	"github.com/Matthew-Eucaristo/decruft/internal/module"
)

// machine is a fake home with one folder per test item and a module that
// offers them, like a real built-in module does.
type machine struct {
	home    string
	free    int64
	ran     [][]string
	trashed []string
	items   func(home string) []item.Item
}

func (mc *machine) env() *module.Env {
	return &module.Env{
		Home: mc.home, OS: "test", Getenv: func(string) string { return "" },
		LookPath: func(string) (string, error) { return "", os.ErrNotExist },
		Running:  func(string) bool { return false },
		Measure: func(p string) module.Measured {
			if _, err := os.Lstat(p); err != nil {
				return module.Measured{Missing: true}
			}
			return module.Measured{Bytes: 1e9, Frees: 1e9}
		},
	}
}

func (mc *machine) executor(mode Mode) *Executor {
	mods := []module.Module{{ID: "test", Scan: func(_ context.Context, env *module.Env) module.Result {
		var r module.Result
		for _, it := range mc.items(env.Home) {
			if it.Path != "" && env.Measure(it.Path).Missing {
				continue
			}
			r.Items = append(r.Items, it)
		}
		return r
	}}}
	return &Executor{Mode: mode, Home: mc.home, Env: mc.env(), Modules: mods,
		Run: func(_ context.Context, _ []string, argv []string) error {
			mc.ran = append(mc.ran, argv)
			mc.free += 1e9
			return nil
		},
		Free:        func(string) int64 { return mc.free },
		MoveToTrash: func(p string) error { mc.trashed = append(mc.trashed, p); return nil },
	}
}

func newMachine(t *testing.T) *machine {
	home := t.TempDir()
	for _, d := range []string{"Library/Caches/a", "Library/Caches/b", "Library/Developer/Xcode/DerivedData/App-x"} {
		if err := os.MkdirAll(filepath.Join(home, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mc := &machine{home: home, free: 5e9}
	mc.items = func(home string) []item.Item {
		cmd := item.Item{ID: "cmd", Source: "test", Kind: "cache", Name: "cache", Path: filepath.Join(home, "Library/Caches/a"),
			Bytes: 1e9, Frees: 1e9, Level: item.Safe, Action: item.Action{Kind: item.Command, Argv: []string{"/bin/tool", "clean"}}}
		dd := item.Item{ID: "dd", Source: "test", Kind: "derived", Name: "DerivedData", Path: filepath.Join(home, "Library/Developer/Xcode/DerivedData/App-x"),
			Bytes: 1e9, Frees: 1e9, Level: item.Safe, Action: item.Action{Kind: item.Trash}}
		sudo := item.Item{ID: "sudo", Source: "test", Kind: "apt", Name: "apt", Bytes: 1e9, Frees: 1e9, Level: item.Safe,
			Action: item.Action{Kind: item.Command, Argv: []string{"sudo", "apt-get", "clean"}}}
		prot := item.Item{ID: "prot", Source: "test", Kind: "archives", Name: "Archives", Path: filepath.Join(home, "Library/Caches/b"),
			Bytes: 1e9, Frees: 1e9, Level: item.Protected, Action: item.Action{Kind: item.Manual, How: "Xcode"}}
		out := []item.Item{cmd, dd, sudo, prot}
		for i := range out {
			out[i].Seal()
		}
		return out
	}
	return mc
}

func (mc *machine) item(id string) item.Item {
	for _, it := range mc.items(mc.home) {
		if it.ID == id {
			return it
		}
	}
	panic(id)
}

func TestCommandRunsAndIsMeasured(t *testing.T) {
	mc := newMachine(t)
	o := mc.executor(Trash).Do(context.Background(), Step{Items: []item.Item{mc.item("cmd")}})[0]
	if o.How != Freed || o.Freed != 1e9 || len(mc.ran) != 1 {
		t.Fatalf("outcome %+v ran %v", o, mc.ran)
	}
}

func TestDryRunNeverActs(t *testing.T) {
	mc := newMachine(t)
	ex := mc.executor(DryRun)
	for _, id := range []string{"cmd", "dd"} {
		o := ex.Do(context.Background(), Step{Items: []item.Item{mc.item(id)}})[0]
		if o.How != Dry {
			t.Fatalf("%s: %+v", id, o)
		}
	}
	if len(mc.ran) != 0 {
		t.Fatalf("dry run ran %v", mc.ran)
	}
	if _, err := os.Stat(mc.item("dd").Path); err != nil {
		t.Fatal("dry run moved a folder")
	}
}

func TestTamperedItemIsRefused(t *testing.T) {
	mc := newMachine(t)
	// A caller swaps the command of a safe item: the module's own copy wins.
	it := mc.item("cmd")
	it.Action.Argv = []string{"/bin/rm", "-rf", "/"}
	o := mc.executor(Trash).Do(context.Background(), Step{Items: []item.Item{it}})[0]
	if o.How != Freed || len(mc.ran) != 1 || mc.ran[0][0] != "/bin/tool" {
		t.Fatalf("%+v ran %v", o, mc.ran)
	}
}

func TestSudoAndProtectedAreRefused(t *testing.T) {
	mc := newMachine(t)
	ex := mc.executor(Trash)
	for _, id := range []string{"sudo", "prot"} {
		it := mc.item(id)
		it.Level = item.Safe // even if a caller tampers with the level
		if id == "prot" {
			it.Action = item.Action{Kind: item.Trash}
		}
		o := ex.Do(context.Background(), Step{Items: []item.Item{it}})[0]
		if o.How != Refused {
			t.Fatalf("%s: %+v", id, o)
		}
	}
	if len(mc.ran) != 0 || len(mc.trashed) != 0 {
		t.Fatalf("ran %v, trashed %v", mc.ran, mc.trashed)
	}
	if _, err := os.Stat(filepath.Join(mc.home, "Library/Caches/b")); err != nil {
		t.Fatal("protected folder touched")
	}
}

func TestChangedSinceScanIsRefused(t *testing.T) {
	mc := newMachine(t)
	it := mc.item("cmd")
	ex := mc.executor(Trash)
	// The folder is replaced: same path, new identity in the fingerprint.
	mc.items = func(home string) []item.Item {
		x := it
		x.Evidence = []string{"other-inode"}
		x.Seal()
		return []item.Item{x}
	}
	if o := ex.Do(context.Background(), Step{Items: []item.Item{it}})[0]; o.How != Refused || !strings.Contains(o.Error, "changed") {
		t.Fatalf("%+v", o)
	}
	// And gone entirely.
	mc.items = func(string) []item.Item { return nil }
	if o := ex.Do(context.Background(), Step{Items: []item.Item{it}})[0]; o.How != Refused || !strings.Contains(o.Error, "no longer") {
		t.Fatalf("%+v", o)
	}
	if len(mc.ran) != 0 {
		t.Fatal("stale item ran")
	}
}

func TestGuard(t *testing.T) {
	home := t.TempDir()
	ex := &Executor{Home: home}
	deep := filepath.Join(home, "a", "b", "c")
	link := filepath.Join(home, "a", "b", "link")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(deep, link); err != nil {
		t.Fatal(err)
	}
	cases := map[string]bool{
		deep:                         true,
		home:                         false,
		filepath.Join(home, "a"):     false, // too close to home
		filepath.Join(home, "a/b"):   false,
		link:                         false, // symlink
		"/tmp":                       false, // outside home
		"relative/path":              false,
		filepath.Join(home, "a/b/x"): false, // missing
	}
	for p, ok := range cases {
		if err := ex.guard(p); (err == nil) != ok {
			t.Errorf("guard(%s) = %v, want ok=%v", p, err, ok)
		}
	}
}

func TestTrashAndPermanent(t *testing.T) {
	mc := newMachine(t)
	o := mc.executor(Trash).Do(context.Background(), Step{Items: []item.Item{mc.item("dd")}})[0]
	if o.How != Trashed || len(mc.trashed) != 1 || mc.trashed[0] != mc.item("dd").Path {
		t.Fatalf("%+v %v", o, mc.trashed)
	}
	o = mc.executor(Permanent).Do(context.Background(), Step{Items: []item.Item{mc.item("dd")}})[0]
	if o.How != Deleted {
		t.Fatalf("%+v", o)
	}
	if _, err := os.Stat(filepath.Join(mc.home, "Library/Developer/Xcode/DerivedData/App-x")); !os.IsNotExist(err) {
		t.Fatal("folder still there")
	}
}

func TestCancelledContextActsOnNothing(t *testing.T) {
	mc := newMachine(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if o := mc.executor(Trash).Do(ctx, Step{Items: []item.Item{mc.item("cmd")}})[0]; o.How != Refused {
		t.Fatalf("%+v", o)
	}
	if len(mc.ran) != 0 {
		t.Fatal("ran after cancel")
	}
}

func TestPlanOrdersAndBatches(t *testing.T) {
	a := item.Item{ID: "a", Priority: 5, Action: item.Action{Kind: item.Command}}
	b := item.Item{ID: "b", Priority: -1, Action: item.Action{Kind: item.Command}}
	d1 := item.Item{ID: "d1", Action: item.Action{Kind: item.Delegate}, Delegate: &item.Delegation{Tool: "nodekill"}}
	d2 := item.Item{ID: "d2", Action: item.Action{Kind: item.Delegate}, Delegate: &item.Delegation{Tool: "nodekill"}}
	steps := Plan([]item.Item{a, d1, b, d2})
	if len(steps) != 3 || steps[0].Items[0].ID != "b" || steps[1].Items[0].ID != "a" || steps[2].Tool != "nodekill" || len(steps[2].Items) != 2 {
		t.Fatalf("%+v", steps)
	}
}

// fakeTool answers apply with a fixed result and records the plan.
func TestDelegateSendsOnlyIDsAndUserFlags(t *testing.T) {
	var gotArgs []string
	var gotPlan map[string]any
	fam := &family.Family{Home: "/h", Run: func(_ context.Context, stdin []byte, argv ...string) ([]byte, int, error) {
		gotArgs = argv
		_ = json.Unmarshal(stdin, &gotPlan)
		return []byte(`{"schema_version":1,"results":[{"id":"art-1","status":"done","estimated_bytes":500},{"id":"art-2","status":"refused","error":"changed since the scan"}]}`), 3, nil
	}}
	ex := &Executor{Mode: DryRun, Home: "/h", Family: fam, Free: func(string) int64 { return 0 },
		Tools: map[string]family.Tool{"nodekill": {Name: "nodekill", Path: "/bin/nodekill", Contract: true}}}
	mk := func(id string) item.Item {
		return item.Item{ID: "nodekill:" + id, Name: id, Source: "nodekill", Level: item.Review, Action: item.Action{Kind: item.Delegate},
			Delegate: &item.Delegation{Tool: "nodekill", ID: id, Fingerprint: "fp-" + id}}
	}
	outs := ex.Do(context.Background(), Step{Tool: "nodekill", Items: []item.Item{mk("art-1"), mk("art-2")}})
	if strings.Join(gotArgs, " ") != "/bin/nodekill apply --plan - --json --dry-run" {
		t.Fatalf("args %v", gotArgs)
	}
	if strings.Contains(strings.Join(gotArgs, " "), "--force") || strings.Contains(strings.Join(gotArgs, " "), "--unlock") {
		t.Fatal("never pass override flags")
	}
	entries := gotPlan["items"].([]any)
	first := entries[0].(map[string]any)
	if len(first) != 2 || first["id"] != "art-1" || first["fingerprint"] != "fp-art-1" {
		t.Fatalf("plan entry %v", first)
	}
	if outs[0].How != Delegated || outs[1].How != Refused || outs[1].Error == "" {
		t.Fatalf("%+v", outs)
	}
}

func TestDelegateWithoutContractIsRefused(t *testing.T) {
	ex := &Executor{Home: "/h", Free: func(string) int64 { return 0 }, Tools: map[string]family.Tool{"flutterkill": {Name: "flutterkill"}}}
	it := item.Item{ID: "x", Action: item.Action{Kind: item.Delegate}, Delegate: &item.Delegation{Tool: "flutterkill"}}
	if o := ex.Do(context.Background(), Step{Tool: "flutterkill", Items: []item.Item{it}})[0]; o.How != Refused {
		t.Fatalf("%+v", o)
	}
}
