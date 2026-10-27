package item

import (
	"encoding/json"
	"fmt"
)

// Category groups items the way a developer thinks about their machine.
type Category int

const (
	Xcode Category = iota
	Go
	Homebrew
	Packages
	Containers
	Android
	Node
	Flutter
	Worktrees
	System
	numCategories
)

// Categories lists every category in display order.
func Categories() []Category {
	out := make([]Category, numCategories)
	for i := range out {
		out[i] = Category(i)
	}
	return out
}

var categoryInfo = [numCategories]struct {
	key, name, color string
}{
	Xcode:      {"xcode", "Xcode & Simulators", "#6E7CF5"},
	Go:         {"go", "Go", "#2EC4B6"},
	Homebrew:   {"homebrew", "Homebrew", "#F59E42"},
	Packages:   {"packages", "Package caches", "#B48CF2"},
	Containers: {"containers", "Containers", "#E879B9"},
	Android:    {"android", "Android & Gradle", "#8BD346"},
	Node:       {"node", "Node.js", "#E8C93A"},
	Flutter:    {"flutter", "Flutter & Dart", "#4CC3F2"},
	Worktrees:  {"worktrees", "Git worktrees", "#C9A27A"},
	System:     {"system", "System packages", "#9AA5B1"},
}

func (c Category) valid() bool { return c >= 0 && c < numCategories }

// Key is the stable machine name, used in JSON.
func (c Category) Key() string {
	if !c.valid() {
		return "unknown"
	}
	return categoryInfo[c].key
}

// String is the display name.
func (c Category) String() string {
	if !c.valid() {
		return "Other"
	}
	return categoryInfo[c].name
}

// Color is the category's truecolor hex; terminals with fewer colours get
// the nearest one.
func (c Category) Color() string {
	if !c.valid() {
		return "#888888"
	}
	return categoryInfo[c].color
}

func (c Category) MarshalJSON() ([]byte, error) { return json.Marshal(c.Key()) }

func (c *Category) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	for _, x := range Categories() {
		if x.Key() == s {
			*c = x
			return nil
		}
	}
	return fmt.Errorf("unknown category %q", s)
}
