package geom

import "testing"

func BenchmarkFlattenCircle(b *testing.B) {
	p := Circle(100, 100, 50)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Flatten(p, 0.25)
	}
}

func BenchmarkFlattenComplexPath(b *testing.B) {
	var pb PathBuilder
	pb.MoveTo(0, 0)
	for i := 0; i < 50; i++ {
		x := float32(i * 10)
		pb.CubicTo(x+2, 10, x+8, -10, x+10, 0)
		pb.QuadTo(x+15, 20, x+20, 0)
	}
	p := pb.Finish()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Flatten(p, 0.25)
	}
}

func BenchmarkStrokeRect(b *testing.B) {
	p := Rect(10, 10, 200, 100)
	opts := StrokeOptions{
		Width: 4,
		Cap:   CapRound,
		Join:  JoinRound,
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Stroke(p, opts)
	}
}

func BenchmarkBoundingBox(b *testing.B) {
	p := RoundedRect(10, 10, 300, 200, 16)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _, _, _ = BoundingBox(p)
	}
}
