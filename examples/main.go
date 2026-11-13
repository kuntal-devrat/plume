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
	spark  []float32
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

func findFontPath() string {
	candidates := []string{
		filepath.Join("testdata", "default.ttf"),
		filepath.Join("..", "testdata", "default.ttf"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "testdata/default.ttf"
}

func main() {
	const (
		canvasW = 1280
		canvasH = 800
	)

	fmt.Printf("Rendering Plume high-resolution showcase (%dx%d px)...\n", canvasW, canvasH)

	// 1. Load TrueType Font & initialize glyph cache
	fontPath := findFontPath()
	font, err := text.LoadFontFile(fontPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading font %s: %v\n", fontPath, err)
		os.Exit(1)
	}
	cache := text.NewGlyphCache(4096)

	// 2. Sample Data
	metrics := []metricCard{
		{"TOTAL REQUESTS", "1,842,910", "+14.2%", plume.Hex("#38bdf8"), []float32{12, 18, 14, 25, 22, 35, 42}},
		{"P99 LATENCY", "4.2 ms", "-8.1%", plume.Hex("#34d399"), []float32{30, 26, 28, 20, 18, 15, 12}},
		{"ERROR RATE", "0.012%", "-0.004%", plume.Hex("#a78bfa"), []float32{5, 8, 4, 10, 6, 3, 2}},
		{"ACTIVE CLUSTERS", "48 / 48", "OPTIMAL", plume.Hex("#fbbf24"), []float32{40, 42, 44, 46, 47, 48, 48}},
	}

	services := []serviceRow{
		{"auth-gateway", "us-east-1", "HEALTHY", "1.2 ms", "4.8 ms", "0.001%", 0.998, "2m ago"},
		{"payment-core", "us-east-1", "HEALTHY", "3.4 ms", "12.1 ms", "0.000%", 1.000, "14m ago"},
		{"vector-rasterizer", "eu-central", "OPTIMAL", "0.8 ms", "2.1 ms", "0.000%", 0.995, "1h ago"},
		{"flexbox-layout", "eu-central", "OPTIMAL", "0.004 ms", "0.01 ms", "0.000%", 1.000, "3h ago"},
		{"grid-engine", "ap-northeast", "OPTIMAL", "0.06 ms", "0.18 ms", "0.000%", 0.999, "5h ago"},
		{"glyph-atlas", "us-west-2", "HEALTHY", "0.08 µs", "0.15 µs", "0.000%", 0.992, "1d ago"},
		{"session-cache", "us-west-2", "HEALTHY", "0.4 ms", "1.2 ms", "0.002%", 0.985, "2d ago"},
		{"db-replica-01", "ap-southeast", "HEALTHY", "4.1 ms", "18.5 ms", "0.012%", 0.970, "4d ago"},
	}

	// 3. Build Layout Tree (Flexbox root + Grid metrics + Grid table)
	tree := layout.NewTree()

	root := tree.NewNode(layout.Style{
		Display:       layout.DisplayFlex,
		FlexDirection: layout.FlexDirectionColumn,
		Width:         layout.Px(canvasW),
		Height:        layout.Px(canvasH),
		Padding:       layout.Uniform(32),
		Gap:           layout.Px(24),
	})

	// Header bar: Flex row with space-between
	header := tree.NewNode(layout.Style{
		Display:        layout.DisplayFlex,
		FlexDirection:  layout.FlexDirectionRow,
		JustifyContent: layout.JustifySpaceBetween,
		AlignItems:     layout.AlignCenter,
		Height:         layout.Px(50),
	})
	tree.AddChild(root, header)

	// Metrics row: CSS Grid with 4 equal fr tracks
	metricsGrid := tree.NewNode(layout.Style{
		Display: layout.DisplayGrid,
		GridTemplateColumns: []layout.Dimension{
			layout.Fr(1), layout.Fr(1), layout.Fr(1), layout.Fr(1),
		},
		ColumnGap: layout.Px(18),
		Height:    layout.Px(110),
	})
	tree.AddChild(root, metricsGrid)

	var metricNodeIDs []layout.NodeID
	for i := 0; i < len(metrics); i++ {
		n := tree.NewNode(layout.Style{})
		tree.AddChild(metricsGrid, n)
		metricNodeIDs = append(metricNodeIDs, n)
	}

	// Main table container: Flex column fill
	tableBox := tree.NewNode(layout.Style{
		Display:       layout.DisplayFlex,
		FlexDirection: layout.FlexDirectionColumn,
		FlexGrow:      1,
		Padding:       layout.Uniform(20),
		Gap:           layout.Px(12),
	})
	tree.AddChild(root, tableBox)

	// Table header & rows Grid
	tableGrid := tree.NewNode(layout.Style{
		Display: layout.DisplayGrid,
		GridTemplateColumns: []layout.Dimension{
			layout.Fr(2.2),
			layout.Fr(1.3),
			layout.Fr(1.2),
			layout.Fr(1.0),
			layout.Fr(1.0),
			layout.Fr(1.0),
			layout.Fr(1.8),
			layout.Fr(1.0),
		},
		RowGap:   layout.Px(8),
		FlexGrow: 1,
	})
	tree.AddChild(tableBox, tableGrid)

	var thIDs []layout.NodeID
	for i := 0; i < 8; i++ {
		th := tree.NewNode(layout.Style{Height: layout.Px(28)})
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

	// 4. Compute Layout
	startLayout := time.Now()
	tree.Compute(canvasW, canvasH)
	layoutDur := time.Since(startLayout)

	// 5. Render Scene onto Raster Canvas
	startRender := time.Now()
	canvas := raster.NewCanvas(canvasW, canvasH)
	canvas.Clear(plume.Hex("#0b0f19"))

	// Subtle radial/gradient background glow effect
	glowPath := geom.Circle(canvasW/2, -100, 450)
	canvas.DrawPath(glowPath, plume.Fill(plume.Color{R: 20, G: 35, B: 65, A: 70}))

	// Header rendering
	hRect := tree.ComputedRect(header)
	logoBadge := geom.Circle(hRect.X+18, hRect.Y+hRect.Height/2, 16)
	canvas.DrawPath(logoBadge, plume.Fill(plume.Hex("#38bdf8")))

	// Stylized vector "P" feather in logo badge
	var pb geom.PathBuilder
	pb.MoveTo(hRect.X+13, hRect.Y+hRect.Height/2+7)
	pb.LineTo(hRect.X+13, hRect.Y+hRect.Height/2-7)
	pb.CubicTo(hRect.X+19, hRect.Y+hRect.Height/2-7, hRect.X+24, hRect.Y+hRect.Height/2-3, hRect.X+17, hRect.Y+hRect.Height/2)
	pFeather := geom.Stroke(pb.Finish(), geom.StrokeOptions{Width: 2.5})
	canvas.DrawPath(pFeather, plume.Fill(plume.White))

	text.DrawText(canvas, cache, font, "PLUME ENGINE", hRect.X+44, hRect.Y+32, 20, plume.White)

	// Status badge pill in header
	badgeRect := raster.Rect{X: hRect.X + hRect.Width - 170, Y: hRect.Y + 10, Width: 170, Height: 30}
	canvas.DrawRoundedRect(badgeRect, 15, plume.Fill(plume.Hex("#064e3b")))
	statusDot := geom.Circle(badgeRect.X+18, badgeRect.Y+15, 4)
	canvas.DrawPath(statusDot, plume.Fill(plume.Hex("#34d399")))
	text.DrawText(canvas, cache, font, "100% W3C CONFORMANT", badgeRect.X+30, badgeRect.Y+20, 10, plume.Hex("#a7f3d0"))

	// Metrics Cards
	for i, m := range metrics {
		mRect := tree.ComputedRect(metricNodeIDs[i])
		canvas.DrawRoundedRect(mRect, 12, plume.Fill(plume.Hex("#111827")))

		// Accent border bar on left
		bar := raster.Rect{X: mRect.X, Y: mRect.Y, Width: 4, Height: mRect.Height}
		canvas.DrawRoundedRect(bar, 2, plume.Fill(m.accent))

		text.DrawText(canvas, cache, font, m.title, mRect.X+18, mRect.Y+28, 11, plume.Hex("#9ca3af"))
		text.DrawText(canvas, cache, font, m.value, mRect.X+18, mRect.Y+68, 24, plume.White)
		text.DrawText(canvas, cache, font, m.change, mRect.X+18, mRect.Y+94, 12, m.accent)

		// Smooth vector sparkline curve
		if len(m.spark) > 1 {
			sparkW := float32(75)
			sparkH := float32(36)
			sparkX := mRect.X + mRect.Width - sparkW - 16
			sparkY := mRect.Y + mRect.Height - sparkH - 24

			var spb geom.PathBuilder
			step := sparkW / float32(len(m.spark)-1)
			for j, val := range m.spark {
				vx := sparkX + float32(j)*step
				vy := sparkY + sparkH - (val/50.0)*sparkH
				if j == 0 {
					spb.MoveTo(vx, vy)
				} else {
					spb.LineTo(vx, vy)
				}
			}
			sparkPath := geom.Stroke(spb.Finish(), geom.StrokeOptions{Width: 2.0})
			canvas.DrawPath(sparkPath, plume.Fill(m.accent))
		}
	}

	// Table Container Background
	tRect := tree.ComputedRect(tableBox)
	canvas.DrawRoundedRect(tRect, 14, plume.Fill(plume.Hex("#111827")))

	// Table Headers
	headers := []string{"SERVICE", "REGION", "STATUS", "P50", "P99", "ERRORS", "UPTIME (30D)", "DEPLOYED"}
	for i, thID := range thIDs {
		rect := tree.ComputedRect(thID)
		text.DrawText(canvas, cache, font, headers[i], rect.X+8, rect.Y+20, 11, plume.Hex("#6b7280"))
	}

	// Table Rows
	for r, s := range services {
		cols := rowIDs[r]

		// Alternating subtle zebra striping
		if r%2 == 1 {
			rStart := tree.ComputedRect(cols[0])
			rEnd := tree.ComputedRect(cols[7])
			rowBg := raster.Rect{
				X:      rStart.X - 4,
				Y:      rStart.Y - 2,
				Width:  (rEnd.X + rEnd.Width) - rStart.X + 8,
				Height: rStart.Height + 4,
			}
			canvas.DrawRoundedRect(rowBg, 6, plume.Fill(plume.Hex("#162032")))
		}

		// Service Name
		r0 := tree.ComputedRect(cols[0])
		text.DrawText(canvas, cache, font, s.name, r0.X+8, r0.Y+24, 13, plume.Hex("#f9fafb"))

		// Region
		r1 := tree.ComputedRect(cols[1])
		text.DrawText(canvas, cache, font, s.region, r1.X+8, r1.Y+24, 12, plume.Hex("#9ca3af"))

		// Status Badge
		r2 := tree.ComputedRect(cols[2])
		statusPill := raster.Rect{X: r2.X + 8, Y: r2.Y + 8, Width: 76, Height: 22}
		canvas.DrawRoundedRect(statusPill, 4, plume.Fill(plume.Hex("#064e3b")))
		text.DrawText(canvas, cache, font, s.status, statusPill.X+10, statusPill.Y+15, 10, plume.Hex("#34d399"))

		// Latency & Errors
		r3 := tree.ComputedRect(cols[3])
		text.DrawText(canvas, cache, font, s.p50, r3.X+8, r3.Y+24, 12, plume.Hex("#e5e7eb"))

		r4 := tree.ComputedRect(cols[4])
		text.DrawText(canvas, cache, font, s.p99, r4.X+8, r4.Y+24, 12, plume.Hex("#e5e7eb"))

		r5 := tree.ComputedRect(cols[5])
		text.DrawText(canvas, cache, font, s.errorRt, r5.X+8, r5.Y+24, 12, plume.Hex("#9ca3af"))

		// Uptime progress indicator bar
		r6 := tree.ComputedRect(cols[6])
		barW := r6.Width - 28
		bgBar := raster.Rect{X: r6.X + 8, Y: r6.Y + 14, Width: barW, Height: 8}
		canvas.DrawRoundedRect(bgBar, 4, plume.Fill(plume.Hex("#1f2937")))

		fgBar := raster.Rect{X: r6.X + 8, Y: r6.Y + 14, Width: barW * s.uptime, Height: 8}
		barCol := plume.Hex("#10b981")
		if s.uptime < 0.98 {
			barCol = plume.Hex("#f59e0b")
		}
		canvas.DrawRoundedRect(fgBar, 4, plume.Fill(barCol))

		// Deployment timestamp
		r7 := tree.ComputedRect(cols[7])
		text.DrawText(canvas, cache, font, s.deploy, r7.X+8, r7.Y+24, 12, plume.Hex("#6b7280"))
	}

	renderDur := time.Since(startRender)

	// 6. Save PNG
	outPath := "output.png"
	if err := plume.SavePNG(canvas.ToRGBA(), outPath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save output PNG: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Render complete in %v (Layout: %v, Raster: %v)\n", layoutDur+renderDur, layoutDur, renderDur)
	fmt.Printf("Successfully generated high-resolution PNG: %s (%dx%d px)\n", outPath, canvasW, canvasH)
}
