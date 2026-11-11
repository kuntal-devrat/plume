package plume

import (
	"testing"
)

func TestHexParsing(t *testing.T) {
	tests := []struct {
		input string
		want  Color
	}{
		{"#fff", Color{255, 255, 255, 255}},
		{"#000", Color{0, 0, 0, 255}},
		{"#FF0000", Color{255, 0, 0, 255}},
		{"#00ff00", Color{0, 255, 0, 255}},
		{"0000ff", Color{0, 0, 255, 255}},
		{"#0f172a", Color{15, 23, 42, 255}},
		{"#1e293b", Color{30, 41, 59, 255}},
		{"#FF000080", Color{128, 0, 0, 128}}, // premultiplied
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Hex(tt.input)
			if got != tt.want {
				t.Errorf("Hex(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

func TestColorFromRGBA(t *testing.T) {
	// Fully opaque — no premultiplication needed.
	c := ColorFromRGBA(100, 150, 200, 255)
	if c != (Color{100, 150, 200, 255}) {
		t.Errorf("got %+v", c)
	}

	// Fully transparent.
	c = ColorFromRGBA(100, 150, 200, 0)
	if c != (Color{}) {
		t.Errorf("got %+v", c)
	}

	// Half alpha: 200 * 128 / 255 ≈ 100
	c = ColorFromRGBA(200, 200, 200, 128)
	if c.A != 128 {
		t.Errorf("alpha = %d, want 128", c.A)
	}
}

func TestFill(t *testing.T) {
	p := Fill(White)
	if p.Color != White {
		t.Error("Fill(White) should produce white paint")
	}
}
