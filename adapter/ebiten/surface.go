// Package ebiten provides an integration adapter between the Plume rendering
// engine and the Ebitengine 2D game library.
//
// It enables game developers to build complex Flexbox & CSS Grid UI layouts
// (menus, inventory grids, HUDs, dialogue boxes) and stream rendered vector
// pixels directly to GPU textures with zero heap allocations.
package ebiten

import (
	"image"

	"github.com/kuntal-devrat/plume/raster"
)

// WritableImage is an interface satisfied by *ebiten.Image without forcing
// a hard dependency on the Ebitengine package.
type WritableImage interface {
	WritePixels(pixels []byte)
}

// Surface manages a Plume raster Canvas and coordinates zero-copy pixel
// synchronization with game engine textures.
type Surface struct {
	canvas *raster.Canvas
	width  int
	height int
}

// NewSurface creates an adapter surface with the given dimensions.
func NewSurface(width, height int) *Surface {
	return &Surface{
		canvas: raster.NewCanvas(width, height),
		width:  width,
		height: height,
	}
}

// Canvas returns the underlying Plume Canvas for drawing operations.
func (s *Surface) Canvas() *raster.Canvas {
	return s.canvas
}

// Pixels returns the raw underlying premultiplied RGBA byte slice,
// suitable for passing directly to ebiten.Image.WritePixels.
func (s *Surface) Pixels() []byte {
	return s.canvas.Pixels()
}

// FlushTo copies the rendered canvas pixel buffer into any destination
// satisfying [WritableImage] (e.g. *ebiten.Image).
func (s *Surface) FlushTo(dst WritableImage) {
	dst.WritePixels(s.canvas.Pixels())
}

// AsRGBA wraps the canvas pixel slice as a standard library *image.RGBA.
func (s *Surface) AsRGBA() *image.RGBA {
	return s.canvas.ToRGBA()
}

// Resize updates the surface dimensions if the window or viewport changes.
func (s *Surface) Resize(width, height int) {
	if s.width == width && s.height == height {
		return
	}
	s.width = width
	s.height = height
	s.canvas = raster.NewCanvas(width, height)
}
