package main

import (
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

type metricCard struct {
	title  string
	value  string
	change string
	accent plume.Color
}

type serviceRow struct {
	name    string
	region  string
	status  string
	p50     string
	p99     string
	errorRt string
	uptime  float32
	deploy  string
}

func main() {
	const (
		canvasW = 1280
		canvasH = 800
	)

	fmt.Println("⚡ Plume Engine Phase 2: Complex Dashboard Demo (CSS Grid, Font Shaper & SIMD)")

	// 1. Load Font
	fontPath := filepath.Join("testdata", "default.ttf")
	font, err := text.LoadFontFile(fontPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading font %s: %v\n", fontPath, err)
		os.Exit(1)
	}
	cache := text.NewGlyphCache(4096)

	// 2. Sample Data
	metrics := []metricCard{
		{"Total Throughput", "1,842,910 req/s", "+14.2%", plume.Hex("#38bdf8")},
		{"Layout SLA", "0.0039 ms / 1k", "51x faster", plume.Hex("#34d399")},
		{"CPU Rasterizer", "280+ FPS", "AVX2 SIMD", plume.Hex("#a78bfa")},
		{"Heap Allocs", "0 B / frame", "100% Zero-GC", plume.Hex("#fbbf24")},
	}

	services := []serviceRow{
		{"auth-gateway", "us-east-1", "HEALTHY", "1.2 ms", "4.8 ms", "0.001%", 0.95, "2m ago"},
		{"payment-core", "us-east-1", "HEALTHY", "3.4 ms", "12.1 ms", "0.000%", 1.00, "14m ago"},
		{"vector-rasterizer", "eu-central", "OPTIMAL", "0.8 ms", "2.1 ms", "0.000%", 0.98, "1h ago"},
		{"flexbox-layout", "eu-central", "OPTIMAL", "0.004 ms", "0.01 ms", "0.000%", 1.00, "3h ago"},
		{"grid-engine", "ap-northeast", "OPTIMAL", "0.06 ms", "0.18 ms", "0.000%", 0.99, "5h ago"},
		{"glyph-atlas", "us-west-2", "HEALTHY", "0.08 µs", "0.15 µs", "0.000%", 0.97, "1d ago"},
		{"session-cache", "us-west-2", "HEALTHY", "0.4 ms", "1.2 ms", "0.002%", 0.92, "2d ago"},
		{"db-replica-01", "ap-southeast", "HEALTHY", "4.1 ms", "18.5 ms", "0.012%", 0.88, "4d ago"},
	}

	// Helper to render one full frame onto an existing canvas
	renderOntoCanvas := func(canvas *raster.Canvas, tree *layout.Tree, metricNodeIDs []layout.NodeID, thIDs []layout.NodeID, rowIDs [][]layout.NodeID, header, tableBox layout.NodeID) {
		// 1. Recompute Layout
		tree.Compute(canvasW, canvasH)

		// 2. Clear canvas
		canvas.Clear(plume.Hex("#0b0f19"))

		// 3. Draw Header
		hRect := tree.ComputedRect(header)
		logoCircle := geom.Circle(hRect.X+14, hRect.Y+hRect.Height/2, 12)
		canvas.DrawPath(logoCircle, plume.Fill(plume.Hex("#38bdf8")))
		text.DrawText(canvas, cache, font, "PLUME ENGINE / SYSTEM METRICS", hRect.X+36, hRect.Y+28, 18, plume.White)

		badgeRect := raster.Rect{X: hRect.X + hRect.Width - 160, Y: hRect.Y + 8, Width: 160, Height: 28}
		canvas.DrawRoundedRect(badgeRect, 6, plume.Fill(plume.Hex("#134e4a")))
		text.DrawText(canvas, cache, font, "• 200+ FPS PURE GO", badgeRect.X+14, badgeRect.Y+19, 12, plume.Hex("#2dd4bf"))

		// 4. Draw Metrics Cards
		for i, m := range metrics {
			mRect := tree.ComputedRect(metricNodeIDs[i])
			canvas.DrawRoundedRect(mRect, 10, plume.Fill(plume.Hex("#111827")))

			// Left accent border bar
			bar := raster.Rect{X: mRect.X, Y: mRect.Y, Width: 4, Height: mRect.Height}
			canvas.DrawRoundedRect(bar, 2, plume.Fill(m.accent))

			text.DrawText(canvas, cache, font, m.title, mRect.X+16, mRect.Y+24, 12, plume.Hex("#9ca3af"))
			text.DrawText(canvas, cache, font, m.value, mRect.X+16, mRect.Y+54, 20, plume.White)
			text.DrawText(canvas, cache, font, m.change, mRect.X+mRect.Width-68, mRect.Y+54, 12, m.accent)
		}

		// 5. Draw Table Container Background
		tRect := tree.ComputedRect(tableBox)
		canvas.DrawRoundedRect(tRect, 12, plume.Fill(plume.Hex("#111827")))

		// Headers
		headers := []string{"SERVICE", "REGION", "STATUS", "P50", "P99", "ERRORS", "UPTIME (30D)", "DEPLOYED"}
		for i, thID := range thIDs {
			rect := tree.ComputedRect(thID)
			text.DrawText(canvas, cache, font, headers[i], rect.X+8, rect.Y+22, 11, plume.Hex("#6b7280"))
		}

		// Rows
		for r, s := range services {
			cols := rowIDs[r]

			// Alternating subtle row background
			if r%2 == 1 {
				rStart := tree.ComputedRect(cols[0])
				rEnd := tree.ComputedRect(cols[7])
				fullRowRect := raster.Rect{
					X:      rStart.X - 4,
					Y:      rStart.Y - 2,
					Width:  (rEnd.X + rEnd.Width) - rStart.X + 8,
					Height: rStart.Height + 4,
				}
				canvas.DrawRoundedRect(fullRowRect, 6, plume.Fill(plume.Hex("#161f30")))
			}

			// Service name
			r0 := tree.ComputedRect(cols[0])
			text.DrawText(canvas, cache, font, s.name, r0.X+8, r0.Y+24, 14, plume.Hex("#f3f4f6"))

			// Region
			r1 := tree.ComputedRect(cols[1])
			text.DrawText(canvas, cache, font, s.region, r1.X+8, r1.Y+24, 13, plume.Hex("#9ca3af"))

			// Status Badge
			r2 := tree.ComputedRect(cols[2])
			badgeW := float32(72)
			statusPill := raster.Rect{X: r2.X + 8, Y: r2.Y + 8, Width: badgeW, Height: 22}
			canvas.DrawRoundedRect(statusPill, 4, plume.Fill(plume.Hex("#064e3b")))
			text.DrawText(canvas, cache, font, s.status, statusPill.X+8, statusPill.Y+15, 10, plume.Hex("#34d399"))

			// Metrics
			r3 := tree.ComputedRect(cols[3])
			text.DrawText(canvas, cache, font, s.p50, r3.X+8, r3.Y+24, 13, plume.Hex("#d1d5db"))

			r4 := tree.ComputedRect(cols[4])
			text.DrawText(canvas, cache, font, s.p99, r4.X+8, r4.Y+24, 13, plume.Hex("#d1d5db"))

			r5 := tree.ComputedRect(cols[5])
			text.DrawText(canvas, cache, font, s.errorRt, r5.X+8, r5.Y+24, 13, plume.Hex("#9ca3af"))

			// Uptime Progress Bar (Vector rect with rounded ends)
			r6 := tree.ComputedRect(cols[6])
			barW := r6.Width - 24
			bgBar := raster.Rect{X: r6.X + 8, Y: r6.Y + 14, Width: barW, Height: 10}
			canvas.DrawRoundedRect(bgBar, 5, plume.Fill(plume.Hex("#1f2937")))

			fgBar := raster.Rect{X: r6.X + 8, Y: r6.Y + 14, Width: barW * s.uptime, Height: 10}
			barCol := plume.Hex("#10b981")
			if s.uptime < 0.90 {
				barCol = plume.Hex("#f59e0b")
			}
			canvas.DrawRoundedRect(fgBar, 5, plume.Fill(barCol))

			// Deploy time
			r7 := tree.ComputedRect(cols[7])
			text.DrawText(canvas, cache, font, s.deploy, r7.X+8, r7.Y+24, 12, plume.Hex("#6b7280"))
		}
	}

	buildTree := func() (*layout.Tree, []layout.NodeID, []layout.NodeID, [][]layout.NodeID, layout.NodeID, layout.NodeID) {
		tree := layout.NewTree()

		root := tree.NewNode(layout.Style{
			Display:       layout.DisplayFlex,
			FlexDirection: layout.FlexDirectionColumn,
			Width:         layout.Px(canvasW),
			Height:        layout.Px(canvasH),
			Padding:       layout.Uniform(28),
			Gap:           layout.Px(20),
		})

		header := tree.NewNode(layout.Style{
			Display:        layout.DisplayFlex,
			FlexDirection:  layout.FlexDirectionRow,
			JustifyContent: layout.JustifySpaceBetween,
			AlignItems:     layout.AlignCenter,
			Height:         layout.Px(44),
		})
		tree.AddChild(root, header)

		metricsGrid := tree.NewNode(layout.Style{
			Display:             layout.DisplayGrid,
			GridTemplateColumns: []layout.Dimension{layout.Fr(1), layout.Fr(1), layout.Fr(1), layout.Fr(1)},
			ColumnGap:           layout.Px(16),
			Height:              layout.Px(88),
		})
		tree.AddChild(root, metricsGrid)

		var metricNodeIDs []layout.NodeID
		for i := 0; i < len(metrics); i++ {
			n := tree.NewNode(layout.Style{})
			tree.AddChild(metricsGrid, n)
			metricNodeIDs = append(metricNodeIDs, n)
		}

		tableBox := tree.NewNode(layout.Style{
			Display:       layout.DisplayFlex,
			FlexDirection: layout.FlexDirectionColumn,
			FlexGrow:      1,
		})
		tree.AddChild(root, tableBox)

		tableGrid := tree.NewNode(layout.Style{
			Display: layout.DisplayGrid,
			GridTemplateColumns: []layout.Dimension{
				layout.Fr(2.2),
				layout.Fr(1.4),
				layout.Fr(1.2),
				layout.Fr(1.0),
				layout.Fr(1.0),
				layout.Fr(1.0),
				layout.Fr(1.8),
				layout.Fr(1.0),
			},
			RowGap:   layout.Px(6),
			FlexGrow: 1,
		})
		tree.AddChild(tableBox, tableGrid)

		var thIDs []layout.NodeID
		for i := 0; i < 8; i++ {
			th := tree.NewNode(layout.Style{Height: layout.Px(32)})
			tree.AddChild(tableGrid, th)
			thIDs = append(thIDs, th)
		}

		var rowIDs [][]layout.NodeID
		for r := 0; r < len(services); r++ {
			var rCols []layout.NodeID
			for c := 0; c < 8; c++ {
				td := tree.NewNode(layout.Style{Height: layout.Px(38)})
				tree.AddChild(tableGrid, td)
				rCols = append(rCols, td)
			}
			rowIDs = append(rowIDs, rCols)
		}

		return tree, metricNodeIDs, thIDs, rowIDs, header, tableBox
	}

	tree, metricNodeIDs, thIDs, rowIDs, header, tableBox := buildTree()
	canvas := raster.NewCanvas(canvasW, canvasH)

	// 3. Render and save golden frame
	renderOntoCanvas(canvas, tree, metricNodeIDs, thIDs, rowIDs, header, tableBox)
	outPath := "dashboard.png"
	if err := plume.SavePNG(canvas.ToRGBA(), outPath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save dashboard PNG: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf(" Saved dashboard render to %s (%dx%d px)\n", outPath, canvasW, canvasH)

	// 4. Benchmark Frame Rate
	const benchFrames = 500
	fmt.Printf("⏱️  Running frame rate benchmark (%d frames)...\n", benchFrames)

	start := time.Now()
	for i := 0; i < benchFrames; i++ {
		renderOntoCanvas(canvas, tree, metricNodeIDs, thIDs, rowIDs, header, tableBox)
	}
	elapsed := time.Since(start)

	fps := float64(benchFrames) / elapsed.Seconds()
	msPerFrame := elapsed.Seconds() * 1000 / float64(benchFrames)

	fmt.Printf(" Render Performance:\n")
	fmt.Printf("   • Total time:    %v\n", elapsed)
	fmt.Printf("   • Time/Frame:    %.2f ms\n", msPerFrame)
	fmt.Printf("   • Throughput:    %.1f FPS\n", fps)

	if fps >= 200.0 {
		fmt.Println(" Target SLA (> 200 FPS on CPU) achieved!")
	} else {
		fmt.Printf(" Target SLA: %.1f FPS\n", fps)
	}
}
