package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	"github.com/Matthew-Eucaristo/decruft/internal/act"
	"github.com/Matthew-Eucaristo/decruft/internal/disk"
	"github.com/Matthew-Eucaristo/decruft/internal/inventory"
	"github.com/Matthew-Eucaristo/decruft/internal/item"
	"github.com/Matthew-Eucaristo/decruft/internal/module"
)

// fixture is a fake machine: a home folder with one cache folder that a
// stand-in command removes, measured by a fake free-space counter.
type fixture struct {
	t    *testing.T
	home string
	mu   sync.Mutex
	free int64
	ran  [][]string
}

func newFixture(t *testing.T) *fixture {
	home := t.TempDir()
	f := &fixture{t: t, home: home, free: 10e9}
	if err := os.MkdirAll(filepath.Join(home, "Library", "Caches", "thing", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *fixture) env() *module.Env {
	return &module.Env{
		Home: f.home, OS: "test", Getenv: func(string) string { return "" },
		LookPath: func(string) (string, error) { return "", os.ErrNotExist },
		Output:   func(context.Context, []string, ...string) ([]byte, error) { return nil, os.ErrNotExist },
		Running:  func(string) bool { return false },
		Measure: func(p string) module.Measured {
			if _, err := os.Lstat(p); err != nil {
				return module.Measured{Missing: true}
			}
			return module.Measured{Bytes: 2e9, Frees: 2e9}
		},
	}
}

// mod offers the cache folder as a safe official-command item and a
// protected one, like a real module.
func (f *fixture) mod() module.Module {
	return module.Module{ID: "thing", Name: "Thing", Scan: func(_ context.Context, env *module.Env) module.Result {
		var r module.Result
		dir := filepath.Join(env.Home, "Library", "Caches", "thing")
		m := env.Measure(dir)
		if m.Missing {
			return r
		}
		it := item.Item{ID: "thing-1", Source: "thing", Category: item.Go, Kind: "thing-cache", Name: "Thing cache", Path: dir,
			Bytes: m.Bytes, Frees: m.Frees, Level: item.Safe, Action: item.Action{Kind: item.Command, Argv: []string{"/bin/thing", "clean"}},
			Explain: "A cache.", Regrows: true}
		p := item.Item{ID: "thing-2", Source: "thing", Category: item.Android, Kind: "gradle-home", Name: "Gradle caches",
			Path: filepath.Join(env.Home, ".gradle"), Bytes: 1e9, Frees: 1e9, Level: item.Protected, Action: item.Action{Kind: item.None, How: "no command"}}
		r.Items = append(r.Items, it, p)
		return r
	}}
}

func (f *fixture) model(mode act.Mode) *Model {
	env := f.env()
	mods := []module.Module{f.mod()}
	ex := &act.Executor{Mode: mode, Home: f.home, Env: env, Modules: mods,
		Run: func(_ context.Context, _ []string, argv []string) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.ran = append(f.ran, argv)
			f.free += 2e9
			return os.RemoveAll(filepath.Join(f.home, "Library", "Caches", "thing"))
		},
		Free: func(string) int64 { f.mu.Lock(); defer f.mu.Unlock(); return f.free },
	}
	sc := &inventory.Scanner{Env: env, Modules: mods, NoFamily: true}
	m := New(Options{Scanner: sc, Exec: ex, Home: f.home, Version: "test",
		Disk: func(context.Context, string) disk.Info {
			return disk.Info{Name: "Test", Total: 100e9, Free: f.free, Purgeable: -1, Swap: -1, Trash: -1}
		}})
	m.width, m.height = 110, 32
	return m
}

// drive runs commands like Bubble Tea would, feeding their messages back
// until nothing is left (ticks are dropped so the test ends).
func drive(m *Model, cmd tea.Cmd) {
	queue := []tea.Cmd{cmd}
	for steps := 0; len(queue) > 0 && steps < 500; steps++ {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		msg := c()
		switch msg := msg.(type) {
		case tea.BatchMsg:
			queue = append(queue, msg...)
			continue
		case nil, frameMsg:
			continue
		}
		if strings.Contains(fmt.Sprintf("%T", msg), "Tick") {
			continue
		}
		_, next := m.Update(msg)
		queue = append(queue, next)
	}
	// Settle the springs, as the frame clock would.
	for m.anim.step() {
	}
	m.animating = false
}

func press(m *Model, keys ...string) {
	for _, k := range keys {
		var msg tea.KeyPressMsg
		switch k {
		case "enter":
			msg = tea.KeyPressMsg{Code: tea.KeyEnter}
		case "esc":
			msg = tea.KeyPressMsg{Code: tea.KeyEscape}
		case "space":
			msg = tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
		case "down":
			msg = tea.KeyPressMsg{Code: tea.KeyDown}
		case "ctrl+c":
			msg = tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
		default:
			msg = tea.KeyPressMsg{Code: rune(k[0]), Text: k}
		}
		_, cmd := m.Update(msg)
		drive(m, cmd)
	}
}

func screenText(m *Model) string { return ansi.Strip(m.render()) }

func TestScanThenCleanShowsReceipt(t *testing.T) {
	f := newFixture(t)
	m := f.model(act.Trash)
	drive(m, m.Init())
	out := screenText(m)
	for _, want := range []string{"decruft", "Go", "2.0 GB", "Android & Gradle", "free of 100.0 GB"} {
		if !strings.Contains(out, want) {
			t.Fatalf("overview lacks %q:\n%s", want, out)
		}
	}
	press(m, "s", "c")
	if out := screenText(m); !strings.Contains(out, "Official commands") || !strings.Contains(out, "thing clean") {
		t.Fatalf("confirm sheet:\n%s", out)
	}
	press(m, "y")
	out = screenText(m)
	if len(f.ran) != 1 || f.ran[0][0] != "/bin/thing" {
		t.Fatalf("ran %v", f.ran)
	}
	for _, want := range []string{"Done in", "Freed now", "Before", "After", "+2.0 GB measured", "come back as you work"} {
		if !strings.Contains(out, want) {
			t.Fatalf("receipt lacks %q:\n%s", want, out)
		}
	}
	if r := m.Receipt(); r.After-r.Before != 2e9 {
		t.Fatalf("receipt delta %d", r.After-r.Before)
	}
}

func TestDryRunRunsNothing(t *testing.T) {
	f := newFixture(t)
	m := f.model(act.DryRun)
	drive(m, m.Init())
	press(m, "s", "c", "y")
	if len(f.ran) != 0 {
		t.Fatalf("dry run ran %v", f.ran)
	}
	if out := screenText(m); !strings.Contains(out, "Would free (dry run)") {
		t.Fatalf("receipt:\n%s", out)
	}
}

func TestProtectedNeverSelected(t *testing.T) {
	f := newFixture(t)
	m := f.model(act.Trash)
	drive(m, m.Init())
	// Select everything in every category, the protected one included.
	for range 3 {
		press(m, "space", "down")
	}
	run, _ := m.plan()
	for _, it := range run {
		if it.Level == item.Protected {
			t.Fatalf("protected item in plan: %s", it.Name)
		}
	}
	if m.selected["thing-2"] {
		t.Fatal("protected item selected")
	}
}

func TestNarrowAndNoColor(t *testing.T) {
	f := newFixture(t)
	m := f.model(act.Trash)
	m.Update(tea.ColorProfileMsg{Profile: colorprofile.Ascii})
	drive(m, m.Init())
	for _, w := range []int{40, 60, 80, 140} {
		m.width, m.height = w, 20
		for _, keys := range [][]string{nil, {"enter"}, {"t"}, {"esc", "esc"}, {"?"}, {"x"}} {
			press(m, keys...)
			for i, l := range strings.Split(m.render(), "\n") {
				if ansi.StringWidth(l) > w {
					t.Fatalf("width %d, line %d too wide (%d): %q", w, i, ansi.StringWidth(l), ansi.Strip(l))
				}
			}
		}
	}
}

func TestCtrlCStopsAfterCurrentStep(t *testing.T) {
	f := newFixture(t)
	m := f.model(act.Trash)
	drive(m, m.Init())
	press(m, "s", "c")
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	// The step is in flight; the first ctrl+c only stops the queue.
	m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if !m.stopping || !m.running {
		t.Fatal("first ctrl+c should stop after the running step")
	}
	drive(m, cmd)
	if m.running {
		t.Fatal("run should have finished")
	}
}

func TestTreemapNavigation(t *testing.T) {
	f := newFixture(t)
	m := f.model(act.Trash)
	drive(m, m.Init())
	press(m, "t")
	if m.screen != treemap || m.tmCat != -1 {
		t.Fatal("t opens the treemap of all categories")
	}
	press(m, "enter")
	if m.tmCat < 0 {
		t.Fatal("enter zooms into a category")
	}
	press(m, "esc", "esc")
	if m.screen != overview {
		t.Fatal("esc twice returns to the overview")
	}
}

func TestOverviewAfterRun(t *testing.T) {
	f := newFixture(t)
	m := f.model(act.DryRun)
	drive(m, m.Init())
	press(m, "s", "c", "y", "x")
	if m.screen != overview {
		t.Fatalf("screen %v", m.screen)
	}
	_ = screenText(m)
	press(m, "enter", "t", "esc", "esc")
	_ = screenText(m)
}
