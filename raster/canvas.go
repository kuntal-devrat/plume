package raster

import (
	"image"
	"image/color"

	plume "github.com/plume-engine/plume"
	"github.com/plume-engine/plume/geom"
	"github.com/plume-engine/plume/layout"
)

// canvas.go provides a high-level 2D rendering surface that ties together
// the rasterizer, blender, and paint systems into a simple drawing API.

// ---------------------------------------------------------------------------
// Rect — layout-compatible rectangle type
// ---------------------------------------------------------------------------

// Rect describes an axis-aligned rectangle in pixel coordinates.
// It is aliased to layout.Rect so that computed layout rectangles can be passed
// directly to canvas drawing methods without type conversion.
type Rect = layout.Rect

// ---------------------------------------------------------------------------
// Canvas
// ---------------------------------------------------------------------------

// Canvas is a 2D rendering surface backed by a premultiplied RGBA pixel
// buffer. It owns a reusable Rasterizer and coverage buffer so that
// multiple draw calls can be issued without per-call allocations.
//
// Pixel data is stored in row-major order as a flat []uint8 with four bytes
// per pixel (R, G, B, A), all premultiplied.
type Canvas struct {
	width      int
	height     int
	pixels     []uint8      // RGBA, len = width*height*4
	rasterizer *Rasterizer
	coverage   []uint8      // alpha mask, len = width*height
}

// NewCanvas creates a Canvas of the given dimensions. All pixels are
// initialized to transparent black (0, 0, 0, 0).
func NewCanvas(width, height int) *Canvas {
	return &Canvas{
		width:      width,
		height:     height,
		pixels:     make([]uint8, width*height*4),
		rasterizer: NewRasterizer(width, height),
		coverage:   make([]uint8, width*height),
	}
}

// Width returns the canvas width in pixels.
func (c *Canvas) Width() int { return c.width }

// Height returns the canvas height in pixels.
func (c *Canvas) Height() int { return c.height }

// ---------------------------------------------------------------------------
// Clear
// ---------------------------------------------------------------------------

// Clear fills every pixel of the canvas with the given solid color,
// replacing any existing content.
func (c *Canvas) Clear(col plume.Color) {
	r, g, b, a := col.R, col.G, col.B, col.A
	for i := 0; i < len(c.pixels); i += 4 {
		c.pixels[i] = r
		c.pixels[i+1] = g
		c.pixels[i+2] = b
		c.pixels[i+3] = a
	}
}

// ---------------------------------------------------------------------------
// DrawPath
// ---------------------------------------------------------------------------

// DrawPath fills a geometric path with the given paint. The path may contain
// any verbs (including curves); it is flattened internally by the rasterizer.
//
// Pipeline:
//  1. Reset rasterizer and coverage buffer.
//  2. Add the path to the rasterizer (which flattens curves).
//  3. Produce the alpha coverage mask.
//  4. For every pixel with non-zero coverage, blend the paint color onto the
//     canvas pixel buffer using source-over compositing.
func (c *Canvas) DrawPath(p geom.Path, paint plume.Paint) {
	w, h := c.width, c.height

	// Reset reusable buffers.
	c.rasterizer.Reset()
	for i := range c.coverage {
		c.coverage[i] = 0
	}

	// Rasterize.
	c.rasterizer.AddPath(p)
	c.rasterizer.Rasterize(c.coverage)

	// Resolve paint.
	ps := NewSolidPaint(paint.Color)

	// Composite.
	for y := 0; y < h; y++ {
		rowOff := y * w
		for x := 0; x < w; x++ {
			cov := c.coverage[rowOff+x]
			if cov == 0 {
				continue
			}
			col := ps.ColorAt(x, y)
			pixOff := (rowOff + x) * 4
			BlendPixel(c.pixels, pixOff, col.R, col.G, col.B, col.A, cov)
		}
	}
}

// ---------------------------------------------------------------------------
// DrawRect
// ---------------------------------------------------------------------------

// DrawRect draws a filled axis-aligned rectangle.
func (c *Canvas) DrawRect(r Rect, paint plume.Paint) {
	p := geom.Rect(r.X, r.Y, r.Width, r.Height)
	c.DrawPath(p, paint)
}

// ---------------------------------------------------------------------------
// DrawRoundedRect
// ---------------------------------------------------------------------------

// DrawRoundedRect draws a filled rounded rectangle with the given corner
// radius. The radius is internally clamped to half the smaller dimension.
func (c *Canvas) DrawRoundedRect(r Rect, radius float32, paint plume.Paint) {
	p := geom.RoundedRect(r.X, r.Y, r.Width, r.Height, radius)
	c.DrawPath(p, paint)
}

// ---------------------------------------------------------------------------
// DrawText (Phase 1 stub)
// ---------------------------------------------------------------------------

// DrawText renders text at the given baseline position.
//
// Phase 1 implementation: since no font rasterizer is available yet, this
// draws a placeholder colored rectangle whose width is estimated from the
// string length and font size (≈ 0.6 × fontSize per character). The
// rectangle's height equals the font size.
func (c *Canvas) DrawText(text string, x, y float32, style plume.TextStyle) {
	if len(text) == 0 {
		return
	}
	// Rough glyph-width estimate: 60% of em-square.
	estWidth := float32(len(text)) * style.FontSize * 0.6
	estHeight := style.FontSize

	// Position rectangle so that (x, y) is the top-left of the text box.
	p := geom.Rect(x, y, estWidth, estHeight)
	c.DrawPath(p, plume.Fill(style.Color))
}

// ---------------------------------------------------------------------------
// ToRGBA
// ---------------------------------------------------------------------------

// ToRGBA converts the canvas's internal pixel buffer into a standard library
// *image.RGBA suitable for encoding to PNG or other formats.
func (c *Canvas) ToRGBA() *image.RGBA {
	img := image.NewRGBA(image.Rectangle{
		Min: image.Point{X: 0, Y: 0},
		Max: image.Point{X: c.width, Y: c.height},
	})
	// Copy pixel data row by row. image.RGBA.Stride may differ from
	// width*4, so we copy per-row.
	for y := 0; y < c.height; y++ {
		srcOff := y * c.width * 4
		dstOff := y * img.Stride
		copy(img.Pix[dstOff:dstOff+c.width*4], c.pixels[srcOff:srcOff+c.width*4])
	}
	return img
}

// Ensure plume.Color satisfies color.Color at compile time.
var _ color.Color = plume.Color{}
