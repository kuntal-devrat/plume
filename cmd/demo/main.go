package main

import (
	"fmt"
	"os"

	"github.com/plume-engine/plume"
	"github.com/plume-engine/plume/geom"
	"github.com/plume-engine/plume/layout"
	"github.com/plume-engine/plume/raster"
)

func main() {
	const (
		canvasW = 800
		canvasH = 600
	)

	fmt.Println("🚀 Plume Engine Demo (Phase 1: Pure Go Zero-Cgo Layout & Vector Rasterizer)")

	// 1. Create layout tree
	tree := layout.NewTree()

	// Root container: vertical flex column with padding
	root := tree.NewNode(layout.Style{
		Display:        layout.DisplayFlex,
		FlexDirection:  layout.FlexDirectionColumn,
		JustifyContent: layout.JustifySpaceBetween,
		Width:          layout.Px(canvasW),
		Height:         layout.Px(canvasH),
		Padding:        layout.Uniform(32),
		Gap:            layout.Px(20),
	})

	// Header bar: horizontal flex row
	header := tree.NewNode(layout.Style{
		Display:        layout.DisplayFlex,
		FlexDirection:  layout.FlexDirectionRow,
		JustifyContent: layout.JustifySpaceBetween,
		AlignItems:     layout.AlignCenter,
		Height:         layout.Px(64),
	})
	tree.AddChild(root, header)

	// Main content area: horizontal flex row with 3 feature cards
	cardsRow := tree.NewNode(layout.Style{
		Display:       layout.DisplayFlex,
		FlexDirection: layout.FlexDirectionRow,
		FlexGrow:      1,
		Gap:           layout.Px(20),
	})
	tree.AddChild(root, cardsRow)

	card1 := tree.NewNode(layout.Style{
		Display:  layout.DisplayFlex,
		FlexGrow: 1,
	})
	card2 := tree.NewNode(layout.Style{
		Display:  layout.DisplayFlex,
		FlexGrow: 1,
	})
	card3 := tree.NewNode(layout.Style{
		Display:  layout.DisplayFlex,
		FlexGrow: 1,
	})
	tree.AddChild(cardsRow, card1)
	tree.AddChild(cardsRow, card2)
	tree.AddChild(cardsRow, card3)

	// Footer bar: horizontal flex row
	footer := tree.NewNode(layout.Style{
		Display:        layout.DisplayFlex,
		FlexDirection:  layout.FlexDirectionRow,
		JustifyContent: layout.JustifySpaceBetween,
		AlignItems:     layout.AlignCenter,
		Height:         layout.Px(48),
	})
	tree.AddChild(root, footer)

	// 2. Compute Layout (zero-allocations in hot loop)
	tree.Compute(canvasW, canvasH)
	fmt.Println(" Layout computed successfully")

	// 3. Render directly onto 2D Canvas
	canvas := raster.NewCanvas(canvasW, canvasH)

	// Background: dark slate #0f172a
	canvas.Clear(plume.Hex("#0f172a"))

	// Header background: subtle rounded bar #1e293b
	headerBounds := tree.ComputedRect(header)
	canvas.DrawRoundedRect(headerBounds, 12, plume.Fill(plume.Hex("#1e293b")))

	// Header logo accent (drawn as smooth vector circle)
	logoCircle := geom.Circle(headerBounds.X+24, headerBounds.Y+headerBounds.Height/2, 14)
	canvas.DrawPath(logoCircle, plume.Fill(plume.Hex("#38bdf8")))

	// Header text placeholder
	canvas.DrawText("Plume Engine", headerBounds.X+50, headerBounds.Y+24, plume.TextStyle{
		FontSize: 16,
		Color:    plume.White,
	})

	// Render Cards
	cardColors := []plume.Color{
		plume.Hex("#1e293b"), // Slate card 1
		plume.Hex("#1e293b"), // Slate card 2
		plume.Hex("#1e293b"), // Slate card 3
	}
	cardAccents := []plume.Color{
		plume.Hex("#3b82f6"), // Blue accent
		plume.Hex("#10b981"), // Emerald accent
		plume.Hex("#f59e0b"), // Amber accent
	}
	cardNodes := []layout.NodeID{card1, card2, card3}

	for i, cNode := range cardNodes {
		bounds := tree.ComputedRect(cNode)

		// Draw card body with rounded corners
		canvas.DrawRoundedRect(bounds, 16, plume.Fill(cardColors[i]))

		// Top accent badge inside card
		badgeRect := raster.Rect{
			X:      bounds.X + 20,
			Y:      bounds.Y + 20,
			Width:  bounds.Width - 40,
			Height: 36,
		}
		canvas.DrawRoundedRect(badgeRect, 8, plume.Fill(cardAccents[i]))

		// Inner vector decoration: small circle indicator
		indCircle := geom.Circle(bounds.X+36, bounds.Y+bounds.Height-36, 10)
		canvas.DrawPath(indCircle, plume.Fill(cardAccents[i]))

		// Card content text
		canvas.DrawText(fmt.Sprintf("Card #%d", i+1), bounds.X+20, bounds.Y+76, plume.TextStyle{
			FontSize: 18,
			Color:    plume.White,
		})
	}

	// Footer bar
	footerBounds := tree.ComputedRect(footer)
	canvas.DrawRoundedRect(footerBounds, 8, plume.Fill(plume.Hex("#1e293b")))
	canvas.DrawText("Pure Go Zero-Cgo Layout & Vector Engine", footerBounds.X+16, footerBounds.Y+16, plume.TextStyle{
		FontSize: 14,
		Color:    plume.Hex("#94a3b8"),
	})

	// 4. Save to standard Go image
	img := canvas.ToRGBA()
	outPath := "output.png"
	if err := plume.SavePNG(img, outPath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save PNG: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf(" Saved rendered canvas to %s (%dx%d px)\n", outPath, canvasW, canvasH)
}
