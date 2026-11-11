# Plume

Plume is a 2D layout and vector rasterization library written in pure Go. It provides CSS Flexbox and Grid layout, analytical anti-aliased CPU rasterization, vector path geometry, TrueType font parsing, and adapters for game engines and native desktop apps without Cgo dependencies.

## Features

- **Pure Go**: Builds with standard `go build` (`CGO_ENABLED=0`) across Linux, macOS, Windows, and WebAssembly.
- **Layout Engine (`plume/layout`)**:
  - CSS Flexbox (direction, wrap, grow, shrink, basis, alignment, gap, padding, margin, min/max bounds).
  - CSS Grid (flexible `fr` tracks, fixed pixel and percentage tracks, line placement, spans, and auto-placement).
  - Positioning: `PositionRelative` and `PositionAbsolute` with directional insets (`Top`, `Right`, `Bottom`, `Left`) and dimension auto-stretching.
  - Alignment: Baseline alignment (`AlignBaseline`, `AlignSelfBaseline`) with line ascent calculation.
  - Intrinsic sizing: Leaf node measurement callbacks (`MeasureFunc`, `MeasureMode`).
  - Arena-backed tree with integer indices (`NodeID uint32`). Layout passes reuse pre-allocated scratch slices without heap allocations.
- **Vector Geometry (`plume/geom`)**:
  - Path builder for lines, quadratic Béziers, cubic Béziers, and arcs.
  - De Casteljau curve flattening, bounding box calculation, path stroking with caps and joins, dash patterns, and 2D affine transforms.
- **Software Rasterizer (`plume/raster`)**:
  - Analytical signed-area anti-aliasing inspired by FreeType and tiny-skia.
  - Clipping stack (`PushClipRect`, `PushClipPath`, `PopClip`) with state preservation (`Save`, `Restore`).
  - Full W3C Compositing & Blending: 22 blend modes (Porter-Duff + Multiply, Screen, Overlay, Darken, Lighten, ColorDodge, ColorBurn, HardLight, SoftLight, Difference, Exclusion).
  - Gamma-correct linear color space blending (IEC 61966-2-1 standard transfer curve).
  - Subpixel LCD anti-aliasing (`DrawSubpixelMask`) for 3-channel RGB subpixel stripe displays.
  - Active scanline and horizontal range tracking to eliminate redundant clears and sweeps.
  - AVX2 Go assembly routines on x86-64 for alpha blending with portable pure-Go fallbacks.
- **Font Engine (`plume/text`)**:
  - Pure-Go SFNT/TrueType binary parser (`cmap`, `glyf`, `loca`, `head`, `hmtx`, `hhea`, `kern`, `GSUB`).
  - OpenType `GSUB` table parsing with automatic multi-glyph ligature substitution (e.g. `fi`, `fl`).
  - Bidirectional text layout (Unicode UAX #9) with RTL script detection (Hebrew, Arabic) and right-to-left glyph positioning.
  - Multi-line text shaper with word wrapping, kerning, and an LRU glyph alpha mask cache.
- **Ecosystem Adapters**:
  - `adapter/ebiten`: Zero-copy surface bridge and pointer hit testing for Ebitengine games.
  - `adapter/wails`: Canvas serialization to base64 data URLs and raw buffers for desktop webviews.
  - `gpu`: Vector polygon tessellator supporting interior cutouts (holes) for generating GPU index/vertex buffers.
  - `playground`: In-browser WebAssembly interactive layout preview.

## Architecture

```
plume/
├── layout/        # CSS Flexbox and Grid layout (zero dependencies)
├── geom/          # 2D path geometry, flattening, stroking, transforms
├── raster/        # Analytical scanline rasterizer and Canvas drawing API
├── text/          # TrueType parser, glyph outline decoder, and text shaper
├── gpu/           # Vector polygon ear-clipping tessellator
├── adapter/
│   ├── ebiten/    # Ebitengine surface bridge and UI hit-testing
│   └── wails/     # Wails desktop canvas bridge
├── cmd/
│   ├── demo/      # Flexbox rendering verification demo
│   ├── dashboard/ # Complex CSS Grid dashboard benchmark
│   ├── oggen/     # Headless social card generator CLI
│   └── wasm/      # WebAssembly browser binary
└── playground/    # Static HTML + WebAssembly interactive editor
```

## Quick Start

```go
package main

import (
	"github.com/plume-engine/plume"
	"github.com/plume-engine/plume/layout"
	"github.com/plume-engine/plume/raster"
)

func main() {
	// 1. Build a layout tree
	tree := layout.NewTree()

	root := tree.NewNode(layout.Style{
		Display:        layout.DisplayFlex,
		FlexDirection:  layout.FlexDirectionColumn,
		JustifyContent: layout.JustifySpaceBetween,
		Width:          layout.Px(800),
		Height:         layout.Px(600),
		Padding:        layout.Uniform(24),
	})

	card := tree.NewNode(layout.Style{
		Display:       layout.DisplayFlex,
		FlexDirection: layout.FlexDirectionRow,
		AlignItems:    layout.AlignCenter,
		Gap:           layout.Px(16),
		Height:        layout.Px(80),
	})
	tree.AddChild(root, card)

	// 2. Compute Layout (zero allocations in hot loop)
	tree.Compute(800, 600)

	// 3. Render onto Canvas
	canvas := raster.NewCanvas(800, 600)
	canvas.Clear(plume.Hex("#0f172a"))

	bounds := tree.ComputedRect(card)
	canvas.DrawRoundedRect(bounds, 12, plume.Fill(plume.Hex("#1e293b")))

	// 4. Save PNG
	plume.SavePNG(canvas.ToRGBA(), "output.png")
}
```

## Current Limitations & Roadmap

- **Complex Text Shaping**: While OpenType `GSUB` ligatures (Type 4) and bidirectional right-to-left layout are implemented, `GPOS` mark-to-base and mark-to-mark positioning tables (required for complex Indic scripts like Devanagari or Bengali) and full Arabic contextual joining forms are not yet complete. For full HarfBuzz-equivalent shaping, an external shaper is required.
- **Color Fonts**: Colored glyph formats (`COLR`/`CPAL` and `CBDT`/`CBLC`) are not currently parsed.
- **Multi-Threading**: Rasterization runs synchronously on a single goroutine per canvas. Tile-based parallel sweeping across multiple worker goroutines is planned.
- **Production Readiness**: Plume is a new, pure-Go implementation. While it passes unit tests, race checks, and benchmarks, it has not yet undergone years of fuzz testing against malicious fonts or hostile SVG/HTML vectors in multi-tenant environments.

## License

Dual-licensed under MIT or Apache 2.0.
