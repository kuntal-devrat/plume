package raster

import plume "github.com/plume-engine/plume"

// paint.go provides paint resolution — converting a high-level Paint
// description into concrete per-pixel colors.
//
// Currently supports solid colors, with extensible interfaces for
// gradient and pattern fills.

// PaintSource resolves a paint to a concrete RGBA color at a given pixel
// coordinate. For solid colors the position is ignored; gradient and
// pattern fills will use it to compute spatially-varying colors.
type PaintSource struct {
	color plume.Color
}

// NewSolidPaint creates a PaintSource that always returns the given solid
// color, regardless of pixel position.
func NewSolidPaint(c plume.Color) PaintSource {
	return PaintSource{color: c}
}

// ColorAt returns the resolved color at pixel position (x, y).
// For a solid paint this is always the paint's color.
func (p PaintSource) ColorAt(x, y int) plume.Color {
	return p.color
}
