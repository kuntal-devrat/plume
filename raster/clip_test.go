package raster

import (
	"testing"

	plume "github.com/plume-engine/plume"
	"github.com/plume-engine/plume/geom"
)

func TestClipRect(t *testing.T) {
	c := NewCanvas(100, 100)
	c.Clear(plume.Color{R: 0, G: 0, B: 0, A: 0})

	// Clip to center [25, 25, 50, 50]
	c.PushClipRect(Rect{X: 25, Y: 25, Width: 50, Height: 50})

	// Draw full canvas red rect
	c.DrawRect(Rect{X: 0, Y: 0, Width: 100, Height: 100}, plume.Fill(plume.Color{R: 255, G: 0, B: 0, A: 255}))

	c.PopClip()

	pix := c.Pixels()

	// Inside clipped region: (50, 50) should be red
	idxInside := (50*100 + 50) * 4
	if pix[idxInside] != 255 || pix[idxInside+3] != 255 {
		t.Fatalf("inside clip expected red, got RGBA: %v, %v, %v, %v",
			pix[idxInside], pix[idxInside+1], pix[idxInside+2], pix[idxInside+3])
	}

	// Outside clipped region: (10, 10) should be transparent
	idxOutside := (10*100 + 10) * 4
	if pix[idxOutside+3] != 0 {
		t.Fatalf("outside clip expected transparent, got RGBA: %v, %v, %v, %v",
			pix[idxOutside], pix[idxOutside+1], pix[idxOutside+2], pix[idxOutside+3])
	}
}

func TestClipPath(t *testing.T) {
	c := NewCanvas(100, 100)
	c.Clear(plume.Color{R: 0, G: 0, B: 0, A: 0})

	// Clip to circular region around (50, 50) r=30
	c.PushClipPath(geom.Circle(50, 50, 30))

	// Draw full canvas blue rect
	c.DrawRect(Rect{X: 0, Y: 0, Width: 100, Height: 100}, plume.Fill(plume.Color{R: 0, G: 0, B: 255, A: 255}))

	c.PopClip()

	pix := c.Pixels()

	// Center (50, 50) must be blue
	idxInside := (50*100 + 50) * 4
	if pix[idxInside+2] != 255 || pix[idxInside+3] != 255 {
		t.Fatalf("circle inside expected blue, got RGBA: %v, %v, %v, %v",
			pix[idxInside], pix[idxInside+1], pix[idxInside+2], pix[idxInside+3])
	}

	// Corner (5, 5) must be 0
	idxOutside := (5*100 + 5) * 4
	if pix[idxOutside+3] != 0 {
		t.Fatalf("circle outside expected transparent, got RGBA: %v, %v, %v, %v",
			pix[idxOutside], pix[idxOutside+1], pix[idxOutside+2], pix[idxOutside+3])
	}
}

func TestBlendModes(t *testing.T) {
	// Multiply: white (1, 1, 1) * color (r, g, b) = (r, g, b)
	r, g, b, a := Blend(255, 255, 255, 255, 100, 150, 200, 255, BlendModeMultiply)
	if r != 100 || g != 150 || b != 200 || a != 255 {
		t.Fatalf("multiply white failed: %v, %v, %v, %v", r, g, b, a)
	}

	// Clear mode always yields zero
	r, g, b, a = Blend(100, 100, 100, 255, 200, 200, 200, 255, BlendModeClear)
	if r != 0 || g != 0 || b != 0 || a != 0 {
		t.Fatalf("clear failed: %v, %v, %v, %v", r, g, b, a)
	}

	// Screen mode: 0 screen x = x
	r, g, b, a = Blend(0, 0, 0, 255, 120, 130, 140, 255, BlendModeScreen)
	if r != 120 || g != 130 || b != 140 || a != 255 {
		t.Fatalf("screen 0 failed: %v, %v, %v, %v", r, g, b, a)
	}
}

func TestLinearGammaBlending(t *testing.T) {
	// Verify sRGB transfer round-trip
	for i := 0; i < 256; i++ {
		lin := SrgbToLinear(uint8(i))
		back := LinearToSrgb(lin)
		diff := int(back) - i
		if diff < -1 || diff > 1 {
			t.Fatalf("gamma round-trip error at %d: got %d", i, back)
		}
	}

	// BlendLinearSrcOver should composite without panic
	r, g, b, a := BlendLinearSrcOver(0, 255, 0, 255, 255, 0, 0, 128)
	if a != 255 || r == 0 || g == 0 {
		t.Fatalf("linear blend unexpected result: %v, %v, %v, %v", r, g, b, a)
	}
}

func TestSubpixelMask(t *testing.T) {
	c := NewCanvas(10, 10)
	c.Clear(plume.Color{R: 0, G: 0, B: 0, A: 255})

	// 1-pixel RGB mask with full red subpixel coverage, 0 green, 0 blue
	subpixelMask := []uint8{255, 0, 0}
	c.DrawSubpixelMask(subpixelMask, 5, 5, 1, 1, plume.Color{R: 255, G: 255, B: 255, A: 255})

	pix := c.Pixels()
	idx := (5*10 + 5) * 4
	// Red subpixel should be white (255), Green and Blue remain black (0)
	if pix[idx] != 255 || pix[idx+1] != 0 || pix[idx+2] != 0 {
		t.Fatalf("expected R=255, G=0, B=0, got %v, %v, %v", pix[idx], pix[idx+1], pix[idx+2])
	}
}
