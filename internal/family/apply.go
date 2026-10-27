package family

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Matthew-Eucaristo/decruft/internal/item"
)

// Mode is how a tool should remove things.
type Mode int

const (
	Trash Mode = iota
	DryRun
	Permanent
)

// Result is what a tool did with one item.
type Result struct {
	ID        string   `json:"id"` // the tool's own ID
	Status    string   `json:"status"`
	Action    string   `json:"action"`
	Error     string   `json:"error"`
	Reasons   []string `json:"reasons"`
	Estimated int64    `json:"estimated_bytes"`
	Freed     int64    `json:"freed_bytes_measured"`
}

// Apply hands items back to their tool as one plan. Consent comes only from
// the flags decruft passes for the user's own mode: never --force or
// --unlock, so the tool's guards decide.
func (f *Family) Apply(ctx context.Context, t Tool, items []item.Item, mode Mode) ([]Result, error) {
	if !t.Contract {
		return nil, fmt.Errorf("%s has no plan/apply contract; open it to clean", t.Name)
	}
	type entry struct {
		ID          string `json:"id"`
		Fingerprint string `json:"fingerprint"`
		Action      string `json:"action,omitempty"`
	}
	plan := struct {
		Schema int     `json:"schema_version"`
		Items  []entry `json:"items"`
	}{Schema: Schema}
	for _, it := range items {
		if it.Delegate == nil || it.Delegate.Tool != t.Name {
			return nil, fmt.Errorf("%s is not %s's item", it.Name, t.Name)
		}
		plan.Items = append(plan.Items, entry{it.Delegate.ID, it.Delegate.Fingerprint, it.Delegate.Action})
	}
	in, err := json.Marshal(plan)
	if err != nil {
		return nil, err
	}
	args := []string{t.Path, "apply", "--plan", "-", "--json"}
	switch mode {
	case DryRun:
		args = append(args, "--dry-run")
	case Permanent:
		args = append(args, "--permanent")
	}
	args = append(args, t.Roots...)
	out, code, err := f.Run(ctx, in, args...)
	if err != nil {
		return nil, fmt.Errorf("%s apply: %w", t.Name, err)
	}
	var doc struct {
		Schema  int      `json:"schema_version"`
		Results []Result `json:"results"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		return nil, fmt.Errorf("%s apply (exit %d): unreadable result: %v", t.Name, code, err)
	}
	if doc.Schema != Schema {
		return nil, fmt.Errorf("%s apply: contract version %d, expected %d", t.Name, doc.Schema, Schema)
	}
	return doc.Results, nil
}
