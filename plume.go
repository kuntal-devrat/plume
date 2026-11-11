// Package plume provides 2D vector path rendering and layout primitives in pure Go.
//
// The engine is organized into sub-packages:
//   - plume/layout: CSS Flexbox and Grid layout computation.
//   - plume/geom:   Vector path construction, curve flattening, stroking, and transforms.
//   - plume/raster: Software rasterizer with analytical anti-aliasing.
//   - plume/text:   TrueType font parsing and text shaping.
//   - plume/gpu:    Vector path triangulation for GPU vertex buffers.
package plume

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"
)

// Color represents a premultiplied RGBA color with 8 bits per channel.
type Color struct {
	R, G, B, A uint8
}

// RGBA returns the color as 16-bit premultiplied RGBA values,
// implementing the color.Color interface.
func (c Color) RGBA() (r, g, b, a uint32) {
	r = uint32(c.R)
	r |= r << 8
	g = uint32(c.G)
	g |= g << 8
	b = uint32(c.B)
	b |= b << 8
	a = uint32(c.A)
	a |= a << 8
	return
}

// ColorFromRGBA creates a Color from non-premultiplied RGBA values.
// Alpha premultiplication is applied automatically.
func ColorFromRGBA(r, g, b, a uint8) Color {
	if a == 255 {
		return Color{R: r, G: g, B: b, A: 255}
	}
	if a == 0 {
		return Color{}
	}
	return Color{
		R: uint8(uint16(r) * uint16(a) / 255),
		G: uint8(uint16(g) * uint16(a) / 255),
		B: uint8(uint16(b) * uint16(a) / 255),
		A: a,
	}
}

// Hex parses a CSS-style hex color string and returns a fully opaque Color.
// Supported formats: "#RGB", "#RRGGBB", "#RRGGBBAA".
// The leading '#' is optional. Panics on invalid input.
func Hex(s string) Color {
	s = strings.TrimPrefix(s, "#")
	var r, g, b, a uint8
	a = 255
	switch len(s) {
	case 3:
		r = hexNibble(s[0]) * 17
		g = hexNibble(s[1]) * 17
		b = hexNibble(s[2]) * 17
	case 6:
		r = hexByte(s[0], s[1])
		g = hexByte(s[2], s[3])
		b = hexByte(s[4], s[5])
	case 8:
		r = hexByte(s[0], s[1])
		g = hexByte(s[2], s[3])
		b = hexByte(s[4], s[5])
		a = hexByte(s[6], s[7])
	default:
		panic(fmt.Sprintf("plume.Hex: invalid color string %q", s))
	}
	return ColorFromRGBA(r, g, b, a)
}

func hexNibble(c byte) uint8 {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	default:
		panic(fmt.Sprintf("plume.Hex: invalid hex character %q", c))
	}
}

func hexByte(hi, lo byte) uint8 {
	return hexNibble(hi)<<4 | hexNibble(lo)
}

// Predefined colors.
var (
	White       = Color{R: 255, G: 255, B: 255, A: 255}
	Black       = Color{R: 0, G: 0, B: 0, A: 255}
	Transparent = Color{}
)

// Paint describes how a shape is filled or stroked.
type Paint struct {
	Color Color
}

// Fill creates a solid-color fill paint.
func Fill(c Color) Paint {
	return Paint{Color: c}
}

// TextStyle describes how text should be rendered.
type TextStyle struct {
	FontSize float32
	Color    Color
}

// SavePNG encodes an image.Image to a PNG file at the given path.
func SavePNG(img image.Image, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("plume.SavePNG: %w", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("plume.SavePNG: %w", err)
	}
	return nil
}
