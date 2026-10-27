package inventory

import (
	"context"
	"testing"

	"github.com/Matthew-Eucaristo/decruft/internal/item"
	"github.com/Matthew-Eucaristo/decruft/internal/module"
)

func builtin(id, path string, kind item.Kind) item.Item {
	return item.Item{ID: id, Source: "xcode", Kind: id, Path: path, Bytes: 10, Frees: 10, Level: item.Safe, Action: item.Action{Kind: kind}}
}

func delegated(tool, id, path string) item.Item {
	return item.Item{ID: tool + ":" + id, Source: tool, Kind: id, Path: path, Bytes: 10, Frees: 10, Level: item.Safe,
		Action: item.Action{Kind: item.Delegate}, Delegate: &item.Delegation{Tool: tool, ID: id}}
}

func ids(items []item.Item) map[string]bool {
	out := map[string]bool{}
	for _, it := range items {
		out[it.ID] = true
	}
	return out
}

func TestDedupe(t *testing.T) {
	items := []item.Item{
		builtin("dd", "/h/Library/Developer/Xcode/DerivedData/App-x", item.Trash),
		delegated("flutterkill", "xcode-derived-data", "/h/Library/Developer/Xcode/DerivedData"), // contains a built-in: dropped
		delegated("fell", "wt", "/h/dev/app/.claude/worktrees/a"),
		delegated("nodekill", "nm", "/h/dev/app/.claude/worktrees/a/node_modules"), // inside fell's worktree: dropped
		builtin("gradle", "/h/.gradle/caches", item.None),                          // info only: loses to a tool that acts
		delegated("flutterkill", "gradle-caches", "/h/.gradle/caches"),
		builtin("simctl-unavailable", "", item.Command),
		delegated("flutterkill", "simulators-unavailable", ""), // the same thing without a path
		delegated("nodekill", "same", "/h/x"),
		delegated("flutterkill", "same", "/h/x"), // same path, two tools: the first in family order wins
	}
	out, dropped := Dedupe(items)
	got := ids(out)
	want := []string{"dd", "fell:wt", "flutterkill:gradle-caches", "simctl-unavailable", "nodekill:same"}
	for _, id := range want {
		if !got[id] {
			t.Errorf("missing %s", id)
		}
	}
	if len(out) != len(want) || dropped != len(items)-len(want) {
		t.Fatalf("kept %v (dropped %d)", got, dropped)
	}
}

func TestDedupeKeepsSameSourceSiblings(t *testing.T) {
	items := []item.Item{builtin("a", "/h/a", item.Trash), builtin("b", "/h/a/b", item.Trash)}
	if out, _ := Dedupe(items); len(out) != 2 {
		t.Fatal("one module's own items are never deduplicated against each other")
	}
}

func TestVisibleDropsEmpty(t *testing.T) {
	out := Visible([]item.Item{{ID: "a", Bytes: 0}, {ID: "b", Bytes: -1}, {ID: "c", Bytes: 5}})
	if len(out) != 2 || out[0].ID != "b" {
		t.Fatalf("%+v", out)
	}
}

func TestCollectSurvivesAPanickingModule(t *testing.T) {
	bad := module.Module{ID: "bad", Name: "Bad", Scan: func(context.Context, *module.Env) module.Result { panic("boom") }}
	good := module.Module{ID: "good", Scan: func(context.Context, *module.Env) module.Result {
		return module.Result{Items: []item.Item{{ID: "g", Bytes: 1, Level: item.Safe}}}
	}}
	s := &Scanner{Env: &module.Env{}, Modules: []module.Module{bad, good}, NoFamily: true}
	_, items, warns := s.Collect(context.Background())
	if len(items) != 1 || len(warns) != 1 || items[0].Fingerprint == "" {
		t.Fatalf("items %+v warns %v", items, warns)
	}
}
