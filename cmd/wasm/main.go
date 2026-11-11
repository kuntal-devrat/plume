//go:build js && wasm

package main

import (
	"encoding/json"
	"fmt"
	"syscall/js"
	"time"

	"github.com/plume-engine/plume"
	"github.com/plume-engine/plume/geom"
	"github.com/plume-engine/plume/layout"
	"github.com/plume-engine/plume/raster"
)

// UIConfig represents the live interactive parameters sent from JavaScript.
type UIConfig struct {
	Width        int     `json:"width"`
	Height       int     `json:"height"`
	Direction    string  `json:"direction"`    // "row" or "column"
	Justify      string  `json:"justify"`      // "start", "center", "end", "space-between", "space-around"
	Align        string  `json:"align"`        // "stretch", "start", "center", "end"
	Gap          float32 `json:"gap"`          // gap in px
	CardCount    int     `json:"cardCount"`    // number of cards
	CornerRadius float32 `json:"cornerRadius"` // card border radius
	Theme        string  `json:"theme"`        // "dark", "light", "blue"
}

func main() {
	fmt.Println("🪶 Plume WebAssembly Engine initialized")

	js.Global().Set("plumeRender", js.FuncOf(plumeRender))
	js.Global().Set("plumeBenchmark", js.FuncOf(plumeBenchmark))

	// Keep WebAssembly runtime alive
	select {}
}

func plumeRender(this js.Value, args []js.Value) any {
	if len(args) < 2 {
		return "Error: expected (configJSON, canvasElement)"
	}

	configJSON := args[0].String()
	canvasEl := args[1]

	var cfg UIConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Sprintf("Error parsing JSON: %v", err)
	}

	if cfg.Width <= 0 {
		cfg.Width = 800
	}
	if cfg.Height <= 0 {
		cfg.Height = 500
	}
	if cfg.CardCount <= 0 {
		cfg.CardCount = 4
	}

	canvas := renderPlumeScene(cfg)

	// Copy pixels into HTML5 canvas 2D context
	ctx := canvasEl.Call("getContext", "2d")
	imgData := ctx.Call("createImageData", cfg.Width, cfg.Height)
	dataArr := imgData.Get("data")

	// Raw pixel copy
	rawPix := canvas.Pixels()
	js.CopyBytesToJS(dataArr, rawPix)
	ctx.Call("putImageData", imgData, 0, 0)

	return "OK"
}

func plumeBenchmark(this js.Value, args []js.Value) any {
	cfg := UIConfig{
		Width:        800,
		Height:       600,
		Direction:    "row",
		Justify:      "space-between",
		Align:        "center",
		Gap:          16,
		CardCount:    6,
		CornerRadius: 12,
		Theme:        "dark",
	}

	const frames = 100
	start := time.Now()
	for i := 0; i < frames; i++ {
		_ = renderPlumeScene(cfg)
	}
	elapsed := time.Since(start)

	fps := float64(frames) / elapsed.Seconds()
	msPerFrame := elapsed.Seconds() * 1000 / float64(frames)

	res := map[string]any{
		"fps":        fps,
		"msPerFrame": msPerFrame,
		"frames":     frames,
		"totalMs":    elapsed.Milliseconds(),
	}

	b, _ := json.Marshal(res)
	return string(b)
}

func renderPlumeScene(cfg UIConfig) *raster.Canvas {
	w, h := cfg.Width, cfg.Height

	tree := layout.NewTree()

	// Direction
	dir := layout.FlexDirectionRow
	if cfg.Direction == "column" {
		dir = layout.FlexDirectionColumn
	}

	// Justify
	justify := layout.JustifySpaceBetween
	switch cfg.Justify {
	case "start":
		justify = layout.JustifyFlexStart
	case "center":
		justify = layout.JustifyCenter
	case "end":
		justify = layout.JustifyFlexEnd
	case "space-around":
		justify = layout.JustifySpaceAround
	}

	// Align
	align := layout.AlignCenter
	switch cfg.Align {
	case "stretch":
		align = layout.AlignStretch
	case "start":
		align = layout.AlignFlexStart
	case "end":
		align = layout.AlignFlexEnd
	}

	root := tree.NewNode(layout.Style{
		Display:        layout.DisplayFlex,
		FlexDirection:  dir,
		JustifyContent: justify,
		AlignItems:     align,
		Width:          layout.Px(float32(w)),
		Height:         layout.Px(float32(h)),
		Padding:        layout.Uniform(24),
		Gap:            layout.Px(cfg.Gap),
	})

	var cardIDs []layout.NodeID
	for i := 0; i < cfg.CardCount; i++ {
		card := tree.NewNode(layout.Style{
			FlexGrow: 1,
			Height:   layout.Px(float32(h) * 0.45),
			MinWidth: layout.Px(60),
		})
		tree.AddChild(root, card)
		cardIDs = append(cardIDs, card)
	}

	tree.Compute(float32(w), float32(h))

	canvas := raster.NewCanvas(w, h)

	// Theme colors
	bgCol := plume.Hex("#0b0f19")
	cardCol := plume.Hex("#1e293b")
	if cfg.Theme == "light" {
		bgCol = plume.Hex("#f8fafc")
		cardCol = plume.Hex("#e2e8f0")
	}

	canvas.Clear(bgCol)

	palette := []plume.Color{
		plume.Hex("#38bdf8"),
		plume.Hex("#34d399"),
		plume.Hex("#a78bfa"),
		plume.Hex("#fbbf24"),
		plume.Hex("#f43f5e"),
		plume.Hex("#818cf8"),
	}

	for i, cID := range cardIDs {
		rect := tree.ComputedRect(cID)
		canvas.DrawRoundedRect(rect, cfg.CornerRadius, plume.Fill(cardCol))

		// Header accent bar
		bar := raster.Rect{
			X:      rect.X,
			Y:      rect.Y,
			Width:  rect.Width,
			Height: 8,
		}
		canvas.DrawRoundedRect(bar, 4, plume.Fill(palette[i%len(palette)]))

		// Inner vector circle
		circ := geom.Circle(rect.X+rect.Width*0.5, rect.Y+rect.Height*0.5, 16)
		canvas.DrawPath(circ, plume.Fill(palette[i%len(palette)]))
	}

	return canvas
}
