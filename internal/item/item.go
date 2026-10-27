// Package item defines what decruft shows: one reclaimable thing, who owns
// it, how safe it is and exactly how it would be removed.
package item

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// Level is how freely an item may be removed. The words are the family's.
type Level string

const (
	Safe      Level = "safe"      // rebuildable by its tool; may be bulk-selected
	Review    Level = "review"    // allowed when chosen one by one, never in bulk
	Protected Level = "protected" // shown for context; decruft never acts on it
	Live      Level = "live"      // in use right now; nothing is allowed
)

// ParseLevel maps a family tool's level to decruft's. Anything unknown is
// protected, so a new word in a tool's output can never unlock an item.
func ParseLevel(s string) Level {
	switch s {
	case "safe":
		return Safe
	case "review", "caution":
		return Review
	case "live":
		return Live
	}
	return Protected
}

// Allowed reports whether items at this level may be acted on at all.
func (l Level) Allowed() bool { return l == Safe || l == Review }

// Rank orders levels from most to least permissive.
func (l Level) Rank() int {
	switch l {
	case Safe:
		return 0
	case Review:
		return 1
	case Protected:
		return 2
	}
	return 3
}

// Kind is how an item frees its space.
type Kind string

const (
	Command  Kind = "command"  // the owning tool's official cleanup; deletes by its own design
	Trash    Kind = "trash"    // a folder moved to the trash (deleted with --permanent)
	Delegate Kind = "delegate" // handed to a family tool through its plan/apply contract
	Handoff  Kind = "handoff"  // a family tool without a contract: open it to clean
	Manual   Kind = "manual"   // needs sudo or an app's own UI: shown, never run
	None     Kind = "none"     // information only
)

// Action is exactly what removing the item would do.
type Action struct {
	Kind Kind     `json:"kind"`
	Argv []string `json:"argv,omitempty"` // the command decruft runs (Command) or shows (Manual)
	Env  []string `json:"-"`              // extra environment for Argv
	// How explains a Manual or None action in words ("Xcode › Settings › Components").
	How string `json:"how,omitempty"`
}

// CommandLine is Argv as a shell would show it.
func (a Action) CommandLine() string {
	parts := make([]string, len(a.Argv))
	for i, s := range a.Argv {
		if s == "" || strings.ContainsAny(s, " \t\"'$;&|<>()*?[]{}~`\\") {
			s = "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
		}
		parts[i] = s
	}
	return strings.Join(parts, " ")
}

// Display is the command with its executable's folder dropped, for people:
// /opt/homebrew/bin/go clean -cache → go clean -cache.
func (a Action) Display() string {
	b := a
	b.Argv = append([]string(nil), a.Argv...)
	if len(b.Argv) > 0 {
		b.Argv[0] = b.Argv[0][strings.LastIndexAny(b.Argv[0], `/\`)+1:]
	}
	return b.CommandLine()
}

// Reason is one piece of evidence behind an item's level.
type Reason struct {
	Code   string `json:"code"`
	Level  Level  `json:"level"`
	Detail string `json:"detail"`
}

// Delegation ties an item to the family tool that owns it.
type Delegation struct {
	Tool        string `json:"tool"`
	ID          string `json:"id"`
	Fingerprint string `json:"fingerprint"`
	Action      string `json:"action,omitempty"` // fell's strip/retire; empty when implied
	Text        string `json:"text"`             // what the tool will do, in words
}

// Item is one reclaimable thing.
type Item struct {
	ID       string   `json:"id"`
	Source   string   `json:"source"` // built-in module ID or family tool name
	Category Category `json:"category"`
	Kind     string   `json:"kind"`
	Name     string   `json:"name"`
	Path     string   `json:"path,omitempty"`
	// Bytes is the space it takes on disk, -1 when unknown.
	Bytes int64 `json:"bytes"`
	// Frees is what removing it frees right away: its own blocks, without
	// those shared with APFS clones, snapshots or hard links. -1 unknown.
	Frees int64 `json:"frees"`
	// UpTo marks Frees as an upper bound (prune commands keep what is in use).
	UpTo bool `json:"up_to,omitempty"`
	// SystemData marks bytes macOS files under "System Data" in Settings.
	SystemData bool        `json:"system_data,omitempty"`
	Level      Level       `json:"level"`
	Reasons    []Reason    `json:"reasons,omitempty"`
	Action     Action      `json:"action"`
	Delegate   *Delegation `json:"delegate,omitempty"`
	Explain    string      `json:"explain"`
	Regenerate string      `json:"regenerate,omitempty"`
	// Regrows marks working-set caches that come back as soon as you build.
	Regrows bool `json:"regrows,omitempty"`
	// Rebuilt is when the cache folder was last created, a hint at regrowth.
	Rebuilt     time.Time `json:"rebuilt,omitempty"`
	Note        string    `json:"note,omitempty"`
	Fingerprint string    `json:"fingerprint"`
	// Priority orders a run: lower goes first (brew autoremove before brew
	// cleanup, so cleanup also takes what autoremove left behind).
	Priority int `json:"-"`
	// Evidence is extra safety-relevant state for the fingerprint.
	Evidence []string `json:"-"`
}

// Selectable reports whether decruft can act on the item itself.
func (it *Item) Selectable() bool {
	switch it.Action.Kind {
	case Command, Trash, Delegate:
		return it.Level.Allowed()
	}
	return false
}

// Size is what the item frees, falling back to its size on disk.
func (it *Item) Size() int64 {
	if it.Frees >= 0 {
		return it.Frees
	}
	return it.Bytes
}

// Add appends a reason and lowers the level to the strictest one seen.
func (it *Item) Add(code string, l Level, detail string) {
	it.Reasons = append(it.Reasons, Reason{code, l, detail})
	if l.Rank() > it.Level.Rank() {
		it.Level = l
	}
}

// Has reports whether the item carries a reason code.
func (it *Item) Has(code string) bool {
	for _, r := range it.Reasons {
		if r.Code == code {
			return true
		}
	}
	return false
}

// Seal computes the fingerprint from the safety-relevant state and the
// item's Evidence (a directory's identity, a device's state). Sizes are left
// out, so a cache growing between scan and apply does not invalidate a plan.
func (it *Item) Seal() {
	h := sha256.New()
	for _, s := range append([]string{it.ID, it.Kind, it.Path, string(it.Level), string(it.Action.Kind), strings.Join(it.Action.Argv, "\x00")}, it.Evidence...) {
		h.Write([]byte(s))
		h.Write([]byte{0})
	}
	it.Fingerprint = "fp-" + hex.EncodeToString(h.Sum(nil))[:16]
}

// ID builds a stable item ID from a module and a key.
func ID(source, key string) string {
	h := sha256.Sum256([]byte(source + "\x00" + key))
	return source + "-" + hex.EncodeToString(h[:])[:10]
}
