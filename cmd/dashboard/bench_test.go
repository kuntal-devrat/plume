package main

import (
	"path/filepath"
	"testing"

	"github.com/plume-engine/plume"
	"github.com/plume-engine/plume/layout"
	"github.com/plume-engine/plume/raster"
	"github.com/plume-engine/plume/text"
)

func BenchmarkDashboardRender(b *testing.B) {
	fontPath := filepath.Join("..", "..", "testdata", "default.ttf")
	font, err := text.LoadFontFile(fontPath)
	if err != nil {
		b.Fatal(err)
	}
	cache := text.NewGlyphCache(4096)

	const (
		w = 1280
		h = 800
	)

	canvas := raster.NewCanvas(w, h)
	tree := layout.NewTree()
	root := tree.NewNode(layout.Style{
		Display:             layout.DisplayGrid,
		Width:               layout.Px(w),
		Height:              layout.Px(h),
		GridTemplateColumns: []layout.Dimension{layout.Fr(1), layout.Fr(1), layout.Fr(1), layout.Fr(1)},
		Gap:                 layout.Px(16),
		Padding:             layout.Uniform(24),
	})

	for i := 0; i < 16; i++ {
		card := tree.NewNode(layout.Style{})
		tree.AddChild(root, card)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		tree.Compute(w, h)
		canvas.Clear(plume.Hex("#0b0f19"))

		for j := 0; j < 16; j++ {
			r := tree.ComputedRect(layout.NodeID(j + 1))
			canvas.DrawRoundedRect(r, 12, plume.Fill(plume.Hex("#1e293b")))
			text.DrawText(canvas, cache, font, "Metric Card", r.X+16, r.Y+32, 16, plume.White)
		}
	}
}
