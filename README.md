# Plume (Surfa) 🪶

**A zero-Cgo, mechanical-sympathy 2D Layout & Vector Rasterization Engine for Go.**

[![Go Reference](https://pkg.go.dev/badge/github.com/plume-engine/plume.svg)](https://pkg.go.dev/github.com/plume-engine/plume)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Zero Cgo](https://img.shields.io/badge/CGO_ENABLED-0-success.svg)](README.md)

---

## ⚡ Highlights

- **Strictly Zero Cgo (`CGO_ENABLED=0`)**: Clean cross-compilation to `linux/amd64`, `linux/arm64`, `darwin/arm64`, `windows/amd64`, and `js/wasm`.
- **Zero Allocations in Hot Loop**: Layout passes compute with **0 bytes/op** and **0 allocs/op** over a flat array arena.
- **Microsecond Layout Speeds**: Computes 1,000-node Flexbox layouts in **~4.0 µs** (50× faster than the 200 µs target SLA).
- **Analytical Area-Based Anti-Aliasing (Analytic AA)**: Subpixel-precise signed area scanline rasterizer (inspired by FreeType & tiny-skia).
- **Decoupled Architecture**: `plume/layout` has zero dependencies on `plume/raster` or `plume/geom`.

---

## 📦 Packages

| Package | Purpose | Dependencies |
|:---|:---|:---|
| [`plume`](file:///home/pika/Documents/Surfa/plume.go) | Root types: `Color`, `Paint`, `Hex()`, `SavePNG()` | Standard Library only |
| [`plume/layout`](file:///home/pika/Documents/Surfa/layout/) | W3C CSS Flexbox layout engine over flat arena | Standard Library only |
| [`plume/geom`](file:///home/pika/Documents/Surfa/geom/) | Vector paths, Bézier flattening, stroking, dashes, affine transforms | Standard Library only |
| [`plume/raster`](file:///home/pika/Documents/Surfa/raster/) | Analytical anti-aliased CPU scanline rasterizer & Canvas API | `geom`, `layout`, stdlib |

---

## 🚀 Quick Start

```go
package main

import (
    "github.com/plume-engine/plume"
    "github.com/plume-engine/plume/layout"
    "github.com/plume-engine/plume/raster"
)

func main() {
    // 1. Create a layout tree
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
        Display:        layout.DisplayFlex,
        FlexDirection:  layout.FlexDirectionRow,
        AlignItems:     layout.AlignCenter,
        Gap:            layout.Px(16),
        Height:         layout.Px(80),
    })
    tree.AddChild(root, card)

    // 2. Compute Layout (zero-allocations in hot loop)
    tree.Compute(800, 600)

    // 3. Render directly onto a 2D Canvas
    canvas := raster.NewCanvas(800, 600)
    canvas.Clear(plume.Hex("#0f172a"))

    // Draw the computed card with rounded corners
    bounds := tree.ComputedRect(card)
    canvas.DrawRoundedRect(bounds, 12, plume.Fill(plume.Hex("#1e293b")))

    // Draw text placeholder
    canvas.DrawText("Hello from Pure Go!", bounds.X+16, bounds.Y+24, plume.TextStyle{
        FontSize: 18,
        Color:    plume.White,
    })

    // 4. Save to standard Go image
    img := canvas.ToRGBA()
    plume.SavePNG(img, "output.png")
}
```

---

## 📊 Benchmarks

Hardware: `11th Gen Intel(R) Core(TM) i5-1155G7 @ 2.50GHz`

```
BenchmarkFlexbox1000Nodes-8     322,719   4,096 ns/op   0 B/op   0 allocs/op
BenchmarkFlexboxDeep-8          689,304   1,803 ns/op   0 B/op   0 allocs/op
BenchmarkFlattenCircle-8        242,240   6,403 ns/op
BenchmarkBoundingBox-8       11,487,394     115 ns/op   0 B/op   0 allocs/op
BenchmarkCanvasClear-8              331   3.4 ms/op (1080p 60fps capable)
```

---

## 📄 License

Dual-licensed under MIT or Apache 2.0.
