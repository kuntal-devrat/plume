package text

import (
	"testing"
)

func TestDetectDirection(t *testing.T) {
	if dir := DetectDirection("Hello World"); dir != DirectionLTR {
		t.Fatalf("expected LTR for English, got %v", dir)
	}
	if dir := DetectDirection("שלום עולם"); dir != DirectionRTL {
		t.Fatalf("expected RTL for Hebrew, got %v", dir)
	}
	if dir := DetectDirection("مرحبا بالعالم"); dir != DirectionRTL {
		t.Fatalf("expected RTL for Arabic, got %v", dir)
	}
}

func TestRTLShaping(t *testing.T) {
	font, err := LoadFontFile("../testdata/default.ttf")
	if err != nil {
		t.Skipf("skipping test; test font not available: %v", err)
	}

	layoutLTR := ShapeTextWithDirection(font, "ABC", 16, 0, DirectionLTR)
	layoutRTL := ShapeTextWithDirection(font, "ABC", 16, 0, DirectionRTL)

	if len(layoutLTR.Lines) == 0 || len(layoutRTL.Lines) == 0 {
		t.Fatalf("expected non-empty lines")
	}

	glyphsLTR := layoutLTR.Lines[0].Glyphs
	glyphsRTL := layoutRTL.Lines[0].Glyphs

	if len(glyphsLTR) != len(glyphsRTL) {
		t.Fatalf("glyph count mismatch")
	}

	// In LTR, glyph[0].X < glyph[len-1].X
	// In RTL, glyph[0].X > glyph[len-1].X
	if glyphsLTR[0].X >= glyphsLTR[len(glyphsLTR)-1].X {
		t.Fatalf("LTR expected ascending X coordinates")
	}
	if glyphsRTL[0].X <= glyphsRTL[len(glyphsRTL)-1].X {
		t.Fatalf("RTL expected descending X coordinates: %v vs %v",
			glyphsRTL[0].X, glyphsRTL[len(glyphsRTL)-1].X)
	}
}

func TestLigatureSubstitution(t *testing.T) {
	font := &Font{
		gsubParsed: true,
		ligatures: LigatureMap{
			10: []Ligature{ // glyph 10 ('f') followed by 11 ('i') -> 99 ('fi')
				{
					Components:  []uint16{11},
					Replacement: 99,
				},
			},
		},
	}

	input := []uint16{5, 10, 11, 20}
	output := font.ApplyLigatures(input)

	if len(output) != 3 {
		t.Fatalf("expected 3 glyphs after ligature replacement, got %v: %v", len(output), output)
	}
	if output[0] != 5 || output[1] != 99 || output[2] != 20 {
		t.Fatalf("unexpected ligature replacement output: %v", output)
	}
}
