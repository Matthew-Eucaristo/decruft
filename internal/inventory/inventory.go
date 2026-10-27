// Package inventory runs every built-in module and family tool in parallel,
// streams what each finds, and makes sure every byte is offered once.
package inventory

import (
	"context"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/Matthew-Eucaristo/decruft/internal/family"
	"github.com/Matthew-Eucaristo/decruft/internal/item"
	"github.com/Matthew-Eucaristo/decruft/internal/module"
	"github.com/Matthew-Eucaristo/decruft/internal/pathx"
)

// Status is where a source is in the scan.
type Status string

const (
	Scanning Status = "scanning"
	Done     Status = "done"
	Failed   Status = "failed"
	Skipped  Status = "skipped"
)

// Source is one module or family tool.
type Source struct {
	Name    string        `json:"name"`
	Family  bool          `json:"family"`
	Version string        `json:"version,omitempty"`
	Status  Status        `json:"status"`
	Error   string        `json:"error,omitempty"`
	Items   int           `json:"items"`
	Took    time.Duration `json:"took_ms"`
	Tool    *family.Tool  `json:"-"`
}

// Event reports one finished source.
type Event struct {
	Source   Source
	Items    []item.Item
	Warnings []string
}

// Scanner holds what a scan needs.
type Scanner struct {
	Env      *module.Env
	Family   *family.Family
	Modules  []module.Module
	Roots    []string // passed to the family tools
	NoFamily bool
}

// Names lists every source in display order, before any finishes.
func (s *Scanner) Names() []Source {
	var out []Source
	for _, m := range s.Modules {
		out = append(out, Source{Name: m.ID, Status: Scanning})
	}
	if !s.NoFamily {
		for _, k := range family.Known {
			out = append(out, Source{Name: k.Name, Family: true, Status: Scanning})
		}
	}
	return out
}

// Start scans every source in parallel and sends one Event per source, then
// closes the channel. Cancelling ctx stops the tools it runs.
func (s *Scanner) Start(ctx context.Context) <-chan Event {
	ch := make(chan Event)
	var wg sync.WaitGroup
	for _, m := range s.Modules {
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()
			r := safeScan(ctx, m, s.Env)
			for i := range r.Items {
				r.Items[i].Seal()
			}
			src := Source{Name: m.ID, Status: Done, Items: len(r.Items), Took: time.Since(start)}
			ch <- Event{Source: src, Items: r.Items, Warnings: r.Warnings}
		}()
	}
	if !s.NoFamily {
		for _, k := range family.Known {
			wg.Add(1)
			go func() {
				defer wg.Done()
				start := time.Now()
				src := Source{Name: k.Name, Family: true}
				t, warn, ok := s.Family.Discover(ctx, k.Name, k.Category)
				if !ok {
					src.Status, src.Error = Skipped, warn
					ch <- Event{Source: src, Warnings: []string{warn}}
					return
				}
				t.Roots = s.Roots
				src.Version, src.Tool = t.Version, &t
				items, warns, err := s.Family.Scan(ctx, t)
				src.Status, src.Items, src.Took = Done, len(items), time.Since(start)
				if err != nil {
					src.Status, src.Error = Failed, err.Error()
					warns = append(warns, err.Error())
				}
				ch <- Event{Source: src, Items: items, Warnings: warns}
			}()
		}
	}
	go func() { wg.Wait(); close(ch) }()
	return ch
}

// safeScan turns a module's panic into a warning, so one broken module
// (an unexpected tool output) cannot take the scan down.
func safeScan(ctx context.Context, m module.Module, env *module.Env) (r module.Result) {
	defer func() {
		if p := recover(); p != nil {
			r = module.Result{Warnings: []string{m.Name + ": scan failed unexpectedly; skipped"}}
		}
	}()
	return m.Scan(ctx, env)
}

// Collect runs a whole scan and returns the deduplicated items.
func (s *Scanner) Collect(ctx context.Context) ([]Source, []item.Item, []string) {
	sources := s.Names()
	var items []item.Item
	var warns []string
	for ev := range s.Start(ctx) {
		for i := range sources {
			if sources[i].Name == ev.Source.Name {
				sources[i] = ev.Source
			}
		}
		items = append(items, ev.Items...)
		warns = append(warns, ev.Warnings...)
	}
	items, dropped := Dedupe(Visible(items))
	if dropped > 0 {
		warns = append(warns, DedupeNote(dropped))
	}
	Sort(items)
	sort.Strings(warns)
	return sources, items, warns
}

// Visible drops items with nothing on disk. A worktree whose blocks are all
// APFS clones stays: it takes space on disk, even if removing it frees
// nothing right away.
func Visible(items []item.Item) []item.Item {
	var out []item.Item
	for _, it := range items {
		if it.Bytes != 0 {
			out = append(out, it)
		}
	}
	return out
}

// DedupeNote explains dropped duplicates.
func DedupeNote(n int) string {
	if n == 1 {
		return "1 item reported by two tools is shown once"
	}
	return strconv.Itoa(n) + " items reported by two tools are shown once"
}

// Sort orders items by category, then by what they free, largest first.
func Sort(items []item.Item) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.Category != b.Category {
			return a.Category < b.Category
		}
		if a.Size() != b.Size() {
			return a.Size() > b.Size()
		}
		return a.Name < b.Name
	})
}

// rank decides who owns overlapping bytes: a built-in module with an
// official command first, then the family tool, then a built-in that only
// informs (Gradle's folders), so a tool that can act is not hidden by one
// that cannot.
func rank(it *item.Item) int {
	builtin := it.Delegate == nil && it.Action.Kind != item.Handoff
	switch {
	case builtin && it.Action.Kind != item.None:
		return 0
	case !builtin:
		return 1
	}
	return 2
}

// aliases pairs path-less items that mean the same thing.
var aliases = map[string]string{"simulators-unavailable": "simctl-unavailable"}

func toolOrder(source string) int {
	for i, k := range family.Known {
		if k.Name == source {
			return i
		}
	}
	return -1
}

// Dedupe keeps one item for every set of overlapping paths (the same path,
// or one inside the other) reported by different sources. Built-in modules
// win, then the outer item: fell's worktree over nodekill's node_modules
// inside it, so the same bytes are never offered twice.
func Dedupe(items []item.Item) ([]item.Item, int) {
	drop := make([]bool, len(items))
	kinds := map[string]bool{}
	for _, it := range items {
		if rank(&it) == 0 {
			kinds[it.Kind] = true
		}
	}
	for i := range items {
		a := &items[i]
		if alias, ok := aliases[a.Kind]; ok && kinds[alias] && rank(a) > 0 {
			drop[i] = true
		}
	}
	// Tools may spell the same folder through a symlink (/tmp, /private/tmp).
	real := make([]string, len(items))
	for i, it := range items {
		if it.Path != "" {
			real[i] = pathx.Real(it.Path)
		}
	}
	for i := range items {
		for j := i + 1; j < len(items); j++ {
			a, b := &items[i], &items[j]
			if drop[i] || drop[j] || a.Source == b.Source || a.Path == "" || b.Path == "" {
				continue
			}
			aInB, bInA := pathx.SameOrWithin(real[j], real[i]), pathx.SameOrWithin(real[i], real[j])
			if !aInB && !bInA {
				continue
			}
			// loser is i or j; a wins ties on the same path by tool order.
			loser := j
			switch ra, rb := rank(a), rank(b); {
			case ra != rb:
				if ra > rb {
					loser = i
				}
			case aInB && bInA:
				if toolOrder(a.Source) > toolOrder(b.Source) {
					loser = i
				}
			case aInB:
				loser = i
			}
			drop[loser] = true
		}
	}
	var out []item.Item
	for i, it := range items {
		if !drop[i] {
			out = append(out, it)
		}
	}
	return out, len(items) - len(out)
}
