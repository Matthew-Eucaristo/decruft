package ui

import (
	"fmt"
	"image/color"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Matthew-Eucaristo/decruft/internal/act"
	"github.com/Matthew-Eucaristo/decruft/internal/format"
	"github.com/Matthew-Eucaristo/decruft/internal/inventory"
	"github.com/Matthew-Eucaristo/decruft/internal/item"
	"github.com/Matthew-Eucaristo/decruft/internal/report"
	"github.com/Matthew-Eucaristo/decruft/internal/trash"
)

const detailRows = 4

func (m *Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.WindowTitle = "decruft"
	return v
}

func (m *Model) tilde(p string) string { return format.Tilde(p, m.opts.Home) }

// fit pads or cuts lines to exactly n.
func fit(lines []string, n int) []string {
	out := make([]string, n)
	copy(out, lines)
	return out
}

func (m *Model) render() string {
	var top []string
	top = append(top, m.header())
	top = append(top, m.diskBar()...)
	var body, foot []string
	switch m.screen {
	case legend:
		body = m.legendLines()
		foot = []string{faint.Render("  any key closes")}
	case confirming:
		body = m.confirmLines()
		foot = []string{m.confirmHelp()}
	case receipt:
		body = m.receiptLines()
		foot = []string{faint.Render("  any key returns to the overview  ·  q quit")}
	case listing:
		body = m.listLines()
		foot = append(m.detailPane(m.listItem()), m.summary(), m.keyHints())
	case treemap:
		foot = append(m.detailPane(m.tmItem()), m.summary(), m.keyHints())
		body = m.treemapLines(max(3, m.height-len(top)-len(foot)))
	default:
		body = m.overviewLines()
		foot = []string{m.summary(), m.keyHints()}
	}
	room := max(1, m.height-len(top)-len(foot))
	lines := append(append(top, fit(body, room)...), foot...)
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, m.width, "…")
	}
	return strings.Join(lines, "\n")
}

// header: brand, mode, disk and scan progress, dropping the least
// important parts when the terminal is narrow.
func (m *Model) header() string {
	mode := green.Render("● trash")
	switch m.opts.Exec.Mode {
	case act.DryRun:
		mode = yellow.Render("● dry run")
	case act.Permanent:
		mode = banner.Render("PERMANENT DELETE")
	}
	type part struct {
		text string
		drop int
	}
	parts := []part{{brand.Render("decruft") + " " + mode, 0}}
	if m.haveDisk && m.disk.Total > 0 {
		parts = append(parts, part{bold.Render(format.Bytes(int64(m.anim.get("free")))) + faint.Render(" free of "+format.Bytes(m.disk.Total)), 1},
			part{faint.Render(m.disk.Name), 3})
	}
	if m.scanning {
		var wait []string
		for _, s := range m.sources {
			if s.Status == inventory.Scanning {
				wait = append(wait, s.Name)
			}
		}
		parts = append(parts, part{m.spin.View() + faint.Render(" scanning "+strings.Join(wait, ", ")), 2})
	}
	if m.running {
		parts = append(parts, part{m.spin.View() + yellow.Render(fmt.Sprintf(" cleaning %d/%d", min(m.stepAt+1, len(m.steps)), len(m.steps))), 0})
	}
	for drop := 4; ; drop-- {
		var keep []string
		for _, p := range parts {
			if p.drop < drop {
				keep = append(keep, p.text)
			}
		}
		if s := " " + strings.Join(keep, faint.Render("  ·  ")); lipgloss.Width(s) <= m.width || drop == 1 {
			return s
		}
	}
}

// presentCats lists categories that have items, largest first.
func (m *Model) presentCats() []item.Category {
	var cats []item.Category
	for _, c := range item.Categories() {
		for i := range m.items {
			if m.items[i].Category == c {
				cats = append(cats, c)
				break
			}
		}
	}
	sort.SliceStable(cats, func(i, j int) bool { return m.catBytes(cats[i]) > m.catBytes(cats[j]) })
	return cats
}

// diskBar is the Apple-style storage bar: developer categories in colour,
// everything else on the disk, then free space; and its legend.
func (m *Model) diskBar() []string {
	w := max(10, m.width-2)
	if !m.haveDisk || m.disk.Total <= 0 {
		return []string{"", " " + faint.Render(strings.Repeat("░", w)), ""}
	}
	cats := m.presentCats()
	var segs []segment
	var dev float64
	for _, c := range cats {
		n := m.anim.get(catKey(c))
		dev += n
		segs = append(segs, segment{bytes: n, color: lipgloss.Color(c.Color()), mono: "█"})
	}
	free := m.anim.get("free")
	other := max(0, float64(m.disk.Total)-free-dev)
	segs = append(segs, segment{bytes: other, color: m.pal.other, mono: "▒"}, segment{bytes: free, color: m.pal.free, mono: "░"})
	bar := " " + m.pal.storageBar(segs, w)

	var leg []string
	for _, c := range cats {
		if n := m.catBytes(c); n > 0 {
			leg = append(leg, m.swatch(c)+" "+c.String()+" "+faint.Render(format.Bytes(n)))
		}
	}
	var tail []string
	if other >= 1e6 {
		tail = append(tail, m.swatchColor(m.pal.other, "▒")+" "+faint.Render("everything else "+format.Bytes(int64(other))))
	}
	tail = append(tail, m.swatchColor(m.pal.free, "░")+" "+faint.Render("free "+format.Bytes(int64(free))))
	// Keep the legend on one line: drop the smallest categories first.
	for len(leg) > 0 && lipgloss.Width(" "+strings.Join(append(append([]string{}, leg...), tail...), "   ")) > m.width {
		leg = leg[:len(leg)-1]
	}
	return []string{bar, bar, " " + strings.Join(append(leg, tail...), "   ")}
}

func (m *Model) swatch(c item.Category) string {
	return m.swatchColor(lipgloss.Color(c.Color()), "■")
}

func (m *Model) swatchColor(col color.Color, mono string) string {
	if m.pal.mono {
		return mono
	}
	return lipgloss.NewStyle().Foreground(col).Render("■")
}

// nowFunc is the clock, replaced in tests.
var nowFunc = time.Now

// --- overview ----------------------------------------------------------------

func (m *Model) overviewLines() []string {
	lines := []string{""}
	cats := m.presentCats()
	if len(cats) == 0 {
		if m.scanning {
			return append(lines, "  "+faint.Render("Looking through Go, Xcode, Homebrew, package caches, Docker, Android and the family tools …"))
		}
		return append(lines, "  "+faint.Render("Nothing to clean. Every area decruft knows is tidy."))
	}
	if !m.moved || !containsCat(cats, m.curCat) {
		m.curCat = cats[0]
	}
	bw := 24
	wide := m.width >= 100
	switch {
	case m.width < 72:
		bw = 0
	case m.width < 100:
		bw = 14
	}
	head := fmt.Sprintf("      %-20s %9s  ", "CATEGORY", "FREES")
	if bw > 0 {
		head += strings.Repeat(" ", bw) + "  "
	}
	if wide {
		head += fmt.Sprintf("%9s %9s  ", "SAFE", "REVIEW")
	}
	lines = append(lines, faint.Render(head+"WHAT"))
	var largest float64 = 1
	for _, c := range cats {
		largest = max(largest, m.anim.get(catKey(c)))
	}
	for _, c := range cats {
		lines = append(lines, m.catRow(c, bw, wide, largest))
	}
	if wins := m.biggestWins(5); len(wins) > 0 {
		lines = append(lines, "", "  "+heading.Render("Biggest safe wins")+faint.Render("  ·  s selects every safe item"))
		lines = append(lines, wins...)
	}
	if hidden := report.Hidden(m.disk, m.items); m.haveDisk && len(hidden) > 0 {
		lines = append(lines, "", "  "+heading.Render("What macOS does not show you"))
		for _, h := range hidden {
			lines = append(lines, "  "+faint.Render(h))
		}
	}
	lines = append(lines, "", m.sourcesLine())
	if w := m.warningLine(); w != "" {
		lines = append(lines, w)
	}
	return lines
}

// biggestWins lists the largest safe items across categories: the quick
// answer to "what should I clean first?".
func (m *Model) biggestWins(n int) []string {
	var its []*item.Item
	for i := range m.items {
		it := &m.items[i]
		if it.Selectable() && it.Level == item.Safe && m.status[it.ID] != done && it.Size() > 0 {
			its = append(its, it)
		}
	}
	sort.SliceStable(its, func(i, j int) bool { return its[i].Size() > its[j].Size() })
	var out []string
	for _, it := range its[:min(n, len(its))] {
		out = append(out, fmt.Sprintf("    %s %s %10s  %-34s %s", m.check([]*item.Item{it}), m.swatch(it.Category),
			sizeText(it), ansi.Truncate(it.Name, 34, "…"), faint.Render(m.actionTag(it))))
	}
	return out
}

// sourcesLine shows each module and family tool with its state.
func (m *Model) sourcesLine() string {
	var parts []string
	for _, s := range m.sources {
		name := s.Name
		if s.Version != "" {
			name += " " + s.Version
		}
		switch s.Status {
		case inventory.Scanning:
			parts = append(parts, m.spin.View()+" "+name)
		case inventory.Done:
			if s.Items == 0 && !s.Family {
				parts = append(parts, faint.Render("· "+name))
				continue
			}
			parts = append(parts, green.Render("✓")+" "+name)
		case inventory.Failed:
			parts = append(parts, red.Render("✗")+" "+name)
		default:
			parts = append(parts, faint.Render("– "+s.Name+" (not installed)"))
		}
	}
	return "  " + faint.Render("Sources  ") + strings.Join(parts, faint.Render("  "))
}

func containsCat(cs []item.Category, c item.Category) bool {
	for _, x := range cs {
		if x == c {
			return true
		}
	}
	return false
}

func (m *Model) catItems(c item.Category) []*item.Item {
	var out []*item.Item
	for i := range m.items {
		if m.items[i].Category == c {
			out = append(out, &m.items[i])
		}
	}
	return out
}

func (m *Model) catRow(c item.Category, bw int, wide bool, largest float64) string {
	cur := " "
	if c == m.curCat {
		cur = cursorC
	}
	its := m.catItems(c)
	var safe, review int64
	var n, regrows int
	sources := map[string]bool{}
	for _, it := range its {
		if m.status[it.ID] == done {
			continue
		}
		n++
		if it.Regrows {
			regrows++
		}
		if it.Delegate != nil || it.Action.Kind == item.Handoff {
			sources[it.Source] = true
		}
		if it.Selectable() {
			switch it.Level {
			case item.Safe:
				safe += max(it.Size(), 0)
			case item.Review:
				review += max(it.Size(), 0)
			}
		}
	}
	if n == 0 { // everything in it was cleaned this session
		return fmt.Sprintf("%s %s %s %s%s  %s", cur, green.Render("✓"), m.swatch(c), struck.Render(c.String()),
			strings.Repeat(" ", max(0, 20-lipgloss.Width(c.String()))), green.Render("cleaned"))
	}
	row := fmt.Sprintf("%s %s %s %-20s %9s  ", cur, m.check(its), m.swatch(c), c.String(), bold.Render(format.Bytes(m.catBytes(c))))
	if bw > 0 {
		b := fine(m.anim.get(catKey(c)), largest, bw)
		bar := lipgloss.NewStyle().Foreground(lipgloss.Color(c.Color())).Render(b)
		if m.pal.mono {
			bar = b
		}
		row += bar + strings.Repeat(" ", bw-lipgloss.Width(b)) + "  "
	}
	dash := func(n int64) string {
		if n == 0 {
			return "—"
		}
		return format.Bytes(n)
	}
	if wide {
		row += green.Render(fmt.Sprintf("%9s", dash(safe))) + " " + yellow.Render(fmt.Sprintf("%9s", dash(review))) + "  "
	}
	var what []string
	what = append(what, fmt.Sprintf("%d item%s", n, plural(n)))
	if len(sources) > 0 {
		var names []string
		for s := range sources {
			names = append(names, s)
		}
		sort.Strings(names)
		what = append(what, "via "+strings.Join(names, ", "))
	}
	if regrows > 0 {
		what = append(what, cyan.Render("↺ regrows"))
	}
	return row + faint.Render(strings.Join(what, " · "))
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// check is the selection mark for a set of items: ● all, ◐ some, ○ none,
// blank when none can be selected.
func (m *Model) check(its []*item.Item) string {
	var can, sel int
	for _, it := range its {
		if it.Selectable() && m.status[it.ID] != done {
			can++
			if m.selected[it.ID] {
				sel++
			}
		}
	}
	switch {
	case can == 0:
		return " "
	case sel == can:
		return cyan.Render("●")
	case sel > 0:
		return cyan.Render("◐")
	}
	return faint.Render("○")
}

func (m *Model) warningLine() string {
	if len(m.warnings) == 0 && m.dupes == 0 {
		return ""
	}
	n := len(m.warnings)
	if m.dupes > 0 {
		n++
	}
	return "  " + yellow.Render(fmt.Sprintf("⚠ %d note%s", n, plural(n))) + faint.Render(" (? shows them)")
}

// --- list --------------------------------------------------------------------

func (m *Model) listItems() []*item.Item { return m.catItems(m.listCat) }

func (m *Model) listItem() *item.Item {
	its := m.listItems()
	if len(its) == 0 {
		return nil
	}
	m.listCursor = max(0, min(m.listCursor, len(its)-1))
	return its[m.listCursor]
}

func (m *Model) listLines() []string {
	its := m.listItems()
	c := m.listCat
	lines := []string{"", fmt.Sprintf("  %s %s  %s  %s", m.swatch(c), heading.Render(c.String()),
		bold.Render(format.Bytes(m.catBytes(c))), faint.Render(fmt.Sprintf("%d item%s", len(its), plural(len(its)))))}
	if len(its) == 0 {
		return append(lines, "", "  "+faint.Render("Nothing left here."))
	}
	bw := 12
	if m.width < 80 {
		bw = 0
	}
	head := "    "
	if bw > 0 {
		head += strings.Repeat(" ", bw+2)
	}
	lines = append(lines, faint.Render(head+fmt.Sprintf("%10s  %-11s ITEM", "FREES", "SAFETY")))
	room := max(1, m.height-len(lines)-11-detailRows)
	if m.listCursor < m.listOffset {
		m.listOffset = m.listCursor
	}
	if m.listCursor >= m.listOffset+room {
		m.listOffset = m.listCursor - room + 1
	}
	m.listOffset = max(0, min(m.listOffset, max(0, len(its)-room)))
	var largest int64 = 1
	for _, it := range its {
		largest = max(largest, it.Size())
	}
	for i := m.listOffset; i < min(len(its), m.listOffset+room); i++ {
		lines = append(lines, m.itemRow(its[i], i == m.listCursor, largest, bw))
	}
	if len(its) > room {
		lines = append(lines, faint.Render(fmt.Sprintf("    %d–%d of %d", m.listOffset+1, min(len(its), m.listOffset+room), len(its))))
	}
	return lines
}

func sizeText(it *item.Item) string {
	s := format.Bytes(it.Size())
	if it.Size() < 0 {
		s = "?"
	}
	if it.UpTo {
		s = "≤ " + s
	}
	return s
}

func (m *Model) itemRow(it *item.Item, isCur bool, largest int64, bw int) string {
	cur := " "
	if isCur {
		cur = cursorC
	}
	mark := m.check([]*item.Item{it})
	switch m.status[it.ID] {
	case done:
		mark = green.Render("✓")
	case queued, working:
		mark = faint.Render("◌")
	case failed:
		mark = red.Render("✗")
	}
	var bar string
	if bw > 0 {
		b := fine(float64(max(it.Size(), 0)), float64(largest), bw)
		if m.status[it.ID] == done {
			b = strings.Repeat("·", lipgloss.Width(b))
		}
		st := lipgloss.NewStyle().Foreground(lipgloss.Color(it.Category.Color()))
		if m.pal.mono {
			st = lipgloss.NewStyle()
		}
		bar = st.Render(b) + strings.Repeat(" ", bw-lipgloss.Width(b)) + "  "
	}
	name := it.Name
	if it.Path != "" && it.Category == item.Node {
		name = m.tilde(it.Path)
	}
	line := fmt.Sprintf("%10s  %s %s  %s", sizeText(it), badge(it.Level), bold.Render(name), faint.Render(m.actionTag(it)))
	switch m.status[it.ID] {
	case done:
		line = struck.Render(ansi.Strip(line))
	case queued, working:
		line = faint.Render(ansi.Strip(line))
	case failed:
		line = red.Render(ansi.Strip(line))
	}
	return fmt.Sprintf("%s %s %s%s", cur, mark, bar, line)
}

// actionTag says briefly how the item would go.
func (m *Model) actionTag(it *item.Item) string {
	switch it.Action.Kind {
	case item.Command:
		return "via " + shortArgv(it.Action.Argv)
	case item.Trash:
		return "→ " + trash.Name
	case item.Delegate:
		return "→ " + it.Source
	case item.Handoff:
		return "o opens " + it.Source
	case item.Manual:
		if len(it.Action.Argv) > 0 {
			return "run yourself: " + shortArgv(it.Action.Argv)
		}
		return "remove it in the app"
	}
	return "shown only"
}

func shortArgv(argv []string) string { return item.Action{Argv: argv}.Display() }

// detailPane explains the item under the cursor: what it is, exactly what
// would run, how it comes back and why it has its level.
func (m *Model) detailPane(it *item.Item) []string {
	out := []string{faint.Render(strings.Repeat("─", max(0, m.width)))}
	if it == nil {
		return fit(out, detailRows+1)
	}
	head := bold.Render(it.Name)
	if it.Path != "" {
		head += "  " + faint.Render(m.tilde(it.Path))
	}
	if it.Bytes >= 0 {
		head += "  " + faint.Render(format.Bytes(it.Bytes)+" on disk")
		if it.Frees >= 0 && it.Frees < it.Bytes*9/10 {
			head += faint.Render(", frees ") + cyan.Render(format.Bytes(it.Frees)) + faint.Render(" now (the rest is shared with clones or snapshots)")
		}
	}
	out = append(out, "  "+head)
	if it.Explain != "" {
		out = append(out, "  "+it.Explain)
	}
	how := "Action: " + magenta.Render(m.tildeText(report.What(it)))
	switch {
	case it.Action.Kind == item.Trash && m.opts.Exec.Mode == act.Permanent:
		how = "Action: " + red.Render("delete permanently")
	case it.Action.Kind == item.Command:
		how += faint.Render("  (the tool's own cleanup; it deletes)")
	case !it.Selectable() && it.Action.Kind != item.Handoff:
		how = levelStyle(it.Level).Render("decruft leaves this alone") + faint.Render(": "+m.tildeText(report.What(it)))
	}
	if it.Regenerate != "" {
		how += faint.Render("  ·  comes back with: ") + it.Regenerate
	}
	out = append(out, "  "+how)
	var why []string
	for _, r := range it.Reasons {
		why = append(why, levelStyle(r.Level).Render(r.Detail))
	}
	if it.Regrows {
		t := "↺ a working-set cache: it comes back as you build"
		if !it.Rebuilt.IsZero() {
			t += " (this one was recreated " + format.Ago(it.Rebuilt, nowFunc()) + ")"
		}
		why = append(why, cyan.Render(t))
	}
	if it.Note != "" {
		why = append(why, faint.Render(it.Note))
	}
	if e := m.errs[it.ID]; e != "" {
		why = append([]string{red.Render("✗ " + e)}, why...)
	}
	if len(why) > 0 {
		out = append(out, "  "+strings.Join(why, faint.Render(" · ")))
	}
	return fit(out, detailRows+1)
}

func (m *Model) tildeText(s string) string { return format.TildeAll(s, m.opts.Home) }

// --- footer ------------------------------------------------------------------

func (m *Model) summary() string {
	var n int
	var bytes int64
	for id := range m.selected {
		if it := m.find(id); it != nil {
			n++
			bytes += max(it.Size(), 0)
		}
	}
	all, allowed, _ := report.Total(m.items)
	parts := []string{fmt.Sprintf("%d selected · frees %s", n, bold.Render(format.Bytes(bytes))),
		faint.Render(fmt.Sprintf("%s found, %s removable here", format.Bytes(all), format.Bytes(allowed)))}
	if m.stopping {
		parts = append(parts, yellow.Render("stopping after this step"))
	}
	if m.ranOnce && !m.running && m.rcpt.Before >= 0 && m.rcpt.After >= 0 && m.opts.Exec.Mode != act.DryRun {
		parts = append(parts, green.Render("free space "+report.Signed(m.rcpt.After-m.rcpt.Before))+faint.Render(" measured"))
	}
	return "  " + strings.Join(parts, faint.Render("  ·  "))
}

func (m *Model) keyHints() string {
	if m.quitWarn {
		return yellow.Render("  Cleaning is running. ctrl+c stops after this step; press it twice to quit now.")
	}
	type k struct {
		text string
		drop int
	}
	var keys []k
	switch m.screen {
	case listing:
		keys = []k{{"↑↓ move", 4}, {"space select", 0}, {"s safe", 2}, {"a all", 3}, {"enter clean", 0}, {"t treemap", 1}, {"o open tool", 5}, {"← back", 1}, {"? help", 0}, {"q quit", 0}}
	case treemap:
		keys = []k{{"←↑↓→ move", 3}, {"enter zoom in", 0}, {"esc zoom out", 1}, {"space select", 0}, {"c clean", 0}, {"t list", 2}, {"? help", 0}, {"q quit", 0}}
	default:
		keys = []k{{"↑↓ move", 4}, {"enter open", 0}, {"space select", 1}, {"s select safe", 0}, {"c clean", 0}, {"t treemap", 1}, {"R rescan", 5}, {"? help", 0}, {"q quit", 0}}
	}
	for drop := 6; ; drop-- {
		var keep []string
		for _, x := range keys {
			if x.drop < drop {
				keep = append(keep, x.text)
			}
		}
		if s := "  " + strings.Join(keep, "  "); lipgloss.Width(s) <= m.width || drop == 1 {
			return faint.Render(s)
		}
	}
}

// --- confirm -----------------------------------------------------------------

func (m *Model) confirmHelp() string {
	if run, _ := m.plan(); len(run) == 0 {
		return faint.Render("  Nothing selected can be cleaned. Any key closes")
	}
	return yellow.Render("  Proceed?") + "  " + bold.Render("y") + faint.Render(" yes   any other key cancels")
}

// confirmLines lists exactly what will happen, split by how it frees space.
func (m *Model) confirmLines() []string {
	run, refuse := m.plan()
	var tr, cmd, del []item.Item
	var tb, cb, db int64
	for _, it := range run {
		switch it.Action.Kind {
		case item.Trash:
			tr, tb = append(tr, it), tb+max(it.Size(), 0)
		case item.Command:
			cmd, cb = append(cmd, it), cb+max(it.Size(), 0)
		case item.Delegate:
			del, db = append(del, it), db+max(it.Size(), 0)
		}
	}
	w := min(m.width, 100) - 2
	head := func(st lipgloss.Style, text string, n int64) string {
		total := format.Bytes(n)
		return st.Render(text + strings.Repeat(" ", max(2, w-lipgloss.Width(text)-len(total))) + total)
	}
	lines := []string{""}
	if m.opts.Exec.Mode == act.DryRun {
		lines = append(lines, yellow.Render("  Dry run: nothing will change."), "")
	}
	room := max(2, (m.height-14)/3)
	list := func(its []item.Item, show func(item.Item) string) {
		for i, it := range its {
			if i >= room && len(its) > room+1 {
				lines = append(lines, faint.Render(fmt.Sprintf("      … and %d more", len(its)-i)))
				return
			}
			lines = append(lines, fmt.Sprintf("      %9s  %s", sizeText(&it), show(it)))
		}
	}
	if len(tr) > 0 {
		h, st := "  Move to the "+trash.Name+" · recoverable, frees space when you empty it", green
		if m.opts.Exec.Mode == act.Permanent {
			h, st = "  Delete permanently · NOT recoverable", red
		}
		lines = append(lines, head(st, h, tb))
		list(tr, func(it item.Item) string { return m.tilde(it.Path) })
	}
	if len(cmd) > 0 {
		lines = append(lines, "", head(magenta, "  Official commands · regenerable, they delete and free space now", cb))
		list(cmd, func(it item.Item) string {
			return shortArgv(it.Action.Argv) + faint.Render("  "+it.Name)
		})
	}
	if len(del) > 0 {
		lines = append(lines, "", head(cyan, "  Delegated · each tool re-checks with its own rules", db))
		list(del, func(it item.Item) string { return it.Source + " · " + it.Name })
	}
	if len(refuse) > 0 {
		lines = append(lines, "", red.Render(fmt.Sprintf("  Not included (%d):", len(refuse))))
		list(refuse, func(it item.Item) string { return it.Name + faint.Render("  "+report.What(&it)) })
	}
	return lines
}

// --- receipt -----------------------------------------------------------------

func (m *Model) receiptLines() []string {
	r := m.rcpt
	lines := []string{"", "  " + heading.Render(fmt.Sprintf("Done in %s", r.Took.Round(1e9)))}
	if r.Before >= 0 && r.After >= 0 && m.disk.Total > 0 {
		w := max(10, min(m.width-30, 80))
		bar := func(free float64) string {
			used := max(0, float64(m.disk.Total)-free)
			return m.pal.storageBar([]segment{{bytes: used, color: m.pal.other, mono: "▒"}, {bytes: free, color: lipgloss.Color("#3FB97A"), mono: "░"}}, w)
		}
		lines = append(lines, "",
			fmt.Sprintf("  %-7s %s  %s free", "Before", bar(float64(r.Before)), format.Bytes(r.Before)),
			fmt.Sprintf("  %-7s %s  %s free", "After", bar(m.anim.get("free")), bold.Render(format.Bytes(int64(m.anim.get("free"))))))
	}
	lines = append(lines, "")
	for _, l := range r.Lines() {
		st := faint
		switch {
		case strings.HasPrefix(l, "✓"):
			st = green
		case strings.HasPrefix(l, "⚠"):
			st = yellow
		case strings.HasPrefix(l, "✗"):
			st = red
		case strings.HasPrefix(l, "  Free space"):
			st = bold
		}
		for _, wl := range strings.Split(ansi.Wrap(l, max(20, m.width-4), ""), "\n") {
			lines = append(lines, "  "+st.Render(m.tildeText(wl)))
		}
	}
	return lines
}

// --- legend ------------------------------------------------------------------

func (m *Model) legendLines() []string {
	var out []string
	para := func(indent, text string) {
		for _, l := range strings.Split(ansi.Wrap(text, max(20, m.width-len(indent)-1), ""), "\n") {
			out = append(out, indent+l)
		}
	}
	out = append(out, "", "  "+bold.Render("Safety"))
	para("    ", badge(item.Safe)+"  rebuildable by its tool; "+bold.Render("s")+" selects these")
	para("    ", badge(item.Review)+"  allowed when you pick it: costly to get back, or holds app data")
	para("    ", badge(item.Protected)+"  shown, never removed: no official command, needs sudo, or the owning app removes it")
	para("    ", badge(item.Live)+"  in use right now (Xcode, a booted simulator, an agent session)")
	out = append(out, "")
	para("  ", "Folders move to the "+trash.Name+" and free space when you empty it. Caches go through their own tool's official command ("+magenta.Render("go clean, brew cleanup, xcrun simctl, pod cache clean, uv cache prune, docker system prune")+"), which deletes and frees space now. fell, nodekill and flutterkill items are handed back to those tools, which check them again. Free space is measured, never assumed.")
	out = append(out, "")
	para("  ", bold.Render("FREES")+" is what removal frees right away: on APFS, blocks shared with clones and snapshots are left out. "+cyan.Render("↺")+" marks caches that come back as you work.")
	out = append(out, "")
	para("  ", bold.Render("Keys")+"  enter opens a category · space selects · s selects every safe item · a selects all in a list · c (or enter in a list) shows the plan · t treemap · o opens flutterkill · R rescans · q quits")
	notes := append([]string(nil), m.warnings...)
	if m.dupes > 0 {
		notes = append(notes, inventory.DedupeNote(m.dupes))
	}
	if len(notes) > 0 {
		out = append(out, "", "  "+yellow.Render("Notes"))
		for _, w := range notes {
			para("    ", yellow.Render("⚠ "+m.tildeText(w)))
		}
	}
	return out
}
