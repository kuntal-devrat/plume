package text

import (
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkParseFont(b *testing.B) {
	data, err := loadFontBytes()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = ParseFont(data)
	}
}

func BenchmarkShapeParagraph(b *testing.B) {
	f, err := loadTestFontBench()
	if err != nil {
		b.Fatal(err)
	}
	text := "Plume is a zero-Cgo, mechanical-sympathy 2D Layout & Vector Rasterization Engine for Go."

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ShapeText(f, text, 16, 400)
	}
}

func BenchmarkGlyphCacheLookup(b *testing.B) {
	f, err := loadTestFontBench()
	if err != nil {
		b.Fatal(err)
	}
	cache := NewGlyphCache(256)
	gid := f.GlyphIndex('W')

	// Warm cache
	_ = cache.GetOrRasterize(f, gid, 24)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = cache.GetOrRasterize(f, gid, 24)
	}
}

func loadFontBytes() ([]byte, error) {
	fontPath := filepath.Join("..", "testdata", "default.ttf")
	return os.ReadFile(fontPath)
}

func loadTestFontBench() (*Font, error) {
	fontPath := filepath.Join("..", "testdata", "default.ttf")
	return LoadFontFile(fontPath)
}
