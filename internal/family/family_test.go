package family

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Matthew-Eucaristo/decruft/internal/item"
)

// fake answers a tool's commands from canned output.
func fake(answers map[string]string) *Family {
	return &Family{
		Home: "/home/u",
		LookPath: func(name string) (string, error) {
			for k := range answers {
				if strings.HasPrefix(k, name+" ") {
					return "/bin/" + name, nil
				}
			}
			return "", errors.New("not found")
		},
		Run: func(_ context.Context, _ []byte, argv ...string) ([]byte, int, error) {
			key := strings.TrimPrefix(strings.Join(argv, " "), "/bin/")
			out, ok := answers[key]
			if !ok {
				return nil, 1, errors.New("unknown command " + key)
			}
			return []byte(out), 0, nil
		},
	}
}

const nodekillScan = `{"schema_version":1,"warnings":["slow disk"],"items":[
 {"id":"art-1","fingerprint":"fp-1","kind":"node_modules","name":"node_modules","path":"~/dev/a/node_modules","measured":true,
  "bytes":{"allocated":1000,"shared":400},"action":"trash","safety":{"level":"safe","reasons":[]},"new_field":42},
 {"id":"glb-2","fingerprint":"fp-2","kind":"npm-cache","name":"npm cache","path":"~/.npm/_cacache","bytes":{"allocated":500,"shared":0},
  "action":"command","command":["npm","cache","clean","--force"],"safety":{"level":"brand-new-level"}},
 {"id":"","fingerprint":"fp-3","name":"no id"}]}`

func TestDiscoverAndScanContractTool(t *testing.T) {
	f := fake(map[string]string{
		"nodekill capabilities --json": `{"schema_version":1,"tool":{"name":"nodekill","version":"0.1.0"}}`,
		"nodekill scan --json":         nodekillScan,
	})
	tool, warn, ok := f.Discover(context.Background(), "nodekill", item.Node)
	if !ok || !tool.Contract || tool.Version != "0.1.0" || warn != "" {
		t.Fatalf("%+v %q %v", tool, warn, ok)
	}
	items, warns, err := f.Scan(context.Background(), tool)
	if err != nil || len(items) != 2 || len(warns) != 1 {
		t.Fatalf("items %d warns %v err %v", len(items), warns, err)
	}
	a, b := items[0], items[1]
	if a.Path != filepath.Join("/home/u", "dev", "a", "node_modules") || a.Bytes != 1000 || a.Frees != 600 || a.Level != item.Safe {
		t.Fatalf("%+v", a)
	}
	if a.Action.Kind != item.Delegate || a.Delegate.ID != "art-1" || a.Delegate.Fingerprint != "fp-1" || a.Delegate.Action != "" {
		t.Fatalf("%+v", a.Delegate)
	}
	// An unknown level never unlocks an item.
	if b.Level != item.Protected || b.Selectable() {
		t.Fatalf("unknown level: %+v", b)
	}
}

func TestFellPicksRetireOnlyWhenAllowed(t *testing.T) {
	scan := `{"schema_version":1,"items":[
	 {"id":"wt-1","fingerprint":"fp-1","kind":"worktree","name":"agent-a","path":"~/dev/app/.claude/worktrees/agent-a","project":{"name":"app"},
	  "bytes":{"allocated":900,"shared":0,"exclusive":900},"strippable_bytes":{"allocated":600,"shared":0,"exclusive":600},
	  "safety":{"level":"caution"},"actions":[{"action":"strip","allowed":true},{"action":"retire","allowed":true}]},
	 {"id":"wt-2","fingerprint":"fp-2","kind":"worktree","name":"agent-b","path":"~/dev/app/b",
	  "bytes":{"allocated":900,"shared":0,"exclusive":900},"strippable_bytes":{"allocated":300,"shared":0,"exclusive":300},
	  "safety":{"level":"protected"},"owner":{"name":"Claude Desktop","how_to_remove":"use the app"},
	  "actions":[{"action":"strip","allowed":true},{"action":"retire","allowed":false}]}]}`
	f := fake(map[string]string{"fell scan --json": scan})
	items, _, err := f.Scan(context.Background(), Tool{Name: "fell", Path: "/bin/fell", Contract: true, Category: item.Worktrees})
	if err != nil || len(items) != 2 {
		t.Fatal(err)
	}
	if d := items[0].Delegate; d == nil || d.Action != "retire" || items[0].Level != item.Review || items[0].Name != "app · agent-a" {
		t.Fatalf("%+v %+v", items[0], d)
	}
	// Protected by fell: shown with the app's own way, never delegated.
	if items[1].Selectable() || items[1].Action.Kind != item.Manual || items[1].Action.How != "use the app" {
		t.Fatalf("%+v", items[1])
	}
}

func TestUnknownSchemaIsSkipped(t *testing.T) {
	f := fake(map[string]string{"fell capabilities --json": `{"schema_version":2,"tool":{"version":"9"}}`})
	if _, warn, ok := f.Discover(context.Background(), "fell", item.Worktrees); ok || !strings.Contains(warn, "version 2") {
		t.Fatalf("ok %v warn %q", ok, warn)
	}
	f = fake(map[string]string{"fell scan --json": `{"schema_version":2,"items":[]}`})
	if _, _, err := f.Scan(context.Background(), Tool{Name: "fell", Path: "/bin/fell", Contract: true}); err == nil {
		t.Fatal("a scan with another schema must be rejected")
	}
	f = fake(map[string]string{"fell scan --json": `not json`})
	if _, _, err := f.Scan(context.Background(), Tool{Name: "fell", Path: "/bin/fell", Contract: true}); err == nil {
		t.Fatal("unreadable output must be rejected")
	}
}

func TestMissingToolIsReported(t *testing.T) {
	f := fake(map[string]string{})
	if _, warn, ok := f.Discover(context.Background(), "fell", item.Worktrees); ok || !strings.Contains(warn, "not installed") {
		t.Fatalf("%v %q", ok, warn)
	}
}

func TestLegacyFlutterkillIsHandoffOnly(t *testing.T) {
	f := fake(map[string]string{
		"flutterkill --version": "flutterkill 0.2.0\n",
		"flutterkill --json":    `{"root":"/home/u/dev","projects":[{"name":"app","path":"/home/u/dev/app","artifacts":[{"name":"build","path":"/home/u/dev/app/build","bytes":300}]}],"caches":[{"id":"gradle-caches","name":"Gradle caches","path":"/home/u/.gradle/caches","bytes":100}]}`,
	})
	tool, _, ok := f.Discover(context.Background(), "flutterkill", item.Flutter)
	if !ok || tool.Contract || tool.Version != "0.2.0" {
		t.Fatalf("%+v", tool)
	}
	items, _, err := f.Scan(context.Background(), tool)
	if err != nil || len(items) != 2 {
		t.Fatal(err, items)
	}
	for _, it := range items {
		if it.Selectable() || it.Action.Kind != item.Handoff {
			t.Fatalf("legacy item must not be selectable: %+v", it)
		}
	}
	if items[1].Category != item.Android {
		t.Fatalf("Gradle cache filed under %v", items[1].Category)
	}
	if _, err := f.Apply(context.Background(), tool, items, Trash); err == nil {
		t.Fatal("apply without a contract must fail")
	}
}

// flutterkill 0.3 speaks the contract: its items are delegated like
// nodekill's, filed by kind, and named after their project.
func TestFlutterkillContractDelegates(t *testing.T) {
	f := fake(map[string]string{
		"flutterkill capabilities --json": `{"schema_version":1,"tool":{"name":"flutterkill","version":"0.3.0"}}`,
		"flutterkill scan --json": `{"schema_version":1,"warnings":[],"items":[
 {"id":"art-1","fingerprint":"fp-1","kind":"build","name":"build","path":"~/dev/app/build","project":{"name":"app"},"measured":true,
  "bytes":{"allocated":300,"shared":0,"exclusive":300},"action":"trash","safety":{"level":"safe","reasons":[{"code":"generated","level":"safe"}]}},
 {"id":"glb-2","fingerprint":"fp-2","kind":"gradle-dist","name":"Gradle 8.3","path":"~/.gradle/wrapper/dists/gradle-8.3-all","project":null,
  "measured":true,"bytes":{"allocated":100},"action":"trash","safety":{"level":"review"}},
 {"id":"glb-3","fingerprint":"fp-3","kind":"pub-cache","name":"Pub cache","path":"~/.pub-cache","measured":true,"bytes":{"allocated":50},
  "action":"command","command":["dart","pub","cache","clean","--force"],"safety":{"level":"review"}}]}`,
		"flutterkill apply --plan - --json --permanent": `{"schema_version":1,"results":[{"id":"art-1","status":"done","action":"delete","estimated_bytes":300}]}`,
	})
	var plan []byte
	run := f.Run
	f.Run = func(ctx context.Context, in []byte, argv ...string) ([]byte, int, error) {
		if in != nil {
			plan = in
		}
		return run(ctx, in, argv...)
	}
	tool, warn, ok := f.Discover(context.Background(), "flutterkill", item.Flutter)
	if !ok || !tool.Contract || tool.Version != "0.3.0" || warn != "" {
		t.Fatalf("%+v %q %v", tool, warn, ok)
	}
	items, _, err := f.Scan(context.Background(), tool)
	if err != nil || len(items) != 3 {
		t.Fatal(err, items)
	}
	art, dist, pub := items[0], items[1], items[2]
	if art.Name != "app · build" || art.Category != item.Flutter || art.Level != item.Safe || art.Action.Kind != item.Delegate || !art.Selectable() {
		t.Fatalf("%+v", art)
	}
	if dist.Category != item.Android || dist.Level != item.Review || dist.Action.Kind != item.Delegate {
		t.Fatalf("%+v", dist)
	}
	if pub.Category != item.Flutter || !strings.Contains(pub.Delegate.Text, "dart pub cache clean --force") {
		t.Fatalf("%+v", pub)
	}
	res, err := f.Apply(context.Background(), tool, []item.Item{art}, Permanent)
	if err != nil || len(res) != 1 || res[0].Status != "done" {
		t.Fatal(err, res)
	}
	if string(plan) != `{"schema_version":1,"items":[{"id":"art-1","fingerprint":"fp-1"}]}` {
		t.Fatalf("plan %s", plan)
	}
}

func TestApplyRejectsForeignItems(t *testing.T) {
	f := fake(map[string]string{})
	it := item.Item{Name: "x", Delegate: &item.Delegation{Tool: "fell", ID: "wt-1"}}
	if _, err := f.Apply(context.Background(), Tool{Name: "nodekill", Contract: true}, []item.Item{it}, Trash); err == nil {
		t.Fatal("an item must go back to its own tool")
	}
}

// rustkillScan is the shape of `rustkill scan --json`: a project's target
// folder (trash), an old toolchain (rustup's own command), the default
// toolchain (protected) and a cache it could not measure.
const rustkillScan = `{"schema_version":1,"tool":{"name":"rustkill","version":"0.1.0","ecosystem":"rust"},"roots":["~/dev"],"warnings":[],"items":[
 {"id":"tgt-1","fingerprint":"fp-1","kind":"target","category":"project","name":"target","path":"~/dev/shop-api/target",
  "project":{"path":"~/dev/shop-api","name":"shop-api","workspace":false,"toolchain":""},"action":"trash","regenerate":"cargo build",
  "explain":"Cargo's build output for shop-api.","measured":true,"bytes":{"allocated":3000,"shared":0,"exclusive":3000,"file_count":9},
  "git":{"ignored":true,"tracked_files":false},"safety":{"level":"safe","reasons":[]}},
 {"id":"glb-2","fingerprint":"fp-2","kind":"toolchain","category":"runtime","name":"toolchain 1.80.0","path":"~/.rustup/toolchains/1.80.0-x86_64-unknown-linux-gnu",
  "project":null,"tool":"rustup","action":"command","command":["~/.cargo/bin/rustup","toolchain","uninstall","1.80.0-x86_64-unknown-linux-gnu"],
  "regenerate":"rustup toolchain install 1.80.0","explain":"A Rust toolchain rustup installed.","note":"no project under the scanned folders pins it",
  "measured":true,"bytes":{"allocated":2000,"shared":0,"exclusive":2000,"file_count":4},
  "safety":{"level":"review","reasons":[{"code":"catalog_review","level":"review","detail":"check that no project still needs it"}]}},
 {"id":"glb-3","fingerprint":"fp-3","kind":"toolchain","category":"runtime","name":"toolchain stable (1.91.0)","path":"~/.rustup/toolchains/stable-x86_64-unknown-linux-gnu",
  "project":null,"tool":"rustup","action":"command","command":["~/.cargo/bin/rustup","toolchain","uninstall","stable-x86_64-unknown-linux-gnu"],
  "measured":true,"bytes":{"allocated":1000,"shared":0,"exclusive":1000,"file_count":4},
  "safety":{"level":"protected","reasons":[{"code":"current","level":"protected","detail":"rustup's default toolchain"}]}},
 {"id":"glb-4","fingerprint":"fp-4","kind":"registry-src","category":"cache","name":"crates.io sources","path":"~/.cargo/registry/src/index.crates.io-1949cf8c6b5b557f",
  "project":null,"tool":"cargo","action":"trash","measured":false,"bytes":{"allocated":0,"shared":0,"exclusive":0,"file_count":0},
  "safety":{"level":"safe","reasons":[]}}]}`

// rustkill speaks the contract: every item goes in Rust, trash and command
// items alike are handed back by id and fingerprint, and what rustkill
// protects stays protected.
func TestRustkillContractDelegates(t *testing.T) {
	f := fake(map[string]string{
		"rustkill capabilities --json":               `{"schema_version":1,"tool":{"name":"rustkill","version":"0.1.0","ecosystem":"rust"}}`,
		"rustkill scan --json":                       rustkillScan,
		"rustkill apply --plan - --json --dry-run":   `{"schema_version":1,"results":[{"id":"tgt-1","status":"dry_run","action":"trash","estimated_bytes":3000},{"id":"glb-2","status":"dry_run","action":"command","estimated_bytes":2000}]}`,
		"rustkill apply --plan - --json --permanent": `{"schema_version":2,"results":[]}`,
		"rustkill apply --plan - --json":             `not json`,
	})
	var plan []byte
	run := f.Run
	f.Run = func(ctx context.Context, in []byte, argv ...string) ([]byte, int, error) {
		if in != nil {
			plan = in
		}
		return run(ctx, in, argv...)
	}
	tool, warn, ok := f.Discover(context.Background(), "rustkill", item.Rust)
	if !ok || !tool.Contract || tool.Version != "0.1.0" || warn != "" {
		t.Fatalf("%+v %q %v", tool, warn, ok)
	}
	items, warns, err := f.Scan(context.Background(), tool)
	if err != nil || len(items) != 4 || len(warns) != 0 {
		t.Fatal(err, warns, items)
	}
	target, old, stable, src := items[0], items[1], items[2], items[3]
	for _, it := range items {
		if it.Category != item.Rust || it.Source != "rustkill" || it.Delegate == nil || it.Delegate.Tool != "rustkill" {
			t.Fatalf("%+v", it)
		}
	}
	if target.Name != "shop-api · target" || target.Path != filepath.Join("/home/u", "dev", "shop-api", "target") ||
		target.Bytes != 3000 || target.Frees != 3000 || target.Level != item.Safe || target.Action.Kind != item.Delegate ||
		!target.Selectable() || target.Delegate.Text != "rustkill moves it to the Trash" {
		t.Fatalf("target: %+v %+v", target, target.Delegate)
	}
	if old.Level != item.Review || !old.Selectable() || old.Action.Kind != item.Delegate || old.Note == "" ||
		old.Delegate.Text != "rustkill runs: ~/.cargo/bin/rustup toolchain uninstall 1.80.0-x86_64-unknown-linux-gnu" ||
		len(old.Reasons) != 1 || old.Reasons[0].Code != "catalog_review" {
		t.Fatalf("old toolchain: %+v %+v", old, old.Delegate)
	}
	// The default toolchain: shown with rustkill's reason, never offered.
	if stable.Level != item.Protected || stable.Selectable() || stable.Action.Kind != item.None || stable.Reasons[0].Code != "current" {
		t.Fatalf("default toolchain: %+v", stable)
	}
	if src.Bytes != -1 || src.Frees != -1 || !src.Selectable() {
		t.Fatalf("unmeasured cache: %+v", src)
	}
	res, err := f.Apply(context.Background(), tool, []item.Item{target, old}, DryRun)
	if err != nil || len(res) != 2 || res[1].Action != "command" {
		t.Fatal(err, res)
	}
	if string(plan) != `{"schema_version":1,"items":[{"id":"tgt-1","fingerprint":"fp-1"},{"id":"glb-2","fingerprint":"fp-2"}]}` {
		t.Fatalf("plan %s", plan)
	}
	if _, err := f.Apply(context.Background(), tool, []item.Item{target}, Permanent); err == nil {
		t.Fatal("an apply result with another schema must be rejected")
	}
	if _, err := f.Apply(context.Background(), tool, []item.Item{target}, Trash); err == nil {
		t.Fatal("an unreadable apply result must be rejected")
	}
}

func TestRustkillUnknownSchemaIsSkipped(t *testing.T) {
	f := fake(map[string]string{"rustkill capabilities --json": `{"schema_version":2,"tool":{"name":"rustkill","version":"1.0.0"}}`})
	if _, warn, ok := f.Discover(context.Background(), "rustkill", item.Rust); ok || !strings.Contains(warn, "rustkill speaks contract version 2") {
		t.Fatalf("ok %v warn %q", ok, warn)
	}
	// No legacy fallback: only flutterkill 0.2 had one.
	f = fake(map[string]string{"rustkill capabilities --json": `usage: rustkill …`, "rustkill --version": "rustkill 0.0.1"})
	if _, warn, ok := f.Discover(context.Background(), "rustkill", item.Rust); ok || !strings.Contains(warn, "upgrade") {
		t.Fatalf("ok %v warn %q", ok, warn)
	}
	f = fake(map[string]string{"rustkill scan --json": strings.Replace(rustkillScan, `"schema_version":1`, `"schema_version":2`, 1)})
	if _, _, err := f.Scan(context.Background(), Tool{Name: "rustkill", Path: "/bin/rustkill", Contract: true, Category: item.Rust}); err == nil {
		t.Fatal("a rustkill scan with another schema must be rejected")
	}
}

func TestKnownIncludesRustkill(t *testing.T) {
	order := map[string]int{}
	for i, k := range Known {
		order[k.Name] = i
		if k.Name == "rustkill" && k.Category != item.Rust {
			t.Fatalf("rustkill files its items under %s", k.Category)
		}
	}
	r, ok := order["rustkill"]
	if !ok || r > order["nodekill"] {
		t.Fatal("rustkill must be known, and come before nodekill so it owns Rust build directories both report")
	}
}
