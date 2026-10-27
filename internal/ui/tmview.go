package ui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/lucasb-eyer/go-colorful"

	"github.com/Matthew-Eucaristo/decruft/internal/format"
	"github.com/Matthew-Eucaristo/decruft/internal/item"
)

// maxTiles keeps tiles big enough to read; the rest share one tile.
const maxTiles = 48

type tile struct {
	key   string // category key or item ID
	cat   item.Category
	it    *item.Item // nil for a category tile or the "smaller" tile
	name  string
	sub   string
	value float64
	done  bool
	cell  cell
}

// tmArea is the treemap's size: the screen minus header, bar, title,
// detail pane, summary and key hints.
func (m *Model) tmArea() (int, int) {
	return max(10, m.width), max(3, m.height-4-1-(detailRows+1)-2)
}

// tiles lays out the current treemap level. Removed items keep their
// place, hatched, so the map does not jump while cleaning.
func (m *Model) tiles() []tile {
	var ts []tile
	if m.tmCat < 0 {
		for _, c := range m.presentCats() {
			var all int64
			its := m.catItems(c)
			for _, it := range its {
				all += max(it.Size(), 0)
			}
			left := m.catBytes(c)
			ts = append(ts, tile{key: c.Key(), cat: c, name: c.String(), value: float64(all), done: left == 0 && all > 0,
				sub: fmt.Sprintf("%s · %d item%s", format.Bytes(left), len(its), plural(len(its)))})
		}
	} else {
		for _, it := range m.catItems(m.tmCat) {
			if it.Size() <= 0 {
				continue
			}
			ts = append(ts, tile{key: it.ID, cat: it.Category, it: it, name: it.Name, value: float64(it.Size()),
				done: m.status[it.ID] == done, sub: sizeText(it)})
		}
	}
	sortTiles(ts)
	if len(ts) > maxTiles {
		var rest float64
		for _, t := range ts[maxTiles-1:] {
			rest += t.value
		}
		n := len(ts) - maxTiles + 1
		ts = append(ts[:maxTiles-1], tile{key: "rest", cat: ts[0].cat, name: fmt.Sprintf("%d smaller", n), value: rest, sub: format.Bytes(int64(rest))})
	}
	w, h := m.tmArea()
	vals := make([]float64, len(ts))
	for i, t := range ts {
		vals[i] = t.value
	}
	for i, r := range squarify(vals, float64(w), float64(h)) {
		ts[i].cell = snap(r)
	}
	return ts
}

func sortTiles(ts []tile) {
	for i := 1; i < len(ts); i++ { // insertion sort keeps equal sizes in category order
		for j := i; j > 0 && ts[j].value > ts[j-1].value; j-- {
			ts[j], ts[j-1] = ts[j-1], ts[j]
		}
	}
}

func (m *Model) tmIndex(ts []tile) int {
	for i, t := range ts {
		if t.key == m.tmKey {
			return i
		}
	}
	if len(ts) > 0 {
		m.tmKey = ts[0].key
	}
	return 0
}

// tmItem is the item under the treemap cursor, for the detail pane.
func (m *Model) tmItem() *item.Item {
	ts := m.tiles()
	if len(ts) == 0 {
		return nil
	}
	return ts[m.tmIndex(ts)].it
}

type gcell struct {
	ch     string
	fg, bg color.Color
	bold   bool
}

func (m *Model) treemapLines(height int) []string {
	ts := m.tiles()
	w, h := m.tmArea()
	h = min(h, height-1)
	if len(ts) == 0 {
		return []string{"", "  " + faint.Render("Nothing to map yet.")}
	}
	cur := m.tmIndex(ts)
	grid := make([][]gcell, h)
	for y := range grid {
		grid[y] = make([]gcell, w)
		for x := range grid[y] {
			grid[y][x] = gcell{ch: " "}
		}
	}
	sep := m.pal.free
	for i, t := range ts {
		c := t.cell
		if c.empty() {
			continue
		}
		base := shade(hex(t.cat), float64(i%3-1)*0.10)
		if t.done {
			base = base.BlendRgb(colorful.Color{R: .5, G: .5, B: .5}, 0.75)
		}
		if i == cur {
			base = shade(base, 0.30)
		}
		bg := lipgloss.Color(base.Hex())
		txt := m.pal.textOn(base)
		for y := c.y; y < min(h, c.y+c.h); y++ {
			for x := c.x; x < min(w, c.x+c.w); x++ {
				g := gcell{ch: " ", bg: bg, fg: txt}
				if m.pal.mono {
					g = gcell{ch: " "}
					if i == cur {
						g.ch = "░"
					}
				}
				if t.done {
					g.ch, g.fg = "╱", lipgloss.Color(shade(base, -0.3).Hex())
				}
				switch {
				case m.pal.mono && y == c.y && x == c.x:
					g.ch = "┌"
				case m.pal.mono && y == c.y:
					g.ch = "─"
				case m.pal.mono && x == c.x:
					g.ch = "│"
				case x == c.x && x > 0:
					g.ch, g.fg = "▏", sep
				}
				grid[y][x] = g
			}
		}
		// Label: a selection mark and the name, then the size and, when it
		// is not plain safe, the safety level.
		mark := ""
		switch {
		case t.done:
			mark = "✓ "
		case t.it != nil && m.selected[t.it.ID]:
			mark = "● "
		case t.it == nil && t.key != "rest":
			if c := stripMark(m.check(m.catItems(t.cat))); c == "●" || c == "◐" {
				mark = c + " "
			}
		}
		sub := t.sub
		if t.it != nil && t.it.Level != item.Safe {
			sub += "  " + levelIcon[t.it.Level] + " " + string(t.it.Level)
		}
		labels := []string{mark + t.name, sub}
		if c.h == 1 {
			labels = []string{mark + t.name + "  " + sub}
		}
		for li, l := range labels {
			y := c.y + li
			if m.pal.mono {
				y++
			}
			if y >= min(h, c.y+c.h) || c.w < 4 {
				break
			}
			runes := []rune(l)
			for k := 0; k < len(runes) && c.x+1+k < c.x+c.w-1 && c.x+1+k < w; k++ {
				g := grid[y][c.x+1+k]
				g.ch, g.bold = string(runes[k]), li == 0
				if !m.pal.mono {
					g.fg = txt
				}
				grid[y][c.x+1+k] = g
			}
		}
	}
	lines := make([]string, h)
	for y, row := range grid {
		var b strings.Builder
		for x := 0; x < len(row); {
			g := row[x]
			run := []string{g.ch}
			x++
			for x < len(row) && row[x].fg == g.fg && row[x].bg == g.bg && row[x].bold == g.bold {
				run = append(run, row[x].ch)
				x++
			}
			st := lipgloss.NewStyle().Bold(g.bold)
			if g.fg != nil {
				st = st.Foreground(g.fg)
			}
			if g.bg != nil {
				st = st.Background(g.bg)
			}
			b.WriteString(st.Render(strings.Join(run, "")))
		}
		lines[y] = b.String()
	}
	title := "  " + heading.Render("All categories")
	if m.tmCat >= 0 {
		title = "  " + heading.Render(m.tmCat.String()) + faint.Render("  ·  esc zooms out")
	} else {
		title += faint.Render("  ·  enter zooms into a category")
	}
	return append([]string{title}, lines...)
}

// stripMark removes styling from a check mark.
func stripMark(s string) string {
	for _, r := range []string{"●", "◐", "○"} {
		if strings.Contains(s, r) {
			return r
		}
	}
	return ""
}
