package ui

import "math"

// rect is a tile in cell units.
type rect struct{ x, y, w, h float64 }

// cellAspect is how much taller a terminal cell is than it is wide. Tiles
// are laid out in a space stretched by it, so they look square on screen.
const cellAspect = 2.0

// squarify lays out values (sorted largest first) inside a w×h cell area
// with the squarified treemap algorithm (Bruls, Huizing, van Wijk 2000),
// corrected for the cell aspect. Zero values get empty rects.
func squarify(values []float64, w, h float64) []rect {
	out := make([]rect, len(values))
	var total float64
	for _, v := range values {
		total += max(v, 0)
	}
	if total <= 0 || w <= 0 || h <= 0 {
		return out
	}
	r := rect{0, 0, w, h * cellAspect}
	scale := r.w * r.h / total
	areas := make([]float64, len(values))
	for i, v := range values {
		areas[i] = max(v, 0) * scale
	}
	worst := func(sum, lo, hi, side float64) float64 {
		if sum <= 0 || lo <= 0 {
			return math.Inf(1)
		}
		s2, side2 := sum*sum, side*side
		return max(side2*hi/s2, s2/(side2*lo))
	}
	for i := 0; i < len(areas); {
		if areas[i] <= 0 {
			i++
			continue
		}
		side := min(r.w, r.h)
		sum, lo, hi := areas[i], areas[i], areas[i]
		j := i + 1
		for ; j < len(areas) && areas[j] > 0; j++ {
			ns, nlo, nhi := sum+areas[j], min(lo, areas[j]), max(hi, areas[j])
			if worst(ns, nlo, nhi, side) > worst(sum, lo, hi, side) {
				break
			}
			sum, lo, hi = ns, nlo, nhi
		}
		if r.w >= r.h { // a column on the left
			cw := sum / r.h
			y := r.y
			for k := i; k < j; k++ {
				hk := areas[k] / cw
				out[k] = rect{r.x, y, cw, hk}
				y += hk
			}
			r.x, r.w = r.x+cw, r.w-cw
		} else { // a row on top
			rh := sum / r.w
			x := r.x
			for k := i; k < j; k++ {
				wk := areas[k] / rh
				out[k] = rect{x, r.y, wk, rh}
				x += wk
			}
			r.y, r.h = r.y+rh, r.h-rh
		}
		i = j
	}
	for k := range out {
		out[k].y /= cellAspect
		out[k].h /= cellAspect
	}
	return out
}

// cell is a tile snapped to whole cells. Adjacent tiles share edges, so
// the grid has no gaps or overlaps.
type cell struct{ x, y, w, h int }

func snap(r rect) cell {
	x0, y0 := int(math.Round(r.x)), int(math.Round(r.y))
	x1, y1 := int(math.Round(r.x+r.w)), int(math.Round(r.y+r.h))
	return cell{x0, y0, x1 - x0, y1 - y0}
}

func (c cell) empty() bool { return c.w <= 0 || c.h <= 0 }

// nearest finds the tile next to cur in a direction (dx, dy), preferring
// tiles that line up with it. It returns cur when there is none.
func nearest(tiles []cell, cur, dx, dy int) int {
	if cur < 0 || cur >= len(tiles) {
		return 0
	}
	c := tiles[cur]
	cx, cy := float64(c.x)+float64(c.w)/2, float64(c.y)+float64(c.h)/2
	best, bestD := cur, math.Inf(1)
	for i, t := range tiles {
		if i == cur || t.empty() {
			continue
		}
		tx, ty := float64(t.x)+float64(t.w)/2, float64(t.y)+float64(t.h)/2
		ddx, ddy := tx-cx, (ty-cy)*cellAspect
		along, across := ddx*float64(dx)+ddy*float64(dy), math.Abs(ddx*float64(dy))+math.Abs(ddy*float64(dx))
		if along <= 0 {
			continue
		}
		// Tiles overlapping cur's span in the other axis are straight ahead.
		overlap := (dx != 0 && t.y < c.y+c.h && c.y < t.y+t.h) || (dy != 0 && t.x < c.x+c.w && c.x < t.x+t.w)
		d := along + 2*across
		if overlap {
			d = along
		}
		if d < bestD {
			best, bestD = i, d
		}
	}
	return best
}
