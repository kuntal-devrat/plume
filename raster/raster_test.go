package raster

import (
	"testing"

	plume "github.com/plume-engine/plume"
	"github.com/plume-engine/plume/geom"
)

// ---------------------------------------------------------------------------
// Rasterizer basics
// ---------------------------------------------------------------------------

// TestRasterizeSquare verifies that a filled square path produces non-zero
// coverage in interior pixels and zero coverage outside.
func TestRasterizeSquare(t *testing.T) {
	const size = 32
	r := NewRasterizer(size, size)

	// Create a 10×10 square at (5, 5).
	p := geom.Rect(5, 5, 10, 10)
	r.AddPath(p)

	alpha := make([]uint8, size*size)
	r.Rasterize(alpha)

	// Interior pixel (10, 10) should have high coverage.
	if a := alpha[10*size+10]; a < 200 {
		t.Errorf("interior pixel (10,10) alpha = %d, want >= 200", a)
	}

	// Exterior pixel (0, 0) should be zero.
	if a := alpha[0]; a != 0 {
		t.Errorf("exterior pixel (0,0) alpha = %d, want 0", a)
	}

	// Exterior pixel (20, 20) should be zero.
	if a := alpha[20*size+20]; a != 0 {
		t.Errorf("exterior pixel (20,20) alpha = %d, want 0", a)
	}
}

// TestRasterizeDiagonalAA verifies that a diagonal edge produces partial
// (anti-aliased) coverage at the boundary.
func TestRasterizeDiagonalAA(t *testing.T) {
	const size = 32
	r := NewRasterizer(size, size)

	// Thin diagonal triangle: creates diagonal edges.
	var b geom.PathBuilder
	b.MoveTo(2, 2)
	b.LineTo(20, 2)
	b.LineTo(2, 20)
	b.Close()
	p := b.Finish()

	r.AddPath(p)
	alpha := make([]uint8, size*size)
	r.Rasterize(alpha)

	// A pixel near the hypotenuse should have partial coverage (AA).
	// The hypotenuse goes from (20,2) to (2,20). Midpoint is (11,11).
	// Pixel just inside at (9, 9) should be partially covered.
	a := alpha[9*size+9]
	if a == 0 {
		t.Error("pixel near diagonal edge (9,9) has zero coverage; expected partial AA")
	}
	if a == 255 {
		// It's on the edge — could be partial. Not strictly wrong but
		// let's check that some nearby pixel is partial.
		t.Log("pixel (9,9) is fully covered; checking neighbours for AA")
	}

	// Check that pixels along the hypotenuse have partial coverage (128 for a 45-degree line).
	foundPartial := false
	for y := 2; y < 20; y++ {
		for x := 2; x < 20; x++ {
			a := alpha[y*size+x]
			if a > 0 && a < 255 {
				foundPartial = true
				break
			}
		}
		if foundPartial {
			break
		}
	}
	if !foundPartial {
		t.Error("no partial-coverage pixel found along diagonal; anti-aliasing may not be working")
	}
}

// ---------------------------------------------------------------------------
// Blender
// ---------------------------------------------------------------------------

func TestBlendSrcOverOpaque(t *testing.T) {
	// Fully opaque source replaces destination.
	r, g, b, a := BlendSrcOver(100, 150, 200, 255, 255, 0, 0, 255)
	if r != 255 || g != 0 || b != 0 || a != 255 {
		t.Errorf("opaque SrcOver = (%d,%d,%d,%d), want (255,0,0,255)", r, g, b, a)
	}
}

func TestBlendSrcOverTransparent(t *testing.T) {
	// Fully transparent source leaves destination unchanged.
	r, g, b, a := BlendSrcOver(100, 150, 200, 255, 0, 0, 0, 0)
	if r != 100 || g != 150 || b != 200 || a != 255 {
		t.Errorf("transparent SrcOver = (%d,%d,%d,%d), want (100,150,200,255)", r, g, b, a)
	}
}

func TestBlendSrcOverSemiTransparent(t *testing.T) {
	// 50% alpha red over solid blue.
	r, g, b, a := BlendSrcOver(0, 0, 255, 255, 128, 0, 0, 128)
	// Expected: out_r = 128 + 0*(127)/255 = 128
	//           out_g = 0 + 0*(127)/255 = 0
	//           out_b = 0 + 255*127/255 ≈ 127
	//           out_a = 128 + 255*127/255 ≈ 255
	if r < 125 || r > 130 {
		t.Errorf("semi-transparent SrcOver R = %d, want ~128", r)
	}
	if b < 124 || b > 130 {
		t.Errorf("semi-transparent SrcOver B = %d, want ~127", b)
	}
	_ = g
	_ = a
}

func TestBlendPixel(t *testing.T) {
	dst := []uint8{0, 0, 255, 255} // solid blue
	BlendPixel(dst, 0, 255, 0, 0, 255, 128)
	// After blending half-coverage red over blue:
	// modulated source: (128, 0, 0, 128)
	// result ≈ (128, 0, 127, 255)
	if dst[0] < 125 || dst[0] > 130 {
		t.Errorf("BlendPixel R = %d, want ~128", dst[0])
	}
}

// ---------------------------------------------------------------------------
// Canvas Clear
// ---------------------------------------------------------------------------

func TestCanvasClear(t *testing.T) {
	c := NewCanvas(4, 4)
	c.Clear(plume.White)

	for i := 0; i < len(c.pixels); i += 4 {
		if c.pixels[i] != 255 || c.pixels[i+1] != 255 || c.pixels[i+2] != 255 || c.pixels[i+3] != 255 {
			t.Fatalf("pixel at byte offset %d = (%d,%d,%d,%d), want white",
				i, c.pixels[i], c.pixels[i+1], c.pixels[i+2], c.pixels[i+3])
		}
	}
}

// ---------------------------------------------------------------------------
// Canvas DrawRect
// ---------------------------------------------------------------------------

func TestCanvasDrawRect(t *testing.T) {
	c := NewCanvas(20, 20)
	c.Clear(plume.Transparent)
	c.DrawRect(Rect{X: 5, Y: 5, Width: 10, Height: 10}, plume.Fill(plume.Color{R: 255, G: 0, B: 0, A: 255}))

	// Check interior pixel (10, 10).
	off := (10*20 + 10) * 4
	if c.pixels[off+3] < 200 {
		t.Errorf("DrawRect interior alpha = %d, want >= 200", c.pixels[off+3])
	}
	if c.pixels[off] < 200 {
		t.Errorf("DrawRect interior R = %d, want >= 200", c.pixels[off])
	}

	// Check exterior pixel (0, 0).
	if c.pixels[3] != 0 {
		t.Errorf("DrawRect exterior alpha = %d, want 0", c.pixels[3])
	}
}

// ---------------------------------------------------------------------------
// Canvas DrawRoundedRect
// ---------------------------------------------------------------------------

func TestCanvasDrawRoundedRect(t *testing.T) {
	c := NewCanvas(40, 40)
	c.Clear(plume.Transparent)
	c.DrawRoundedRect(Rect{X: 5, Y: 5, Width: 30, Height: 30}, 8, plume.Fill(plume.Color{R: 0, G: 0, B: 255, A: 255}))

	// Centre pixel should be fully covered.
	off := (20*40 + 20) * 4
	if c.pixels[off+3] < 200 {
		t.Errorf("DrawRoundedRect centre alpha = %d, want >= 200", c.pixels[off+3])
	}

	// Corner pixel (6, 6) — just inside the bounding box but outside the
	// rounded corner — should have low or zero coverage.
	offCorner := (6*40 + 6) * 4
	if c.pixels[offCorner+3] > 128 {
		t.Errorf("DrawRoundedRect corner (6,6) alpha = %d, want <= 128 (AA corner)", c.pixels[offCorner+3])
	}
}

// ---------------------------------------------------------------------------
// Canvas DrawPath (triangle)
// ---------------------------------------------------------------------------

func TestCanvasDrawPathTriangle(t *testing.T) {
	c := NewCanvas(30, 30)
	c.Clear(plume.Transparent)

	var b geom.PathBuilder
	b.MoveTo(15, 2)
	b.LineTo(28, 28)
	b.LineTo(2, 28)
	b.Close()
	tri := b.Finish()

	c.DrawPath(tri, plume.Fill(plume.Color{R: 0, G: 200, B: 0, A: 255}))

	// Centre of triangle (~15, 20) should be covered.
	off := (20*30 + 15) * 4
	if c.pixels[off+3] < 200 {
		t.Errorf("triangle centre alpha = %d, want >= 200", c.pixels[off+3])
	}
}

// ---------------------------------------------------------------------------
// Canvas ToRGBA
// ---------------------------------------------------------------------------

func TestCanvasToRGBA(t *testing.T) {
	c := NewCanvas(8, 6)
	c.Clear(plume.Color{R: 10, G: 20, B: 30, A: 255})

	img := c.ToRGBA()
	if img.Bounds().Dx() != 8 || img.Bounds().Dy() != 6 {
		t.Fatalf("image size = %dx%d, want 8x6", img.Bounds().Dx(), img.Bounds().Dy())
	}
	// Spot check a pixel.
	r, g, b, a := img.At(3, 3).RGBA()
	// image.RGBA returns 16-bit values; shift down.
	if uint8(r>>8) != 10 || uint8(g>>8) != 20 || uint8(b>>8) != 30 || uint8(a>>8) != 255 {
		t.Errorf("pixel (3,3) = (%d,%d,%d,%d), want (10,20,30,255)",
			uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8))
	}
}

// ---------------------------------------------------------------------------
// PaintSource
// ---------------------------------------------------------------------------

func TestPaintSourceSolid(t *testing.T) {
	ps := NewSolidPaint(plume.Color{R: 42, G: 84, B: 126, A: 255})
	c := ps.ColorAt(100, 200)
	if c.R != 42 || c.G != 84 || c.B != 126 || c.A != 255 {
		t.Errorf("ColorAt = %+v, want {42 84 126 255}", c)
	}
}

// TestRasterizeOutOfBoundsWinding verifies that paths extending outside canvas bounds
// (e.g. negative X and Y > height) do not leak scanline coverage past their right edges.
func TestRasterizeOutOfBoundsWinding(t *testing.T) {
	const w, h = 200, 200
	r := NewRasterizer(w, h)

	// Circle centered below and to the left of the canvas
	circle := geom.Circle(50, 220, 100)
	r.AddPath(circle)

	dst := make([]uint8, w*h)
	r.Rasterize(dst)

	// Interior pixel of the arc (50, 150) should be covered
	if dst[150*w+50] < 200 {
		t.Errorf("interior pixel (50, 150) should be covered, got %d", dst[150*w+50])
	}

	// Pixels well outside the right edge of the circle (e.g. x=160..199) must be 0
	for y := 0; y < h; y++ {
		for x := 160; x < w; x++ {
			if a := dst[y*w+x]; a != 0 {
				t.Fatalf("scanline leak at (%d, %d): alpha = %d, want 0", x, y, a)
			}
		}
	}
}
