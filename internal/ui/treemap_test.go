package ui

import (
	"math"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestSquarifyFillsTheArea(t *testing.T) {
	vals := []float64{50, 25, 12, 8, 3, 2, 0}
	w, h := 120.0, 30.0
	rs := squarify(vals, w, h)
	var area float64
	for i, r := range rs {
		area += r.w * r.h
		if vals[i] == 0 && r.w*r.h != 0 {
			t.Fatal("zero value got area")
		}
		if r.x < -1e-9 || r.y < -1e-9 || r.x+r.w > w+1e-6 || r.y+r.h > h+1e-6 {
			t.Fatalf("tile %d outside: %+v", i, r)
		}
	}
	if math.Abs(area-w*h) > 1e-6 {
		t.Fatalf("area %f of %f", area, w*h)
	}
	// Cell-aspect correction: the biggest tile looks squarish on screen.
	if r := rs[0]; r.w/(r.h*cellAspect) > 3 || (r.h*cellAspect)/r.w > 3 {
		t.Fatalf("first tile too thin: %+v", r)
	}
}

func TestSnappedTilesDoNotOverlap(t *testing.T) {
	rs := squarify([]float64{40, 30, 20, 10, 5, 5, 1}, 97, 23)
	grid := make([][]int, 23)
	for y := range grid {
		grid[y] = make([]int, 97)
	}
	for i, r := range rs {
		c := snap(r)
		for y := c.y; y < c.y+c.h; y++ {
			for x := c.x; x < c.x+c.w; x++ {
				if grid[y][x] != 0 {
					t.Fatalf("tile %d overlaps tile %d at %d,%d", i, grid[y][x]-1, x, y)
				}
				grid[y][x] = i + 1
			}
		}
	}
}

func TestNearest(t *testing.T) {
	tiles := []cell{{0, 0, 10, 10}, {10, 0, 10, 5}, {10, 5, 10, 5}}
	if nearest(tiles, 0, 1, 0) == 0 || nearest(tiles, 1, 0, 1) != 2 || nearest(tiles, 2, -1, 0) != 0 || nearest(tiles, 0, -1, 0) != 0 {
		t.Fatal("nearest")
	}
}

func TestStorageBarWidth(t *testing.T) {
	p := newPalette(true, false)
	segs := []segment{{bytes: 1, color: p.other}, {bytes: 1e9, color: p.other}, {bytes: 3e9, color: p.free}}
	for _, w := range []int{1, 7, 80} {
		if got := ansi.StringWidth(p.storageBar(segs, w)); got != w {
			t.Fatalf("width %d: %d", w, got)
		}
	}
	mono := newPalette(true, true)
	if bar := mono.storageBar([]segment{{bytes: 1, mono: "█"}, {bytes: 1, mono: "░"}}, 10); strings.ContainsRune(bar, '\x1b') || bar != "█████░░░░░" {
		t.Fatalf("mono bar %q", bar)
	}
}
