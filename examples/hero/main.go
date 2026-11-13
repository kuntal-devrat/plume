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

// helper to draw horizontally and vertically centered text inside a box
func drawCenteredText(canvas *raster.Canvas, cache *text.GlyphCache, font *text.Font, str string, r layout.Rect, fontSize float32, col plume.Color) {
	shaped := text.ShapeText(font, str, fontSize, 0)
	x := r.X + (r.Width-shaped.TotalWidth)/2
	y := r.Y + (r.Height-fontSize)/2 + fontSize*0.82
	text.DrawText(canvas, cache, font, str, x, y, fontSize, col)
}

func main() {
	const (
		canvasW = 1440
		canvasH = 960
	)

	fmt.Printf("Rendering Plume SaaS Hero Page (%dx%d px)...\n", canvasW, canvasH)

	fontPath := findFontPath()
	font, err := text.LoadFontFile(fontPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading font %s: %v\n", fontPath, err)
		os.Exit(1)
	}
	cache := text.NewGlyphCache(8192)

	// -------------------------------------------------------------------------
	// 1. Layout Tree Construction
	// -------------------------------------------------------------------------
	tree := layout.NewTree()

	// Root Page Container (Flex Column)
	root := tree.NewNode(layout.Style{
		Display:       layout.DisplayFlex,
		FlexDirection: layout.FlexDirectionColumn,
		Width:         layout.Px(canvasW),
		Height:        layout.Px(canvasH),
		Padding:       layout.EdgesPx(0, 64, 36, 64),
		Gap:           layout.Px(24),
	})

	// --- NAV BAR ---
	navBar := tree.NewNode(layout.Style{
		Display:        layout.DisplayFlex,
		FlexDirection:  layout.FlexDirectionRow,
		JustifyContent: layout.JustifySpaceBetween,
		AlignItems:     layout.AlignCenter,
		Width:          layout.Percent(100),
		Height:         layout.Px(64),
	})
	tree.AddChild(root, navBar)

	navBrand := tree.NewNode(layout.Style{
		Display:       layout.DisplayFlex,
		FlexDirection: layout.FlexDirectionRow,
		AlignItems:    layout.AlignCenter,
		Gap:           layout.Px(12),
		Height:        layout.Px(40),
	})
	tree.AddChild(navBar, navBrand)

	navLinks := tree.NewNode(layout.Style{
		Display:       layout.DisplayFlex,
		FlexDirection: layout.FlexDirectionRow,
		AlignItems:    layout.AlignCenter,
		Gap:           layout.Px(32),
		Height:        layout.Px(40),
	})
	tree.AddChild(navBar, navLinks)

	navActions := tree.NewNode(layout.Style{
		Display:        layout.DisplayFlex,
		FlexDirection:  layout.FlexDirectionRow,
		AlignItems:     layout.AlignCenter,
		JustifyContent: layout.JustifyFlexEnd,
		Gap:            layout.Px(20),
		Height:         layout.Px(40),
		Width:          layout.Px(210),
	})
	tree.AddChild(navBar, navActions)

	// --- HERO HEADER SECTION ---
	heroSection := tree.NewNode(layout.Style{
		Display:        layout.DisplayFlex,
		FlexDirection:  layout.FlexDirectionColumn,
		AlignItems:     layout.AlignCenter,
		JustifyContent: layout.JustifyCenter,
		Gap:            layout.Px(14),
		Height:         layout.Px(210),
	})
	tree.AddChild(root, heroSection)

	announcementPill := tree.NewNode(layout.Style{
		Height: layout.Px(32),
		Width:  layout.Px(370),
	})
	tree.AddChild(heroSection, announcementPill)

	headlineBox := tree.NewNode(layout.Style{
		Height: layout.Px(52),
		Width:  layout.Px(1100),
	})
	tree.AddChild(heroSection, headlineBox)

	subtitleBox := tree.NewNode(layout.Style{
		Height: layout.Px(28),
		Width:  layout.Px(860),
	})
	tree.AddChild(heroSection, subtitleBox)

	ctaRow := tree.NewNode(layout.Style{
		Display:       layout.DisplayFlex,
		FlexDirection: layout.FlexDirectionRow,
		AlignItems:    layout.AlignCenter,
		Gap:           layout.Px(16),
		Height:        layout.Px(46),
	})
	tree.AddChild(heroSection, ctaRow)

	primaryCta := tree.NewNode(layout.Style{
		Width:  layout.Px(170),
		Height: layout.Px(42),
	})
	tree.AddChild(ctaRow, primaryCta)

	secondaryCta := tree.NewNode(layout.Style{
		Width:  layout.Px(150),
		Height: layout.Px(42),
	})
	tree.AddChild(ctaRow, secondaryCta)

	// --- HERO MOCKUP WINDOW (The interactive app preview) ---
	mockupWindow := tree.NewNode(layout.Style{
		Display:       layout.DisplayFlex,
		FlexDirection: layout.FlexDirectionColumn,
		Width:         layout.Percent(100),
		Height:        layout.Px(370),
	})
	tree.AddChild(root, mockupWindow)

	// Mockup Title Bar
	mockupTitlebar := tree.NewNode(layout.Style{
		Display:       layout.DisplayFlex,
		FlexDirection: layout.FlexDirectionRow,
		AlignItems:    layout.AlignCenter,
		Height:        layout.Px(38),
		Padding:       layout.EdgesPx(0, 16, 0, 16),
	})
	tree.AddChild(mockupWindow, mockupTitlebar)

	// Mockup Body (Split Pane Grid: 44% Code, 56% Live Render)
	mockupBody := tree.NewNode(layout.Style{
		Display: layout.DisplayGrid,
		GridTemplateColumns: []layout.Dimension{
			layout.Fr(1.0), layout.Fr(1.25),
		},
		ColumnGap: layout.Px(1),
		FlexGrow:  1,
	})
	tree.AddChild(mockupWindow, mockupBody)

	codePane := tree.NewNode(layout.Style{
		Padding:  layout.Uniform(18),
		FlexGrow: 1,
	})
	tree.AddChild(mockupBody, codePane)

	renderPane := tree.NewNode(layout.Style{
		Padding:  layout.Uniform(18),
		FlexGrow: 1,
	})
	tree.AddChild(mockupBody, renderPane)

	// --- 3 FEATURE CARDS (CSS Grid) ---
	featuresGrid := tree.NewNode(layout.Style{
		Display: layout.DisplayGrid,
		GridTemplateColumns: []layout.Dimension{
			layout.Fr(1), layout.Fr(1), layout.Fr(1),
		},
		ColumnGap: layout.Px(20),
		Height:    layout.Px(120),
	})
	tree.AddChild(root, featuresGrid)

	type featureCard struct {
		tag   string
		title string
		desc  string
		color plume.Color
	}
	features := []featureCard{
		{
			tag:   "100% W3C COMPLIANT",
			title: "Pixel-Identical CSS Specs",
			desc:  "Full Flexbox and CSS Grid algorithms passing 329/329 W3C Web Platform Tests.",
			color: plume.Hex("#38bdf8"),
		},
		{
			tag:   "ZERO CGO & HEAP OPTIMIZED",
			title: "Engineered for Raw Speed",
			desc:  "Flat node arena slices with 0 allocs in layout hot paths. Pure Go cross-compilation.",
			color: plume.Hex("#a78bfa"),
		},
		{
			tag:   "ANALYTICAL RASTERIZATION",
			title: "Subpixel Vector Quality",
			desc:  "Signed-area anti-aliasing, 22 Porter-Duff blend modes, and AVX2 SIMD blending.",
			color: plume.Hex("#34d399"),
		},
	}

	var featNodes []layout.NodeID
	for range features {
		fn := tree.NewNode(layout.Style{
			Display:       layout.DisplayFlex,
			FlexDirection: layout.FlexDirectionColumn,
			Padding:       layout.Uniform(16),
			Gap:           layout.Px(5),
		})
		tree.AddChild(featuresGrid, fn)
		featNodes = append(featNodes, fn)
	}

	// -------------------------------------------------------------------------
	// 2. Compute Layout
	// -------------------------------------------------------------------------
	startLayout := time.Now()
	tree.Compute(canvasW, canvasH)
	layoutDur := time.Since(startLayout)

	// -------------------------------------------------------------------------
	// 3. Render Canvas
	// -------------------------------------------------------------------------
	startRender := time.Now()
	canvas := raster.NewCanvas(canvasW, canvasH)

	// Midnight slate background
	canvas.Clear(plume.Hex("#07090e"))

	// Subtle, smooth ambient top glow (premultiplied alpha via ColorFromRGBA)
	topGlow := geom.Circle(canvasW/2, -180, 520)
	canvas.DrawPath(topGlow, plume.Fill(plume.ColorFromRGBA(30, 64, 175, 45)))

	// Ambient violet corner glow
	cornerGlow := geom.Circle(canvasW-100, canvasH+100, 450)
	canvas.DrawPath(cornerGlow, plume.Fill(plume.ColorFromRGBA(88, 28, 135, 30)))

	// --- DRAW NAV BAR ---
	brandRect := tree.AbsoluteRect(navBrand)
	brandDot := geom.Circle(brandRect.X+16, brandRect.Y+brandRect.Height/2, 14)
	canvas.DrawPath(brandDot, plume.Fill(plume.Hex("#0284c7")))

	// Stylized vector "P"
	var pb geom.PathBuilder
	pb.MoveTo(brandRect.X+11, brandRect.Y+brandRect.Height/2+5)
	pb.LineTo(brandRect.X+11, brandRect.Y+brandRect.Height/2-5)
	pb.CubicTo(brandRect.X+16, brandRect.Y+brandRect.Height/2-5, brandRect.X+21, brandRect.Y+brandRect.Height/2-2, brandRect.X+15, brandRect.Y+brandRect.Height/2+1)
	canvas.DrawPath(geom.Stroke(pb.Finish(), geom.StrokeOptions{Width: 2.0}), plume.Fill(plume.White))

	text.DrawText(canvas, cache, font, "Plume", brandRect.X+38, brandRect.Y+26, 18, plume.White)

	// Version Badge
	vPill := raster.Rect{X: brandRect.X + 96, Y: brandRect.Y + 9, Width: 46, Height: 20}
	canvas.DrawRoundedRect(vPill, 10, plume.Fill(plume.Hex("#1e293b")))
	text.DrawText(canvas, cache, font, "v1.0", vPill.X+10, vPill.Y+14, 10, plume.Hex("#38bdf8"))

	// Nav Links
	linksRect := tree.AbsoluteRect(navLinks)
	navItems := []string{"Platform", "Layout Specs", "Benchmarks", "Docs", "Showcase"}
	linkX := linksRect.X
	for _, item := range navItems {
		text.DrawText(canvas, cache, font, item, linkX, linksRect.Y+25, 13, plume.Hex("#94a3b8"))
		linkX += text.ShapeText(font, item, 13, 0).TotalWidth + 28
	}

	// Nav Actions (Sign In + Get Started)
	actRect := tree.AbsoluteRect(navActions)
	text.DrawText(canvas, cache, font, "Sign In", actRect.X+15, actRect.Y+25, 13, plume.Hex("#cbd5e1"))

	navBtn := raster.Rect{X: actRect.X + 85, Y: actRect.Y + 4, Width: 110, Height: 32}
	canvas.DrawRoundedRect(navBtn, 16, plume.Fill(plume.Hex("#2563eb")))
	drawCenteredText(canvas, cache, font, "Get Started", navBtn, 12, plume.White)

	// --- DRAW HERO SECTION ---
	// Announcement Pill
	pillRect := tree.AbsoluteRect(announcementPill)
	canvas.DrawRoundedRect(pillRect, 16, plume.Fill(plume.Hex("#0f172a")))
	canvas.DrawPath(geom.Stroke(geom.RoundedRect(pillRect.X, pillRect.Y, pillRect.Width, pillRect.Height, 16), geom.StrokeOptions{Width: 1.0}), plume.Fill(plume.Hex("#1e293b")))

	liveDot := geom.Circle(pillRect.X+18, pillRect.Y+16, 4)
	canvas.DrawPath(liveDot, plume.Fill(plume.Hex("#10b981")))
	text.DrawText(canvas, cache, font, "Plume 1.0 is live — 100% W3C Spec Conformance", pillRect.X+30, pillRect.Y+20, 11, plume.Hex("#e2e8f0"))

	// Headline
	headRect := tree.AbsoluteRect(headlineBox)
	drawCenteredText(canvas, cache, font, "Ultra-Fast 2D Layout & Vector Engine in Pure Go", headRect, 38, plume.White)

	// Subtitle
	subRect := tree.AbsoluteRect(subtitleBox)
	drawCenteredText(canvas, cache, font, "Zero Cgo. Zero external dependencies. Sub-millisecond CSS Flexbox & Grid algorithms.", subRect, 15, plume.Hex("#94a3b8"))

	// CTAs
	pCtaRect := tree.AbsoluteRect(primaryCta)
	canvas.DrawRoundedRect(pCtaRect, 21, plume.Fill(plume.Hex("#0284c7")))
	drawCenteredText(canvas, cache, font, "Start Building  →", pCtaRect, 13, plume.White)

	sCtaRect := tree.AbsoluteRect(secondaryCta)
	canvas.DrawRoundedRect(sCtaRect, 21, plume.Fill(plume.Hex("#0f172a")))
	canvas.DrawPath(geom.Stroke(geom.RoundedRect(sCtaRect.X, sCtaRect.Y, sCtaRect.Width, sCtaRect.Height, 21), geom.StrokeOptions{Width: 1.0}), plume.Fill(plume.Hex("#334155")))
	drawCenteredText(canvas, cache, font, "View on GitHub", sCtaRect, 13, plume.Hex("#cbd5e1"))

	// --- DRAW HERO MOCKUP WINDOW ---
	winRect := tree.AbsoluteRect(mockupWindow)

	// Window outer background & border
	canvas.DrawRoundedRect(winRect, 12, plume.Fill(plume.Hex("#0a0e17")))
	canvas.DrawPath(geom.Stroke(geom.RoundedRect(winRect.X, winRect.Y, winRect.Width, winRect.Height, 12), geom.StrokeOptions{Width: 1.0}), plume.Fill(plume.Hex("#1e293b")))

	// Titlebar
	tbRect := tree.AbsoluteRect(mockupTitlebar)
	// Window controls (Red, Yellow, Green)
	canvas.DrawPath(geom.Circle(tbRect.X+16, tbRect.Y+19, 5), plume.Fill(plume.Hex("#ef4444")))
	canvas.DrawPath(geom.Circle(tbRect.X+32, tbRect.Y+19, 5), plume.Fill(plume.Hex("#f59e0b")))
	canvas.DrawPath(geom.Circle(tbRect.X+48, tbRect.Y+19, 5), plume.Fill(plume.Hex("#10b981")))

	// Titlebar Tabs
	text.DrawText(canvas, cache, font, "main.go", tbRect.X+80, tbRect.Y+23, 12, plume.Hex("#38bdf8"))
	tabLine := raster.Rect{X: tbRect.X + 76, Y: tbRect.Y + 35, Width: 54, Height: 2}
	canvas.DrawRoundedRect(tabLine, 1, plume.Fill(plume.Hex("#38bdf8")))

	text.DrawText(canvas, cache, font, "layout_grid.go", tbRect.X+150, tbRect.Y+23, 12, plume.Hex("#64748b"))
	text.DrawText(canvas, cache, font, "rasterizer.go", tbRect.X+246, tbRect.Y+23, 12, plume.Hex("#64748b"))

	// Titlebar divider line
	divLine := raster.Rect{X: winRect.X, Y: tbRect.Y + tbRect.Height, Width: winRect.Width, Height: 1}
	canvas.DrawRect(divLine, plume.Fill(plume.Hex("#1e293b")))

	// Vertical pane divider
	codeRect := tree.AbsoluteRect(codePane)
	paneDiv := raster.Rect{X: codeRect.X + codeRect.Width, Y: tbRect.Y + tbRect.Height, Width: 1, Height: winRect.Height - tbRect.Height}
	canvas.DrawRect(paneDiv, plume.Fill(plume.Hex("#1e293b")))

	// Left Code Editor Pane content
	codeLines := []struct {
		num  string
		text string
		col  plume.Color
	}{
		{"1", "tree := layout.NewTree()", plume.Hex("#e2e8f0")},
		{"2", "root := tree.NewNode(layout.Style{", plume.Hex("#38bdf8")},
		{"3", "    Display:       layout.DisplayGrid,", plume.Hex("#a78bfa")},
		{"4", "    Columns:       []Dimension{Fr(1), Fr(2)},", plume.Hex("#f472b6")},
		{"5", "    Gap:           layout.Px(16),", plume.Hex("#cbd5e1")},
		{"6", "})", plume.Hex("#38bdf8")},
		{"7", "tree.Compute(viewportW, viewportH)", plume.Hex("#34d399")},
		{"8", "canvas := raster.NewCanvas(w, h)", plume.Hex("#e2e8f0")},
		{"9", "canvas.DrawRoundedRect(card, 12, fill)", plume.Hex("#38bdf8")},
		{"10", "// 0 allocations • < 60 µs execution", plume.Hex("#64748b")},
	}
	for i, cl := range codeLines {
		cy := codeRect.Y + 22 + float32(i)*24
		text.DrawText(canvas, cache, font, cl.num, codeRect.X+8, cy, 12, plume.Hex("#475569"))
		text.DrawText(canvas, cache, font, cl.text, codeRect.X+36, cy, 12, cl.col)
	}

	// Right Live Canvas Pane content
	rPaneRect := tree.AbsoluteRect(renderPane)

	// Inner preview card 1
	pCard1 := raster.Rect{X: rPaneRect.X + 16, Y: rPaneRect.Y + 14, Width: rPaneRect.Width - 32, Height: 116}
	canvas.DrawRoundedRect(pCard1, 10, plume.Fill(plume.Hex("#0f172a")))
	canvas.DrawPath(geom.Stroke(geom.RoundedRect(pCard1.X, pCard1.Y, pCard1.Width, pCard1.Height, 10), geom.StrokeOptions{Width: 1.0}), plume.Fill(plume.Hex("#1e293b")))

	text.DrawText(canvas, cache, font, "LIVE ENGINE METRICS", pCard1.X+16, pCard1.Y+26, 11, plume.Hex("#94a3b8"))
	text.DrawText(canvas, cache, font, "14.2 µs", pCard1.X+16, pCard1.Y+66, 28, plume.White)
	text.DrawText(canvas, cache, font, "Per-Frame Layout Latency", pCard1.X+16, pCard1.Y+92, 12, plume.Hex("#34d399"))

	// Sparkline in pCard1
	var spb geom.PathBuilder
	spW, spH := float32(110), float32(38)
	spX, spY := pCard1.X+pCard1.Width-spW-20, pCard1.Y+46
	sparkVals := []float32{12, 16, 14, 22, 19, 28, 35}
	for idx, val := range sparkVals {
		vx := spX + float32(idx)*(spW/6.0)
		vy := spY + spH - (val/40.0)*spH
		if idx == 0 {
			spb.MoveTo(vx, vy)
		} else {
			spb.LineTo(vx, vy)
		}
	}
	canvas.DrawPath(geom.Stroke(spb.Finish(), geom.StrokeOptions{Width: 2.2}), plume.Fill(plume.Hex("#38bdf8")))

	// Inner preview 2-column metrics
	pCard2 := raster.Rect{X: rPaneRect.X + 16, Y: rPaneRect.Y + 142, Width: (rPaneRect.Width - 44) / 2, Height: 96}
	canvas.DrawRoundedRect(pCard2, 8, plume.Fill(plume.Hex("#0f172a")))
	text.DrawText(canvas, cache, font, "W3C CONFORMANCE", pCard2.X+14, pCard2.Y+24, 10, plume.Hex("#64748b"))
	text.DrawText(canvas, cache, font, "100.0%", pCard2.X+14, pCard2.Y+58, 22, plume.Hex("#34d399"))
	text.DrawText(canvas, cache, font, "329/329 passing", pCard2.X+14, pCard2.Y+80, 11, plume.Hex("#94a3b8"))

	pCard3 := raster.Rect{X: rPaneRect.X + 24 + pCard2.Width, Y: rPaneRect.Y + 142, Width: pCard2.Width, Height: 96}
	canvas.DrawRoundedRect(pCard3, 8, plume.Fill(plume.Hex("#0f172a")))
	text.DrawText(canvas, cache, font, "HEAP ALLOCATIONS", pCard3.X+14, pCard3.Y+24, 10, plume.Hex("#64748b"))
	text.DrawText(canvas, cache, font, "0 B/op", pCard3.X+14, pCard3.Y+58, 22, plume.Hex("#38bdf8"))
	text.DrawText(canvas, cache, font, "Zero GC footprint", pCard3.X+14, pCard3.Y+80, 11, plume.Hex("#94a3b8"))

	// --- DRAW 3 FEATURE CARDS AT BOTTOM ---
	for i, feat := range features {
		fRect := tree.AbsoluteRect(featNodes[i])

		// Card container with subtle glass border
		canvas.DrawRoundedRect(fRect, 10, plume.Fill(plume.Hex("#0a0e17")))
		canvas.DrawPath(geom.Stroke(geom.RoundedRect(fRect.X, fRect.Y, fRect.Width, fRect.Height, 10), geom.StrokeOptions{Width: 1.0}), plume.Fill(plume.Hex("#1e293b")))

		// Accent top tag
		text.DrawText(canvas, cache, font, feat.tag, fRect.X+18, fRect.Y+26, 10, feat.color)
		// Title
		text.DrawText(canvas, cache, font, feat.title, fRect.X+18, fRect.Y+50, 15, plume.White)
		// Desc
		text.DrawText(canvas, cache, font, feat.desc, fRect.X+18, fRect.Y+74, 11, plume.Hex("#94a3b8"))
	}

	renderDur := time.Since(startRender)

	// Save PNG
	outDir := "assets"
	os.MkdirAll(outDir, 0755)
	outPath := filepath.Join(outDir, "hero.png")
	if err := plume.SavePNG(canvas.ToRGBA(), outPath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save hero PNG: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Hero render complete in %v (Layout: %v, Raster: %v)\n", layoutDur+renderDur, layoutDur, renderDur)
	fmt.Printf("Successfully generated SaaS Hero PNG: %s (%dx%d px)\n", outPath, canvasW, canvasH)
}
