package text

import (
	"path/filepath"
	"testing"
)

func loadTestFont(t *testing.T) *Font {
	fontPath := filepath.Join("..", "testdata", "default.ttf")
	f, err := LoadFontFile(fontPath)
	if err != nil {
		t.Fatalf("failed to load test font: %v", err)
	}
	return f
}

func TestParseFont(t *testing.T) {
	f := loadTestFont(t)
	if f.UnitsPerEm() == 0 {
		t.Errorf("UnitsPerEm = 0, want > 0")
	}
	if f.NumGlyphs() == 0 {
		t.Errorf("NumGlyphs = 0, want > 0")
	}

	lm := f.LineMetrics(16)
	if lm.Ascent <= 0 {
		t.Errorf("Ascent = %.2f, want > 0", lm.Ascent)
	}
	if lm.LineHeight <= 0 {
		t.Errorf("LineHeight = %.2f, want > 0", lm.LineHeight)
	}
}

func TestCmapLookup(t *testing.T) {
	f := loadTestFont(t)

	// ASCII characters must resolve to valid non-zero glyphs
	chars := []rune{'A', 'a', 'Z', 'z', '0', '9', ' ', '$'}
	for _, c := range chars {
		gid := f.GlyphIndex(c)
		if gid == 0 && c != ' ' { // space might have outline or not, but index is usually non-zero
			t.Errorf("GlyphIndex(%c) = 0, want non-zero glyph", c)
		}
	}
}

func TestGlyphPath(t *testing.T) {
	f := loadTestFont(t)
	gid := f.GlyphIndex('B')
	if gid == 0 {
		t.Fatal("GlyphIndex('B') is 0")
	}

	path, err := f.GlyphPath(gid, 24)
	if err != nil {
		t.Fatalf("GlyphPath failed: %v", err)
	}

	if len(path.Verbs) == 0 {
		t.Errorf("GlyphPath('B') produced empty verbs")
	}
	if len(path.Points) == 0 {
		t.Errorf("GlyphPath('B') produced empty points")
	}
}

func TestShaperSingleLine(t *testing.T) {
	f := loadTestFont(t)
	layout := ShapeText(f, "Hello, World!", 20, 0)

	if len(layout.Lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(layout.Lines))
	}
	line := layout.Lines[0]
	if len(line.Glyphs) != len("Hello, World!") {
		t.Errorf("expected %d glyphs, got %d", len("Hello, World!"), len(line.Glyphs))
	}
	if line.Width <= 0 {
		t.Errorf("line width = %.2f, want > 0", line.Width)
	}
}

func TestShaperWordWrap(t *testing.T) {
	f := loadTestFont(t)
	text := "The quick brown fox jumps over the lazy dog"

	// Measure single line width
	unwrapped := ShapeText(f, text, 20, 0)
	if len(unwrapped.Lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(unwrapped.Lines))
	}
	fullWidth := unwrapped.TotalWidth

	// Wrap at half width
	wrapped := ShapeText(f, text, 20, fullWidth*0.5)
	if len(wrapped.Lines) < 2 {
		t.Errorf("expected at least 2 wrapped lines, got %d", len(wrapped.Lines))
	}

	for i, l := range wrapped.Lines {
		if l.Width > fullWidth*0.55 { // allow slight tolerance
			t.Errorf("line %d width %.2f exceeds max width %.2f", i, l.Width, fullWidth*0.5)
		}
	}
}

func TestGlyphCache(t *testing.T) {
	f := loadTestFont(t)
	cache := NewGlyphCache(100)

	gid := f.GlyphIndex('M')
	cg1 := cache.GetOrRasterize(f, gid, 32)
	if cg1.Width <= 0 || cg1.Height <= 0 || len(cg1.Alpha) == 0 {
		t.Fatalf("cached glyph invalid: %+v", cg1)
	}

	// Retrieve again: must hit cache
	cg2 := cache.GetOrRasterize(f, gid, 32)
	if cg2.Width != cg1.Width || cg2.Height != cg1.Height {
		t.Errorf("cache returned different glyph dimensions")
	}
}
