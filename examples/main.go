package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kuntal-devrat/plume"
	"github.com/kuntal-devrat/plume/geom"
	"github.com/kuntal-devrat/plume/layout"
	"github.com/kuntal-devrat/plume/raster"
	"github.com/kuntal-devrat/plume/text"
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
		Gap:           layout.Px(20),
	})

	// Header bar: Flex row with space-between
	header := tree.NewNode(layout.Style{
		Display:        layout.DisplayFlex,
		FlexDirection:  layout.FlexDirectionRow,
		JustifyContent: layout.JustifySpaceBetween,
		AlignItems:     layout.AlignCenter,
		Height:         layout.Px(48),
	})
	tree.AddChild(root, header)

	// Metrics row: CSS Grid with 4 equal fr tracks
	metricsGrid := tree.NewNode(layout.Style{
		Display: layout.DisplayGrid,
		GridTemplateColumns: []layout.Dimension{
			layout.Fr(1), layout.Fr(1), layout.Fr(1), layout.Fr(1),
		},
		ColumnGap: layout.Px(18),
		Height:    layout.Px(112),
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
		Padding:       layout.Uniform(24),
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
		th := tree.NewNode(layout.Style{Height: layout.Px(30)})
		tree.AddChild(tableGrid, th)
		thIDs = append(thIDs, th)
	}

	var rowIDs [][]layout.NodeID
	for r := 0; r < len(services); r++ {
		var rCols []layout.NodeID
		for c := 0; c < 8; c++ {
			td := tree.NewNode(layout.Style{Height: layout.Px(40)})
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
	canvas.Clear(plume.Hex("#080c14"))

	// Header rendering
	hRect := tree.AbsoluteRect(header)

	// Logo badge with geometric feather icon
	logoBadge := geom.Circle(hRect.X+18, hRect.Y+hRect.Height/2, 16)
	canvas.DrawPath(logoBadge, plume.Fill(plume.Hex("#0284c7")))

	var pb geom.PathBuilder
	pb.MoveTo(hRect.X+12, hRect.Y+hRect.Height/2+6)
	pb.LineTo(hRect.X+12, hRect.Y+hRect.Height/2-6)
	pb.CubicTo(hRect.X+18, hRect.Y+hRect.Height/2-6, hRect.X+24, hRect.Y+hRect.Height/2-2, hRect.X+17, hRect.Y+hRect.Height/2+1)
	pFeather := geom.Stroke(pb.Finish(), geom.StrokeOptions{Width: 2.2})
	canvas.DrawPath(pFeather, plume.Fill(plume.White))

	text.DrawText(canvas, cache, font, "PLUME ENGINE", hRect.X+46, hRect.Y+30, 18, plume.White)
	text.DrawText(canvas, cache, font, "SYSTEM OVERVIEW", hRect.X+210, hRect.Y+30, 13, plume.Hex("#64748b"))

	// Status badge pill in header
	badgeRect := raster.Rect{X: hRect.X + hRect.Width - 180, Y: hRect.Y + 9, Width: 180, Height: 30}
	canvas.DrawRoundedRect(badgeRect, 15, plume.Fill(plume.Hex("#064e3b")))
	statusDot := geom.Circle(badgeRect.X+16, badgeRect.Y+15, 4)
	canvas.DrawPath(statusDot, plume.Fill(plume.Hex("#34d399")))
	text.DrawText(canvas, cache, font, "100% W3C CONFORMANT", badgeRect.X+28, badgeRect.Y+20, 11, plume.Hex("#a7f3d0"))

	// Metrics Cards
	for i, m := range metrics {
		mRect := tree.AbsoluteRect(metricNodeIDs[i])
		canvas.DrawRoundedRect(mRect, 12, plume.Fill(plume.Hex("#0f172a")))

		// Accent border bar on left
		bar := raster.Rect{X: mRect.X, Y: mRect.Y, Width: 4, Height: mRect.Height}
		canvas.DrawRoundedRect(bar, 2, plume.Fill(m.accent))

		text.DrawText(canvas, cache, font, m.title, mRect.X+18, mRect.Y+28, 11, plume.Hex("#94a3b8"))
		text.DrawText(canvas, cache, font, m.value, mRect.X+18, mRect.Y+68, 24, plume.White)
		text.DrawText(canvas, cache, font, m.change, mRect.X+18, mRect.Y+95, 12, m.accent)

		// Smooth vector sparkline curve
		if len(m.spark) > 1 {
			sparkW := float32(76)
			sparkH := float32(34)
			sparkX := mRect.X + mRect.Width - sparkW - 18
			sparkY := mRect.Y + mRect.Height - sparkH - 26

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
	tRect := tree.AbsoluteRect(tableBox)
	canvas.DrawRoundedRect(tRect, 14, plume.Fill(plume.Hex("#0f172a")))

	// Table Headers
	headers := []string{"SERVICE", "REGION", "STATUS", "P50", "P99", "ERRORS", "UPTIME (30D)", "DEPLOYED"}
	for i, thID := range thIDs {
		rect := tree.AbsoluteRect(thID)
		text.DrawText(canvas, cache, font, headers[i], rect.X+10, rect.Y+20, 11, plume.Hex("#64748b"))
	}

	// Table Rows
	for r, s := range services {
		cols := rowIDs[r]

		// Alternating subtle zebra striping
		if r%2 == 1 {
			rStart := tree.AbsoluteRect(cols[0])
			rEnd := tree.AbsoluteRect(cols[7])
			rowBg := raster.Rect{
				X:      rStart.X - 4,
				Y:      rStart.Y - 2,
				Width:  (rEnd.X + rEnd.Width) - rStart.X + 8,
				Height: rStart.Height + 4,
			}
			canvas.DrawRoundedRect(rowBg, 6, plume.Fill(plume.Hex("#172033")))
		}

		// Service Name
		r0 := tree.AbsoluteRect(cols[0])
		text.DrawText(canvas, cache, font, s.name, r0.X+10, r0.Y+25, 13, plume.Hex("#f1f5f9"))

		// Region
		r1 := tree.AbsoluteRect(cols[1])
		text.DrawText(canvas, cache, font, s.region, r1.X+10, r1.Y+25, 12, plume.Hex("#94a3b8"))

		// Status Badge
		r2 := tree.AbsoluteRect(cols[2])
		statusPill := raster.Rect{X: r2.X + 8, Y: r2.Y + 8, Width: 76, Height: 22}
		canvas.DrawRoundedRect(statusPill, 4, plume.Fill(plume.Hex("#064e3b")))
		text.DrawText(canvas, cache, font, s.status, statusPill.X+10, statusPill.Y+15, 10, plume.Hex("#34d399"))

		// Latency & Errors
		r3 := tree.AbsoluteRect(cols[3])
		text.DrawText(canvas, cache, font, s.p50, r3.X+10, r3.Y+25, 12, plume.Hex("#e2e8f0"))

		r4 := tree.AbsoluteRect(cols[4])
		text.DrawText(canvas, cache, font, s.p99, r4.X+10, r4.Y+25, 12, plume.Hex("#e2e8f0"))

		r5 := tree.AbsoluteRect(cols[5])
		text.DrawText(canvas, cache, font, s.errorRt, r5.X+10, r5.Y+25, 12, plume.Hex("#94a3b8"))

		// Uptime progress indicator bar
		r6 := tree.AbsoluteRect(cols[6])
		barW := r6.Width - 28
		bgBar := raster.Rect{X: r6.X + 10, Y: r6.Y + 16, Width: barW, Height: 8}
		canvas.DrawRoundedRect(bgBar, 4, plume.Fill(plume.Hex("#1e293b")))

		fgBar := raster.Rect{X: r6.X + 10, Y: r6.Y + 16, Width: barW * s.uptime, Height: 8}
		barCol := plume.Hex("#10b981")
		if s.uptime < 0.98 {
			barCol = plume.Hex("#f59e0b")
		}
		canvas.DrawRoundedRect(fgBar, 4, plume.Fill(barCol))

		// Deployment timestamp
		r7 := tree.AbsoluteRect(cols[7])
		text.DrawText(canvas, cache, font, s.deploy, r7.X+10, r7.Y+25, 12, plume.Hex("#64748b"))
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
