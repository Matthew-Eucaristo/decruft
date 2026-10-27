// Package report prints a scan as a table or JSON, and a run as a receipt
// that keeps "moved to the Trash", "freed now" and "delegated" apart.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Matthew-Eucaristo/decruft/internal/act"
	"github.com/Matthew-Eucaristo/decruft/internal/disk"
	"github.com/Matthew-Eucaristo/decruft/internal/format"
	"github.com/Matthew-Eucaristo/decruft/internal/inventory"
	"github.com/Matthew-Eucaristo/decruft/internal/item"
	"github.com/Matthew-Eucaristo/decruft/internal/trash"
)

// Scan is everything a scan found.
type Scan struct {
	Disk     disk.Info
	Sources  []inventory.Source
	Items    []item.Item
	Warnings []string
	Home     string
	Version  string
}

// Total sums what items free, overall and for the allowed ones.
func Total(items []item.Item) (all, allowed, safe int64) {
	for _, it := range items {
		n := max(it.Size(), 0)
		all += n
		if it.Selectable() {
			allowed += n
			if it.Level == item.Safe {
				safe += n
			}
		}
	}
	return
}

// What is how an item would be removed, in one line.
func What(it *item.Item) string {
	switch it.Action.Kind {
	case item.Command:
		return "runs " + it.Action.Display()
	case item.Trash:
		return "moves the folder to the " + trash.Name
	case item.Delegate:
		return it.Delegate.Text
	case item.Manual:
		if len(it.Action.Argv) > 0 {
			return "run yourself: " + it.Action.Display()
		}
		return it.Action.How
	}
	return it.Action.How
}

// List prints a readable table, grouped by category, largest first.
func List(w io.Writer, s Scan) {
	tilde := func(p string) string { return format.Tilde(p, s.Home) }
	fmt.Fprintf(w, "decruft %s · %s\n", s.Version, DiskLine(s.Disk))
	for _, l := range Hidden(s.Disk, s.Items) {
		fmt.Fprintln(w, "  "+l)
	}
	all, allowed, safe := Total(s.Items)
	fmt.Fprintf(w, "Reclaimable: %s found · %s decruft can remove · %s safe\n", format.Bytes(all), format.Bytes(allowed), format.Bytes(safe))
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	cat := item.Category(-1)
	for _, it := range s.Items {
		if it.Category != cat {
			tw.Flush()
			cat = it.Category
			var n int64
			for _, x := range s.Items {
				if x.Category == cat {
					n += max(x.Size(), 0)
				}
			}
			fmt.Fprintf(w, "\n%s  %s\n", strings.ToUpper(cat.String()), format.Bytes(n))
		}
		size := format.Bytes(it.Size())
		if it.UpTo {
			size = "≤" + size
		}
		name := it.Name
		if it.Path != "" && it.Kind != "simctl-device" {
			name += "  " + tilde(it.Path)
		}
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n", size, it.Level, name, tildeAll(What(&it), s.Home))
	}
	tw.Flush()
	if len(s.Items) == 0 {
		fmt.Fprintln(w, "\nNothing to clean.")
	}
	var bits []string
	for _, src := range s.Sources {
		if src.Family && src.Status == inventory.Done {
			bits = append(bits, src.Name+" "+src.Version)
		}
	}
	if len(bits) > 0 {
		fmt.Fprintf(w, "\nFamily tools: %s\n", strings.Join(bits, ", "))
	}
	for _, warn := range s.Warnings {
		fmt.Fprintln(w, "warning: "+tildeAll(warn, s.Home))
	}
}

func tildeAll(s, home string) string { return format.TildeAll(s, home) }

// DiskLine is the volume, its free space and its size.
func DiskLine(d disk.Info) string {
	if d.Total <= 0 {
		return d.Name
	}
	return fmt.Sprintf("%s: %s free of %s", d.Name, format.Bytes(d.Free), format.Bytes(d.Total))
}

// Hidden explains what Settings does not: purgeable space, snapshots, swap,
// simulator runtimes counted as System Data, and the Trash.
func Hidden(d disk.Info, items []item.Item) []string {
	var out []string
	if d.Purgeable > 0 {
		out = append(out, fmt.Sprintf("Purgeable %s: macOS can free it by itself, and Finder counts it as available (%s).",
			format.Bytes(d.Purgeable), format.Bytes(d.Free+d.Purgeable)))
	}
	var sys int64
	var parts []string
	for _, it := range items {
		if it.SystemData {
			sys += max(it.Bytes, 0)
		}
	}
	if sys > 0 {
		parts = append(parts, "simulator runtimes "+format.Bytes(sys))
	}
	if d.Swap > 0 {
		parts = append(parts, "swap "+format.Bytes(d.Swap))
	}
	if n := len(d.Snapshots); n > 0 {
		parts = append(parts, fmt.Sprintf("%d local snapshot(s), see tmutil listlocalsnapshots /", n))
	}
	if len(parts) > 0 {
		out = append(out, "Inside “System Data”: "+strings.Join(parts, " · ")+".")
	}
	switch {
	case d.Trash > 0:
		out = append(out, fmt.Sprintf("The %s holds %s; it is freed only when you empty it.", trash.Name, format.Bytes(d.Trash)))
	case d.TrashNote != "":
		out = append(out, "Note: "+d.TrashNote+".")
	}
	return out
}

// Doc is the --json document.
type Doc struct {
	Schema      int                `json:"schema_version"`
	Tool        map[string]string  `json:"tool"`
	GeneratedAt time.Time          `json:"generated_at"`
	Disk        disk.Info          `json:"disk"`
	Sources     []inventory.Source `json:"sources"`
	Warnings    []string           `json:"warnings"`
	Summary     map[string]int64   `json:"summary"`
	Items       []item.Item        `json:"items"`
}

// JSON prints the scan with every home path written as "~/…".
func JSON(w io.Writer, s Scan) error {
	all, allowed, safe := Total(s.Items)
	items := make([]item.Item, len(s.Items))
	for i, it := range s.Items {
		it.Path = format.Tilde(it.Path, s.Home)
		it.Action.Argv = append([]string(nil), it.Action.Argv...)
		for j, a := range it.Action.Argv {
			it.Action.Argv[j] = format.Tilde(a, s.Home)
		}
		items[i] = it
	}
	warns := make([]string, 0, len(s.Warnings))
	for _, w := range s.Warnings {
		warns = append(warns, tildeAll(w, s.Home))
	}
	srcs := append([]inventory.Source(nil), s.Sources...)
	for i := range srcs {
		srcs[i].Error = tildeAll(srcs[i].Error, s.Home)
		srcs[i].Took /= time.Millisecond
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(Doc{
		Schema: 1, Tool: map[string]string{"name": "decruft", "version": s.Version},
		GeneratedAt: time.Now().UTC().Truncate(time.Second), Disk: s.Disk, Sources: srcs, Warnings: warns,
		Summary: map[string]int64{"items": int64(len(items)), "bytes": all, "removable_bytes": allowed, "safe_bytes": safe},
		Items:   items,
	})
}

// Receipt sums a run.
type Receipt struct {
	Before, After int64 // free space, -1 unknown
	Took          time.Duration
	Outcomes      []act.Outcome
	Regrows       bool
}

// Bucket is one line of the receipt.
type Bucket struct {
	Bytes int64
	Items int
	What  []string
}

// Buckets sorts outcomes by how they freed space.
func (r Receipt) Buckets() (trashed, deleted, freed, delegated, dry Bucket, refused, failed, claimed []act.Outcome) {
	tools := map[string]int{}
	cmds := map[string]bool{}
	for _, o := range r.Outcomes {
		switch o.How {
		case act.Trashed:
			trashed.Bytes += o.Estimate
			trashed.Items++
		case act.Deleted:
			deleted.Bytes += o.Estimate
			deleted.Items++
		case act.Freed:
			freed.Bytes += o.Estimate
			freed.Items++
			if !cmds[shortCommand(o.What)] {
				cmds[shortCommand(o.What)] = true
				freed.What = append(freed.What, shortCommand(o.What))
			}
			if o.Claimed {
				claimed = append(claimed, o)
			}
		case act.Delegated:
			delegated.Bytes += o.Estimate
			delegated.Items++
			tools[o.Source]++
		case act.Dry:
			dry.Bytes += o.Estimate
			dry.Items++
		case act.Refused:
			refused = append(refused, o)
		case act.Failed:
			failed = append(failed, o)
		}
	}
	names := make([]string, 0, len(tools))
	for t := range tools {
		names = append(names, t)
	}
	sort.Strings(names)
	for _, t := range names {
		delegated.What = append(delegated.What, fmt.Sprintf("%s ×%d", t, tools[t]))
	}
	return
}

// shortCommand drops the executable's folder: /opt/homebrew/bin/go → go.
func shortCommand(s string) string {
	f := strings.Fields(s)
	if len(f) == 0 {
		return s
	}
	if i := strings.LastIndexByte(f[0], '/'); i >= 0 {
		f[0] = f[0][i+1:]
	}
	if len(f) > 4 {
		f = f[:4]
	}
	return strings.Join(f, " ")
}

// Lines renders the receipt as plain lines (the TUI styles them itself).
func (r Receipt) Lines() []string {
	trashed, deleted, freed, delegated, dry, refused, failed, claimed := r.Buckets()
	var out []string
	row := func(mark, label string, b Bucket, note string) {
		out = append(out, fmt.Sprintf("%s %-20s %9s  %3d item(s)  %s", mark, label, format.Bytes(b.Bytes), b.Items, note))
	}
	if dry.Items > 0 {
		row("·", "Would free (dry run)", dry, "nothing changed")
	}
	if trashed.Items > 0 {
		row("✓", "Moved to "+trash.Name, trashed, "freed when you empty the "+trash.Name+"; Put Back restores")
	}
	if deleted.Items > 0 {
		row("✓", "Deleted", deleted, "permanently")
	}
	if freed.Items > 0 {
		what := freed.What
		if len(what) > 3 {
			what = append(what[:3:3], fmt.Sprintf("%d more", len(freed.What)-3))
		}
		row("✓", "Freed now", freed, strings.Join(what, " · "))
	}
	if delegated.Items > 0 {
		row("✓", "Delegated", delegated, strings.Join(delegated.What, " · ")+" (each tool's own rules)")
	}
	for _, o := range claimed {
		out = append(out, fmt.Sprintf("⚠ %-20s %9s  %s reported success, but free space did not grow by that much (yet)", "Claimed, not freed", format.Bytes(o.Estimate), shortCommand(o.What)))
	}
	for _, o := range refused {
		out = append(out, fmt.Sprintf("⚠ %-20s %9s  %s: %s", "Refused", format.Bytes(o.Estimate), o.Name, o.Error))
	}
	for _, o := range failed {
		out = append(out, fmt.Sprintf("✗ %-20s %9s  %s: %s", "Failed", format.Bytes(o.Estimate), o.Name, o.Error))
	}
	if r.Before >= 0 && r.After >= 0 && dry.Items == 0 && len(r.Outcomes) > 0 {
		line := fmt.Sprintf("  Free space  %s → %s   %s measured", format.Bytes(r.Before), format.Bytes(r.After), Signed(r.After-r.Before))
		if trashed.Items > 0 {
			line += fmt.Sprintf(" · about %s more when you empty the %s", format.Bytes(trashed.Bytes), trash.Name)
		}
		out = append(out, line)
	}
	if r.Regrows && dry.Items == 0 {
		out = append(out, "  ↺ Build caches (Go, Homebrew, DerivedData) come back as you work; that is normal.")
	}
	return out
}

// Print writes the receipt.
func (r Receipt) Print(w io.Writer) {
	if len(r.Outcomes) == 0 {
		return
	}
	fmt.Fprintf(w, "decruft · done in %s\n", r.Took.Round(time.Second))
	for _, l := range r.Lines() {
		fmt.Fprintln(w, "  "+l)
	}
}

// Signed formats a change in bytes with its sign.
func Signed(n int64) string {
	switch {
	case n > 0:
		return "+" + format.Bytes(n)
	case n < 0:
		return "−" + format.Bytes(-n)
	}
	return "±0 B"
}
