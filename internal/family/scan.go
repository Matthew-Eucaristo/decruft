package family

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Matthew-Eucaristo/decruft/internal/item"
)

type sizes struct {
	Allocated int64  `json:"allocated"`
	Shared    int64  `json:"shared"`
	Exclusive *int64 `json:"exclusive"`
}

func (s sizes) exclusive() int64 {
	if s.Exclusive != nil {
		return *s.Exclusive
	}
	return s.Allocated - s.Shared
}

// contractItem is the union of the family's item fields that decruft reads.
// Unknown fields are ignored, as the contract asks.
type contractItem struct {
	ID          string   `json:"id"`
	Fingerprint string   `json:"fingerprint"`
	Kind        string   `json:"kind"`
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	Measured    *bool    `json:"measured"`
	Bytes       sizes    `json:"bytes"`
	Strippable  *sizes   `json:"strippable_bytes"`
	Action      string   `json:"action"`  // nodekill: command or trash
	Command     []string `json:"command"` // nodekill: the argv it runs
	Explain     string   `json:"explain"`
	Regenerate  string   `json:"regenerate"`
	Note        string   `json:"note"`
	Owner       *struct {
		Name        string `json:"name"`
		HowToRemove string `json:"how_to_remove"`
	} `json:"owner"`
	Project *struct {
		Name string `json:"name"`
	} `json:"project"`
	Safety struct {
		Level   string `json:"level"`
		Reasons []struct {
			Code   string `json:"code"`
			Level  string `json:"level"`
			Detail string `json:"detail"`
		} `json:"reasons"`
	} `json:"safety"`
	// fell: one decision per action.
	Actions []struct {
		Action  string `json:"action"`
		Allowed bool   `json:"allowed"`
	} `json:"actions"`
}

// Scan asks a tool what it sees and turns that into decruft items.
func (f *Family) Scan(ctx context.Context, t Tool) ([]item.Item, []string, error) {
	if !t.Contract {
		return f.scanLegacy(ctx, t)
	}
	args := append([]string{t.Path, "scan", "--json"}, t.Roots...)
	out, _, err := f.Run(ctx, nil, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("%s scan: %w", t.Name, err)
	}
	var doc struct {
		Schema   int            `json:"schema_version"`
		Warnings []string       `json:"warnings"`
		Items    []contractItem `json:"items"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		return nil, nil, fmt.Errorf("%s scan: unreadable output (%v); skipped", t.Name, err)
	}
	if doc.Schema != Schema {
		return nil, nil, fmt.Errorf("%s scan: contract version %d, expected %d; skipped", t.Name, doc.Schema, Schema)
	}
	var items []item.Item
	for _, c := range doc.Items {
		if c.ID == "" || c.Fingerprint == "" {
			continue // cannot be planned, so cannot be offered
		}
		items = append(items, f.convert(t, c))
	}
	var warns []string
	for _, w := range doc.Warnings {
		warns = append(warns, t.Name+": "+w)
	}
	return items, warns, nil
}

func (f *Family) convert(t Tool, c contractItem) item.Item {
	it := item.Item{
		ID:          t.Name + ":" + c.ID,
		Source:      t.Name,
		Category:    t.Category,
		Kind:        c.Kind,
		Name:        c.Name,
		Path:        f.expand(c.Path),
		Bytes:       c.Bytes.Allocated,
		Frees:       c.Bytes.exclusive(),
		Level:       item.ParseLevel(c.Safety.Level),
		Explain:     c.Explain,
		Regenerate:  c.Regenerate,
		Note:        c.Note,
		Fingerprint: c.Fingerprint,
	}
	if c.Measured != nil && !*c.Measured {
		it.Bytes, it.Frees = -1, -1
	}
	for _, r := range c.Safety.Reasons {
		l := it.Level
		if r.Level != "" {
			l = item.ParseLevel(r.Level)
		}
		it.Reasons = append(it.Reasons, item.Reason{Code: r.Code, Level: l, Detail: r.Detail})
	}
	d := &item.Delegation{Tool: t.Name, ID: c.ID, Fingerprint: c.Fingerprint}
	switch {
	case c.Actions != nil: // fell: retire when it may, else strip
		allowed := map[string]bool{}
		for _, a := range c.Actions {
			allowed[a.Action] = a.Allowed
		}
		switch {
		case allowed["retire"]:
			d.Action, d.Text = "retire", "fell retire: saves a rescue bundle, moves the checkout to the Trash, then prunes git's record"
		case allowed["strip"] && c.Strippable != nil && c.Strippable.Allocated > 0:
			d.Action, d.Text = "strip", "fell strip: moves the rebuildable artifacts to the Trash; the worktree stays"
			it.Bytes, it.Frees = c.Strippable.Allocated, c.Strippable.exclusive()
		}
		if it.Name == "" && c.Project != nil {
			it.Name = c.Project.Name
		}
		if c.Project != nil && c.Name != "" {
			it.Name = c.Project.Name + " · " + c.Name
		}
		if it.Explain == "" {
			it.Explain = "A git worktree" + ownerText(c)
		}
	case c.Action == "command" && len(c.Command) > 0:
		d.Text = t.Name + " runs: " + strings.Join(c.Command, " ")
	case c.Action == "trash":
		d.Text = t.Name + " moves it to the Trash"
	}
	switch {
	case d.Text != "" && it.Level.Allowed():
		it.Action = item.Action{Kind: item.Delegate}
		it.Delegate = d
	case c.Owner != nil && c.Owner.HowToRemove != "":
		it.Action = item.Action{Kind: item.Manual, How: c.Owner.HowToRemove}
	default:
		it.Action = item.Action{Kind: item.None, How: t.Name + " leaves this alone"}
	}
	if d.Text != "" {
		it.Delegate = d
	}
	return it
}

func ownerText(c contractItem) string {
	if c.Owner == nil || c.Owner.Name == "" {
		return "."
	}
	return " made by " + c.Owner.Name + "."
}

// scanLegacy reads flutterkill 0.2's --json, which has no ids or
// fingerprints: its items are shown, and cleaning opens flutterkill itself.
func (f *Family) scanLegacy(ctx context.Context, t Tool) ([]item.Item, []string, error) {
	args := []string{t.Path, "--json"}
	if len(t.Roots) > 0 {
		args = append(args, t.Roots[0])
	}
	out, _, err := f.Run(ctx, nil, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("%s --json: %w", t.Name, err)
	}
	var doc struct {
		Projects []struct {
			Name      string `json:"name"`
			Path      string `json:"path"`
			Artifacts []struct {
				Name  string `json:"name"`
				Path  string `json:"path"`
				Bytes int64  `json:"bytes"`
			} `json:"artifacts"`
		} `json:"projects"`
		Caches []struct {
			ID      string   `json:"id"`
			Name    string   `json:"name"`
			Path    string   `json:"path"`
			Bytes   int64    `json:"bytes"`
			Command []string `json:"command"`
			Note    string   `json:"note"`
		} `json:"caches"`
	}
	if err := json.Unmarshal(out, &doc); err != nil || (doc.Projects == nil && doc.Caches == nil) {
		return nil, nil, fmt.Errorf("%s --json: unrecognised output; skipped", t.Name)
	}
	how := "flutterkill " + t.Version + " has no plan/apply contract yet: press o to open it and clean there"
	var items []item.Item
	for _, p := range doc.Projects {
		var bytes int64
		var names []string
		for _, a := range p.Artifacts {
			if a.Bytes > 0 {
				bytes += a.Bytes
				names = append(names, a.Name)
			}
		}
		if bytes == 0 {
			continue
		}
		it := item.Item{
			ID: item.ID("flutterkill", p.Path), Source: t.Name, Category: t.Category, Kind: "flutter-project",
			Name: p.Name, Path: f.expand(p.Path), Bytes: bytes, Frees: -1, Level: item.Safe,
			Explain:    "Build outputs of a Flutter/Dart project: " + strings.Join(names, ", ") + ".",
			Regenerate: "flutter pub get / flutter build",
			Action:     item.Action{Kind: item.Handoff, How: how},
		}
		it.Seal()
		items = append(items, it)
	}
	for _, c := range doc.Caches {
		if c.Bytes <= 0 {
			continue
		}
		it := item.Item{
			ID: item.ID("flutterkill", c.ID), Source: t.Name, Category: legacyCategory(c.ID, t.Category), Kind: c.ID,
			Name: c.Name, Path: f.expand(c.Path), Bytes: c.Bytes, Frees: -1, Level: item.Review,
			Explain: c.Note, Action: item.Action{Kind: item.Handoff, How: how},
		}
		if len(c.Command) > 0 {
			it.Note = "flutterkill runs: " + strings.Join(c.Command, " ")
		}
		it.Seal()
		items = append(items, it)
	}
	return items, nil, nil
}

// legacyCategory files flutterkill's shared caches where a developer looks
// for them: Gradle under Android, Xcode's under Xcode.
func legacyCategory(id string, def item.Category) item.Category {
	switch {
	case strings.HasPrefix(id, "gradle"), strings.HasPrefix(id, "android-studio"):
		return item.Android
	case strings.HasPrefix(id, "xcode"), strings.HasPrefix(id, "simulator"):
		return item.Xcode
	case id == "cocoapods-cache", id == "swiftpm-cache":
		return item.Packages
	}
	return def
}
