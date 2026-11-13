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

type canvasState struct {
	clipMask  []uint8
	blendMode BlendMode
	linearBlend bool
}

// Canvas is a 2D rendering surface backed by a premultiplied RGBA pixel
// buffer. It owns a reusable Rasterizer and coverage buffer so that
// multiple draw calls can be issued without per-call allocations.
//
// Pixel data is stored in row-major order as a flat []uint8 with four bytes
// per pixel (R, G, B, A), all premultiplied.
type Canvas struct {
	width       int
	height      int
	pixels      []uint8 // RGBA, len = width*height*4
	rasterizer  *Rasterizer
	coverage    []uint8 // alpha mask, len = width*height
	clipMask    []uint8 // active clip alpha mask (nil if no clip active)
	clipStack   []canvasState
	blendMode   BlendMode
	linearBlend bool
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
		blendMode:  BlendModeSrcOver,
	}
}

// SetBlendMode changes the active compositing blend mode.
func (c *Canvas) SetBlendMode(mode BlendMode) {
	c.blendMode = mode
}

// SetLinearBlending enables or disables gamma-correct linear color space blending.
func (c *Canvas) SetLinearBlending(enabled bool) {
	c.linearBlend = enabled
}

// Save pushes the current canvas clip and blend state onto an internal stack.
func (c *Canvas) Save() {
	var savedMask []uint8
	if c.clipMask != nil {
		savedMask = make([]uint8, len(c.clipMask))
		copy(savedMask, c.clipMask)
	}
	c.clipStack = append(c.clipStack, canvasState{
		clipMask:    savedMask,
		blendMode:   c.blendMode,
		linearBlend: c.linearBlend,
	})
}

// Restore pops the most recently saved state from the stack.
func (c *Canvas) Restore() {
	if len(c.clipStack) == 0 {
		return
	}
	last := c.clipStack[len(c.clipStack)-1]
	c.clipStack = c.clipStack[:len(c.clipStack)-1]
	c.clipMask = last.clipMask
	c.blendMode = last.blendMode
	c.linearBlend = last.linearBlend
}

// PushClipRect intersects the current clipping region with an axis-aligned rectangle.
func (c *Canvas) PushClipRect(r Rect) {
	c.Save()
	x0 := int(r.X + 0.5)
	y0 := int(r.Y + 0.5)
	x1 := int(r.X + r.Width + 0.5)
	y1 := int(r.Y + r.Height + 0.5)

	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > c.width {
		x1 = c.width
	}
	if y1 > c.height {
		y1 = c.height
	}

	w := c.width
	if c.clipMask == nil {
		c.clipMask = make([]uint8, c.width*c.height)
		for y := y0; y < y1; y++ {
			rowOff := y * w
			for x := x0; x < x1; x++ {
				c.clipMask[rowOff+x] = 255
			}
		}
	} else {
		for y := 0; y < c.height; y++ {
			rowOff := y * w
			for x := 0; x < w; x++ {
				if x < x0 || x >= x1 || y < y0 || y >= y1 {
					c.clipMask[rowOff+x] = 0
				}
			}
		}
	}
}

// PushClipPath intersects the current clipping region with an arbitrary geometric path.
func (c *Canvas) PushClipPath(p geom.Path) {
	c.Save()
	c.rasterizer.Reset()
	c.rasterizer.AddPath(p)

	tempMask := make([]uint8, c.width*c.height)
	c.rasterizer.Rasterize(tempMask)

	if c.clipMask == nil {
		c.clipMask = tempMask
	} else {
		for i := range c.clipMask {
			c.clipMask[i] = uint8((uint16(c.clipMask[i])*uint16(tempMask[i]) + 128) / 255)
		}
	}
}

// PopClip restores the previous clip state (synonym for Restore).
func (c *Canvas) PopClip() {
	c.Restore()
}

// Width returns the canvas width in pixels.
func (c *Canvas) Width() int { return c.width }

// Height returns the canvas height in pixels.
func (c *Canvas) Height() int { return c.height }

// Pixels returns the raw underlying premultiplied RGBA pixel slice.
func (c *Canvas) Pixels() []uint8 { return c.pixels }

// ---------------------------------------------------------------------------
// Clear
// ---------------------------------------------------------------------------

// Clear fills every pixel of the canvas with the given solid color,
// replacing any existing content.
func (c *Canvas) Clear(col plume.Color) {
	w := c.width * 4
	for x := 0; x < w; x += 4 {
		c.pixels[x] = col.R
		c.pixels[x+1] = col.G
		c.pixels[x+2] = col.B
		c.pixels[x+3] = col.A
	}
	for y := 1; y < c.height; y++ {
		copy(c.pixels[y*w:(y+1)*w], c.pixels[:w])
	}
}

// ---------------------------------------------------------------------------
// DrawPath
// ---------------------------------------------------------------------------

// DrawPath fills a geometric path with the given paint. The path may contain
// any verbs (including curves); it is flattened internally by the rasterizer.
//
// Pipeline:
//  1. Reset rasterizer.
//  2. Add the path to the rasterizer (which flattens curves).
//  3. Produce the alpha coverage mask for dirty scanlines.
//  4. For every dirty scanline, blend the paint color onto the
//     canvas pixel buffer using vectorized source-over compositing.
func (c *Canvas) DrawPath(p geom.Path, paint plume.Paint) {
	w := c.width

	// Reset reusable rasterizer.
	c.rasterizer.Reset()

	// Rasterize and get dirty scanline range.
	c.rasterizer.AddPath(p)
	yStart, yEnd := c.rasterizer.Rasterize(c.coverage)
	if yStart > yEnd {
		return
	}

	// Composite only the dirty scanlines
	col := paint.Color
	for y := yStart; y <= yEnd; y++ {
		rowOff := y * w
		covRow := c.coverage[rowOff : rowOff+w]

		// Apply active clip mask if present
		if c.clipMask != nil {
			clipRow := c.clipMask[rowOff : rowOff+w]
			for x := 0; x < w; x++ {
				if covRow[x] > 0 {
					covRow[x] = uint8((uint16(covRow[x])*uint16(clipRow[x]) + 128) / 255)
				}
			}
		}

		if c.blendMode == BlendModeSrcOver && !c.linearBlend {
			blendSolidSpan(c.pixels[rowOff*4:(rowOff+w)*4], covRow, col.R, col.G, col.B, col.A)
		} else {
			pixRow := c.pixels[rowOff*4 : (rowOff+w)*4]
			for x := 0; x < w; x++ {
				cov := covRow[x]
				if cov == 0 {
					continue
				}
				pxOff := x * 4
				dr, dg, db, da := pixRow[pxOff], pixRow[pxOff+1], pixRow[pxOff+2], pixRow[pxOff+3]
				sr := uint8((uint16(col.R)*uint16(cov) + 128) / 255)
				sg := uint8((uint16(col.G)*uint16(cov) + 128) / 255)
				sb := uint8((uint16(col.B)*uint16(cov) + 128) / 255)
				sa := uint8((uint16(col.A)*uint16(cov) + 128) / 255)
				if c.linearBlend {
					pixRow[pxOff], pixRow[pxOff+1], pixRow[pxOff+2], pixRow[pxOff+3] = BlendLinearSrcOver(dr, dg, db, da, sr, sg, sb, sa)
				} else {
					pixRow[pxOff], pixRow[pxOff+1], pixRow[pxOff+2], pixRow[pxOff+3] = Blend(dr, dg, db, da, sr, sg, sb, sa, c.blendMode)
				}
			}
		}

		// Clean up dirty row of coverage buffer for next draw
		for x := 0; x < w; x++ {
			c.coverage[rowOff+x] = 0
		}
	}
}

// ---------------------------------------------------------------------------
// DrawRect
// ---------------------------------------------------------------------------

// DrawRect draws a filled axis-aligned rectangle.
func (c *Canvas) DrawRect(r Rect, paint plume.Paint) {
	// If clipping, non-standard blend mode, or linear blending is active, use path pipeline
	if c.clipMask != nil || c.blendMode != BlendModeSrcOver || c.linearBlend {
		c.DrawPath(geom.Rect(r.X, r.Y, r.Width, r.Height), paint)
		return
	}

	// Fast path for axis-aligned rectangles: bypass polygon rasterizer
	x0 := int(r.X + 0.5)
	y0 := int(r.Y + 0.5)
	x1 := int(r.X + r.Width + 0.5)
	y1 := int(r.Y + r.Height + 0.5)

	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > c.width {
		x1 = c.width
	}
	if y1 > c.height {
		y1 = c.height
	}

	if x0 >= x1 || y0 >= y1 {
		return
	}

	col := paint.Color
	w := c.width * 4

	if col.A == 255 {
		row0 := c.pixels[y0*w+x0*4 : y0*w+x1*4]
		for i := 0; i < len(row0); i += 4 {
			row0[i] = col.R
			row0[i+1] = col.G
			row0[i+2] = col.B
			row0[i+3] = 255
		}
		for y := y0 + 1; y < y1; y++ {
			copy(c.pixels[y*w+x0*4:y*w+x1*4], row0)
		}
		return
	}

	// Semi-transparent: use span blender directly
	spanLen := x1 - x0
	if spanLen > len(c.coverage) {
		spanLen = len(c.coverage)
	}
	covSpan := c.coverage[:spanLen]
	for i := range covSpan {
		covSpan[i] = 255
	}

	for y := y0; y < y1; y++ {
		dstOff := y*w + x0*4
		blendSolidSpan(c.pixels[dstOff:dstOff+spanLen*4], covSpan, col.R, col.G, col.B, col.A)
	}
	for i := range covSpan {
		covSpan[i] = 0
	}
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
// DrawText
// ---------------------------------------------------------------------------

// DrawText renders text at the given baseline position as a simple placeholder
// rectangle when true font rasterization (via the text package) is not in use.
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
// DrawMask
// ---------------------------------------------------------------------------

// DrawMask composites a grayscale alpha coverage mask onto the canvas
// at integer pixel coordinates (dstX, dstY) using vectorized alpha blending.
func (c *Canvas) DrawMask(mask []uint8, dstX, dstY, maskW, maskH int, col plume.Color) {
	if maskW <= 0 || maskH <= 0 || len(mask) < maskW*maskH {
		return
	}

	for my := 0; my < maskH; my++ {
		cy := dstY + my
		if cy < 0 || cy >= c.height {
			continue
		}

		cxStart := dstX
		mxStart := 0
		if cxStart < 0 {
			mxStart = -cxStart
			cxStart = 0
		}
		cxEnd := dstX + maskW
		if cxEnd > c.width {
			cxEnd = c.width
		}
		spanLen := cxEnd - cxStart
		if spanLen <= 0 {
			continue
		}

		dstOff := (cy*c.width + cxStart) * 4
		maskOff := my*maskW + mxStart

		blendSolidSpan(c.pixels[dstOff:dstOff+spanLen*4], mask[maskOff:maskOff+spanLen], col.R, col.G, col.B, col.A)
	}
}

// DrawSubpixelMask composites a 3-channel RGB subpixel coverage mask onto the canvas
// for LCD subpixel anti-aliasing. maskRGB has 3 bytes per pixel (R, G, B coverage).
func (c *Canvas) DrawSubpixelMask(maskRGB []uint8, dstX, dstY, maskW, maskH int, col plume.Color) {
	if maskW <= 0 || maskH <= 0 || len(maskRGB) < maskW*maskH*3 {
		return
	}

	for my := 0; my < maskH; my++ {
		cy := dstY + my
		if cy < 0 || cy >= c.height {
			continue
		}

		cxStart := dstX
		mxStart := 0
		if cxStart < 0 {
			mxStart = -cxStart
			cxStart = 0
		}
		cxEnd := dstX + maskW
		if cxEnd > c.width {
			cxEnd = c.width
		}
		spanLen := cxEnd - cxStart
		if spanLen <= 0 {
			continue
		}

		for x := 0; x < spanLen; x++ {
			cPx := cxStart + x
			dstOff := (cy*c.width + cPx) * 4
			mOff := (my*maskW + mxStart + x) * 3

			covR := uint16(maskRGB[mOff])
			covG := uint16(maskRGB[mOff+1])
			covB := uint16(maskRGB[mOff+2])

			if covR == 0 && covG == 0 && covB == 0 {
				continue
			}

			sr := uint8((uint16(col.R)*covR + 128) / 255)
			sg := uint8((uint16(col.G)*covG + 128) / 255)
			sb := uint8((uint16(col.B)*covB + 128) / 255)
			sa := uint8((uint16(col.A)*((covR+covG+covB)/3) + 128) / 255)

			dr := c.pixels[dstOff]
			dg := c.pixels[dstOff+1]
			db := c.pixels[dstOff+2]
			da := c.pixels[dstOff+3]

			c.pixels[dstOff] = sr + uint8((uint16(dr)*(255-covR)+128)/255)
			c.pixels[dstOff+1] = sg + uint8((uint16(dg)*(255-covG)+128)/255)
			c.pixels[dstOff+2] = sb + uint8((uint16(db)*(255-covB)+128)/255)
			c.pixels[dstOff+3] = sa + uint8((uint16(da)*(255-uint16(sa))+128)/255)
		}
	}
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
