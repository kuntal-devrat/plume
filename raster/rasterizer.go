// Package raster implements a CPU software rasterizer with analytical
// anti-aliasing for the Plume rendering engine.
//
// The rasterizer converts flattened vector paths (composed exclusively of
// MoveTo, LineTo, and Close verbs) into per-pixel alpha coverage masks.
// It uses an analytical signed-area approach inspired by FreeType's smooth
// rasterizer and the algorithms described by Raph Levien:
//
//  1. Each line segment is decomposed scanline-by-scanline and column-by-column.
//  2. For every pixel cell an edge passes through, the signed trapezoidal area
//     and vertical cover contribution are accumulated.
//  3. A left-to-right sweep converts the accumulated (area, cover) values into
//     a final alpha (0–255) per pixel using the non-zero winding fill rule.
//
// All coordinates are in pixel space with the origin at the top-left corner.
package raster

import (
	"github.com/plume-engine/plume/geom"
)

// ---------------------------------------------------------------------------
// Internal cell type
// ---------------------------------------------------------------------------

// cell stores the accumulated signed area and cover contributions from edges
// that cross this pixel.
type cell struct {
	area  float32 // Accumulated signed trapezoidal area.
	cover float32 // Accumulated signed vertical cover.
}

// ---------------------------------------------------------------------------
// Rasterizer
// ---------------------------------------------------------------------------

// Rasterizer converts flattened vector paths into pixel coverage masks using
// analytical anti-aliasing with the non-zero winding fill rule.
type Rasterizer struct {
	width  int
	height int
	cells  []cell // flat array, indexed as y*width + x

	minY int // Track dirty scanline range for fast reset and sweep
	maxY int
}

// NewRasterizer creates a Rasterizer for a canvas of the given pixel
// dimensions. The cell buffer is allocated once and reused across Reset calls.
func NewRasterizer(width, height int) *Rasterizer {
	return &Rasterizer{
		width:  width,
		height: height,
		cells:  make([]cell, width*height),
		minY:   height,
		maxY:   -1,
	}
}

// Reset clears all accumulated coverage data so the Rasterizer can be reused
// for the next path without reallocating its internal buffers.
func (r *Rasterizer) Reset() {
	if r.minY <= r.maxY && r.minY >= 0 && r.maxY < r.height {
		start := r.minY * r.width
		end := (r.maxY + 1) * r.width
		for i := start; i < end; i++ {
			r.cells[i] = cell{}
		}
	} else if r.minY <= r.maxY {
		for i := range r.cells {
			r.cells[i] = cell{}
		}
	}
	r.minY = r.height
	r.maxY = -1
}

// AddPath flattens p (converting any curves to line segments) and adds the
// resulting edges to the rasterizer's coverage accumulator.
func (r *Rasterizer) AddPath(p geom.Path) {
	flat := geom.Flatten(p, 0.25)

	var curX, curY float32
	var startX, startY float32
	idx := 0

	for _, v := range flat.Verbs {
		switch v {
		case geom.VerbMove:
			x, y := flat.Points[idx], flat.Points[idx+1]
			idx += 2
			curX, curY = x, y
			startX, startY = x, y

		case geom.VerbLine:
			x, y := flat.Points[idx], flat.Points[idx+1]
			idx += 2
			r.addEdge(curX, curY, x, y)
			curX, curY = x, y

		case geom.VerbClose:
			if curX != startX || curY != startY {
				r.addEdge(curX, curY, startX, startY)
			}
			curX, curY = startX, startY
		}
	}
}

// ---------------------------------------------------------------------------
// Core edge processing
// ---------------------------------------------------------------------------

// addEdge processes a single line segment from (x0, y0) to (x1, y1),
// accumulating its signed area and cover contributions into the cell buffer.
func (r *Rasterizer) addEdge(x0, y0, x1, y1 float32) {
	if y0 == y1 {
		return
	}

	h := float32(r.height)
	if (y0 <= 0 && y1 <= 0) || (y0 >= h && y1 >= h) {
		return
	}

	dy := y1 - y0
	dx := x1 - x0
	dxdy := dx / dy

	// Clip y0 to [0, h]
	if y0 < 0 {
		x0 -= y0 * dxdy
		y0 = 0
	} else if y0 > h {
		x0 -= (y0 - h) * dxdy
		y0 = h
	}

	// Clip y1 to [0, h]
	if y1 < 0 {
		x1 -= y1 * dxdy
		y1 = 0
	} else if y1 > h {
		x1 -= (y1 - h) * dxdy
		y1 = h
	}

	if y0 == y1 {
		return
	}

	// Orient segment top-to-bottom and record direction.
	var dir float32 = 1.0
	yA, yB := y0, y1
	xA := x0
	if y0 > y1 {
		yA, yB = y1, y0
		xA = x1
		dir = -1.0
	}

	eyStart := int(yA)
	eyEnd := int(yB)
	if eyEnd >= r.height {
		eyEnd = r.height - 1
	}

	if eyStart < r.minY {
		r.minY = eyStart
	}
	if eyEnd > r.maxY {
		r.maxY = eyEnd
	}

	for ey := eyStart; ey <= eyEnd; ey++ {
		rowTop := float32(ey)
		rowBot := float32(ey + 1)

		yTop := yA
		if rowTop > yTop {
			yTop = rowTop
		}
		yBot := yB
		if rowBot < yBot {
			yBot = rowBot
		}

		if yTop >= yBot {
			continue
		}

		fDy := yBot - yTop
		xTop := xA + (yTop-yA)*dxdy
		xBot := xA + (yBot-yA)*dxdy

		xMin := xTop
		xMax := xBot
		if xMin > xMax {
			xMin, xMax = xBot, xTop
		}

		ixMin := int(xMin)
		ixMax := int(xMax)

		if ixMin == ixMax {
			// Segment is entirely within column ixMin
			X := ixMin
			dyK := fDy * dir
			uAvg := 0.5*(xTop+xBot) - float32(X)
			areaK := uAvg * dyK

			if X < 0 {
				r.cells[ey*r.width].cover += dyK
			} else if X < r.width {
				ci := ey*r.width + X
				r.cells[ci].cover += dyK
				r.cells[ci].area += areaK
			}
		} else {
			// Segment spans multiple columns
			deltaX := xMax - xMin
			for X := ixMin; X <= ixMax; X++ {
				colLeft := float32(X)
				colRight := float32(X + 1)

				xACol := xMin
				if colLeft > xACol {
					xACol = colLeft
				}
				xBCol := xMax
				if colRight < xBCol {
					xBCol = colRight
				}

				if xACol >= xBCol {
					continue
				}

				frac := (xBCol - xACol) / deltaX
				dyK := fDy * frac * dir
				uAvg := 0.5*(xACol+xBCol) - float32(X)
				areaK := uAvg * dyK

				if X < 0 {
					r.cells[ey*r.width].cover += dyK
				} else if X < r.width {
					ci := ey*r.width + X
					r.cells[ci].cover += dyK
					r.cells[ci].area += areaK
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Sweep — convert accumulated cells to alpha coverage
// ---------------------------------------------------------------------------

// Rasterize converts the accumulated edge data into an alpha coverage mask.
// dst must be a flat []uint8 buffer of length width×height (one byte per
// pixel, row-major order). Each byte receives an alpha value in [0, 255]
// representing the pixel's fractional coverage under the non-zero winding
// fill rule. Returns the dirty scanline range [yStart, yEnd] that was updated.
func (r *Rasterizer) Rasterize(dst []uint8) (int, int) {
	if r.minY > r.maxY || r.minY >= r.height || r.maxY < 0 {
		return 0, -1
	}

	w := r.width
	yStart := r.minY
	if yStart < 0 {
		yStart = 0
	}
	yEnd := r.maxY
	if yEnd >= r.height {
		yEnd = r.height - 1
	}

	for y := yStart; y <= yEnd; y++ {
		var cover float32
		rowOff := y * w
		for x := 0; x < w; x++ {
			c := &r.cells[rowOff+x]
			cover += c.cover
			alpha := cover - c.area
			if alpha < 0 {
				alpha = -alpha
			}
			if alpha > 1.0 {
				alpha = 1.0
			}
			dst[rowOff+x] = uint8(alpha*255.0 + 0.5)
		}
	}
	return yStart, yEnd
}
