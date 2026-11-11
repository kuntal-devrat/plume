package raster

import (
	"testing"

	plume "github.com/plume-engine/plume"
	"github.com/plume-engine/plume/geom"
)

func BenchmarkRasterizeRect(b *testing.B) {
	c := NewCanvas(800, 600)
	paint := plume.Fill(plume.Hex("#3b82f6"))
	r := Rect{X: 50, Y: 50, Width: 700, Height: 500}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		c.DrawRect(r, paint)
	}
}

func BenchmarkRasterizeRoundedRect(b *testing.B) {
	c := NewCanvas(800, 600)
	paint := plume.Fill(plume.Hex("#10b981"))
	r := Rect{X: 100, Y: 100, Width: 400, Height: 300}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		c.DrawRoundedRect(r, 16, paint)
	}
}

func BenchmarkCanvasClear(b *testing.B) {
	c := NewCanvas(1920, 1080)
	col := plume.Hex("#0f172a")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		c.Clear(col)
	}
}

func BenchmarkCanvasDrawCircle(b *testing.B) {
	c := NewCanvas(800, 600)
	paint := plume.Fill(plume.Hex("#ef4444"))
	p := geom.Circle(400, 300, 150)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		c.DrawPath(p, paint)
	}
}
