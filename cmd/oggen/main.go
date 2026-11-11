package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/plume-engine/plume"
	"github.com/plume-engine/plume/geom"
	"github.com/plume-engine/plume/layout"
	"github.com/plume-engine/plume/raster"
	"github.com/plume-engine/plume/text"
)

func main() {
	title := flag.String("title", "Show HN: Plume – Zero-Cgo 2D Layout & Vector Engine in Pure Go", "Card headline title")
	tag := flag.String("tag", "SHOW HN", "Badge tag category")
	author := flag.String("author", "@plume-engine", "Author handle")
	outPath := flag.String("output", "og_card.png", "Output file path")
	bench := flag.Bool("bench", false, "Run 100-card benchmark")
	flag.Parse()

	fontPath := filepath.Join("testdata", "default.ttf")
	font, err := text.LoadFontFile(fontPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading font: %v\n", err)
		os.Exit(1)
	}
	cache := text.NewGlyphCache(2048)

	const (
		cardW = 1200
		cardH = 630
	)

	generateCard := func() *raster.Canvas {
		canvas := raster.NewCanvas(cardW, cardH)
		canvas.Clear(plume.Hex("#0b0f19"))

		// 1. Exterior decorative border
		borderRect := raster.Rect{X: 24, Y: 24, Width: cardW - 48, Height: cardH - 48}
		canvas.DrawRoundedRect(borderRect, 20, plume.Fill(plume.Hex("#111827")))

		// Top glow gradient line
		topAccent := raster.Rect{X: 24, Y: 24, Width: cardW - 48, Height: 6}
		canvas.DrawRoundedRect(topAccent, 3, plume.Fill(plume.Hex("#38bdf8")))

		// Decorative vector circles in bottom-right corner
		circ1 := geom.Circle(cardW-120, cardH-120, 80)
		canvas.DrawPath(circ1, plume.Fill(plume.Hex("#0f172a")))
		circ2 := geom.Circle(cardW-100, cardH-100, 48)
		canvas.DrawPath(circ2, plume.Fill(plume.Hex("#1e293b")))

		// 2. Category badge
		badgeW := float32(len(*tag))*9 + 28
		badgeRect := raster.Rect{X: 72, Y: 72, Width: badgeW, Height: 32}
		canvas.DrawRoundedRect(badgeRect, 6, plume.Fill(plume.Hex("#0369a1")))
		text.DrawText(canvas, cache, font, *tag, badgeRect.X+14, badgeRect.Y+22, 12, plume.White)

		// 3. Main title (wrapped)
		titleH := text.DrawTextWrapped(canvas, cache, font, *title, 72, 170, cardW-180, 40, plume.White)

		// Subtitle
		subtitle := "Pure Go 2D vector rasterization and CSS flexbox/grid layout."
		text.DrawTextWrapped(canvas, cache, font, subtitle, 72, 190+titleH, cardW-220, 20, plume.Hex("#9ca3af"))

		// Footer: Author & Brand
		footerY := float32(cardH - 84)
		logoCircle := geom.Circle(88, footerY-4, 14)
		canvas.DrawPath(logoCircle, plume.Fill(plume.Hex("#38bdf8")))
		text.DrawText(canvas, cache, font, "Plume", 112, footerY+3, 20, plume.White)
		text.DrawText(canvas, cache, font, "• "+*author, 185, footerY+2, 16, plume.Hex("#6b7280"))

		// Stat badge in footer
		statBadge := raster.Rect{X: cardW - 200, Y: footerY - 18, Width: 128, Height: 32}
		canvas.DrawRoundedRect(statBadge, 6, plume.Fill(plume.Hex("#14532d")))
		text.DrawText(canvas, cache, font, "PURE GO", statBadge.X+24, statBadge.Y+22, 12, plume.Hex("#4ade80"))

		return canvas
	}

	card := generateCard()
	if err := plume.SavePNG(card.ToRGBA(), *outPath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save card: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Saved Open Graph card to %s (%dx%d px)\n", *outPath, cardW, cardH)

	if *bench {
		const n = 200
		fmt.Printf("Benchmarking card generation (%d cards)...\n", n)
		start := time.Now()
		for i := 0; i < n; i++ {
			_ = generateCard()
		}
		elapsed := time.Since(start)
		cardsPerSec := float64(n) / elapsed.Seconds()
		msPerCard := elapsed.Seconds() * 1000 / float64(n)

		fmt.Printf("Performance:\n")
		fmt.Printf("  Total time: %v\n", elapsed)
		fmt.Printf("  Latency:    %.2f ms/card\n", msPerCard)
		fmt.Printf("  Throughput: %.0f cards/second\n", cardsPerSec)
	}

	_ = layout.NewTree // ensure layout import is preserved
}
