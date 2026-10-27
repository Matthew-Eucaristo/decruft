package ui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/harmonica"
	"github.com/lucasb-eyer/go-colorful"

	"github.com/Matthew-Eucaristo/decruft/internal/item"
)

// Family look: plain ANSI colours for text, so the terminal theme decides
// the shades. Truecolor is used only where colour carries the data: the
// storage bar and the treemap, one hue per category.
var (
	accent  = lipgloss.Color("5")
	brand   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(accent).Padding(0, 1)
	bold    = lipgloss.NewStyle().Bold(true)
	faint   = lipgloss.NewStyle().Faint(true)
	cyan    = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	green   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	yellow  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	red     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	magenta = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	struck  = lipgloss.NewStyle().Faint(true).Strikethrough(true)
	heading = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5"))
	banner  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("1")).Padding(0, 1)
	cursorC = green.Render("▌")
)

func levelStyle(l item.Level) lipgloss.Style {
	switch l {
	case item.Safe:
		return green
	case item.Review:
		return yellow
	case item.Protected:
		return red
	}
	return magenta
}

var levelIcon = map[item.Level]string{item.Safe: "●", item.Review: "▲", item.Protected: "■", item.Live: "◉"}

func badge(l item.Level) string {
	return levelStyle(l).Render(fmt.Sprintf("%s %-9s", levelIcon[l], l))
}

// palette holds the colours that depend on the terminal's background.
type palette struct {
	mono      bool // NO_COLOR or a terminal without colour: glyphs carry meaning
	other     color.Color
	free      color.Color
	darkText  color.Color
	lightText color.Color
}

func newPalette(dark, mono bool) palette {
	ld := lipgloss.LightDark(dark)
	return palette{
		mono:      mono,
		other:     ld(lipgloss.Color("#B8BEC6"), lipgloss.Color("#4A515B")),
		free:      ld(lipgloss.Color("#E6E9ED"), lipgloss.Color("#22272E")),
		darkText:  lipgloss.Color("#14161A"),
		lightText: lipgloss.Color("#F4F5F7"),
	}
}

func hex(c item.Category) colorful.Color {
	col, err := colorful.Hex(c.Color())
	if err != nil {
		return colorful.Color{R: .5, G: .5, B: .5}
	}
	return col
}

// shade lightens (t > 0) or darkens (t < 0) a colour.
func shade(c colorful.Color, t float64) colorful.Color {
	if t >= 0 {
		return c.BlendRgb(colorful.Color{R: 1, G: 1, B: 1}, t).Clamped()
	}
	return c.BlendRgb(colorful.Color{}, -t).Clamped()
}

// textOn picks readable text for a background.
func (p palette) textOn(bg colorful.Color) color.Color {
	_, _, l := bg.Hsl()
	if l > 0.55 {
		return p.darkText
	}
	return p.lightText
}

var eighths = []string{"", "▏", "▎", "▍", "▌", "▋", "▊", "▉"}

// fine draws n/total of width cells in eighth blocks.
func fine(n, total float64, width int) string {
	if total <= 0 || n <= 0 || width <= 0 {
		return ""
	}
	units := int(n * float64(width) * 8 / total)
	units = max(1, min(units, width*8))
	return strings.Repeat("█", units/8) + eighths[units%8]
}

// segment is one coloured run of the storage bar.
type segment struct {
	bytes float64
	color color.Color
	mono  string // fill in mono mode
}

// storageBar draws segments across width cells, with sub-cell boundaries:
// a partial block takes the left segment's colour as foreground and the
// next one's as background, so even small categories stay visible.
func (p palette) storageBar(segs []segment, width int) string {
	var total float64
	for _, s := range segs {
		total += max(s.bytes, 0)
	}
	if total <= 0 || width <= 0 {
		return ""
	}
	units := width * 8
	ends := make([]int, len(segs)) // cumulative end, in eighths
	var cum float64
	for i, s := range segs {
		cum += max(s.bytes, 0)
		ends[i] = int(cum / total * float64(units))
		if s.bytes > 0 && i > 0 && ends[i] <= ends[i-1] {
			ends[i] = min(ends[i-1]+1, units) // anything present shows at least an eighth
		}
	}
	ends[len(ends)-1] = units
	at := func(u int) int {
		for i, e := range ends {
			if u < e {
				return i
			}
		}
		return len(segs) - 1
	}
	var b strings.Builder
	for x := 0; x < width; x++ {
		u := x * 8
		left := at(u)
		k := min(ends[left]-u, 8)
		if p.mono {
			b.WriteString(segs[left].mono)
			continue
		}
		st := lipgloss.NewStyle().Foreground(segs[left].color)
		if k >= 8 {
			b.WriteString(st.Render("█"))
			continue
		}
		right := at(u + k)
		b.WriteString(st.Background(segs[right].color).Render(eighths[k]))
	}
	return b.String()
}

// springs animate numbers (bytes) towards their targets.
type springs struct {
	s    harmonica.Spring
	vals map[string]*sval
}

type sval struct{ pos, vel, target float64 }

const fps = 30

func newSprings() springs {
	return springs{s: harmonica.NewSpring(harmonica.FPS(fps), 7.0, 0.85), vals: map[string]*sval{}}
}

// set moves a value's target; a new value starts there without animating.
func (s *springs) set(key string, target float64) {
	v, ok := s.vals[key]
	if !ok {
		s.vals[key] = &sval{pos: target, target: target}
		return
	}
	v.target = target
}

func (s *springs) get(key string) float64 {
	if v, ok := s.vals[key]; ok {
		return v.pos
	}
	return 0
}

// step advances every value one frame and reports whether any still moves.
func (s *springs) step() bool {
	moving := false
	for _, v := range s.vals {
		v.pos, v.vel = s.s.Update(v.pos, v.vel, v.target)
		d := v.target - v.pos
		if d < 0 {
			d = -d
		}
		if d < 1e6 && v.vel < 1e6 && v.vel > -1e6 { // within a megabyte: settled
			v.pos, v.vel = v.target, 0
			continue
		}
		moving = true
	}
	return moving
}

func (s *springs) settled() bool {
	for _, v := range s.vals {
		if v.pos != v.target {
			return false
		}
	}
	return true
}
