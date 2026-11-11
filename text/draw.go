package text

import (
	"sync"

	plume "github.com/plume-engine/plume"
	"github.com/plume-engine/plume/raster"
)

var defaultCache = NewGlyphCache(2048)

type layoutKey struct {
	font     *Font
	str      string
	fontSize uint32
	maxWidth uint32
}

var (
	layoutCacheMu sync.RWMutex
	layoutCache   = make(map[layoutKey]TextLayout, 512)
)

func getOrShapeText(font *Font, str string, fontSize, maxWidth float32) TextLayout {
	key := layoutKey{
		font:     font,
		str:      str,
		fontSize: uint32(fontSize * 64),
		maxWidth: uint32(maxWidth * 64),
	}

	layoutCacheMu.RLock()
	tl, ok := layoutCache[key]
	layoutCacheMu.RUnlock()
	if ok {
		return tl
	}

	tl = ShapeText(font, str, fontSize, maxWidth)

	layoutCacheMu.Lock()
	if len(layoutCache) > 1024 {
		layoutCache = make(map[layoutKey]TextLayout, 512)
	}
	layoutCache[key] = tl
	layoutCacheMu.Unlock()

	return tl
}

// DrawText renders shaped text onto a canvas using the given font and glyph cache.
// (x, y) specifies the origin where y is the baseline of the first line.
func DrawText(canvas *raster.Canvas, cache *GlyphCache, font *Font, str string, x, y float32, fontSize float32, col plume.Color) {
	if canvas == nil || font == nil || len(str) == 0 {
		return
	}
	if cache == nil {
		cache = defaultCache
	}

	layout := getOrShapeText(font, str, fontSize, 0)
	var lineY float32

	for _, line := range layout.Lines {
		for _, g := range line.Glyphs {
			if g.GlyphID == 0 {
				continue
			}
			cg := cache.GetOrRasterize(font, g.GlyphID, fontSize)
			if cg.Width == 0 || cg.Height == 0 {
				continue
			}
			gx := int(x + g.X + cg.BearingX + 0.5)
			gy := int(y + lineY + g.Y + cg.BearingY + 0.5)
			canvas.DrawMask(cg.Alpha, gx, gy, cg.Width, cg.Height, col)
		}
		lineY += line.Height
	}
}

// DrawTextWrapped renders shaped and word-wrapped text bounded by maxWidth.
func DrawTextWrapped(canvas *raster.Canvas, cache *GlyphCache, font *Font, str string, x, y, maxWidth float32, fontSize float32, col plume.Color) float32 {
	if canvas == nil || font == nil || len(str) == 0 {
		return 0
	}
	if cache == nil {
		cache = defaultCache
	}

	layout := getOrShapeText(font, str, fontSize, maxWidth)
	var lineY float32

	for _, line := range layout.Lines {
		for _, g := range line.Glyphs {
			if g.GlyphID == 0 {
				continue
			}
			cg := cache.GetOrRasterize(font, g.GlyphID, fontSize)
			if cg.Width == 0 || cg.Height == 0 {
				continue
			}
			gx := int(x + g.X + cg.BearingX + 0.5)
			gy := int(y + lineY + g.Y + cg.BearingY + 0.5)
			canvas.DrawMask(cg.Alpha, gx, gy, cg.Width, cg.Height, col)
		}
		lineY += line.Height
	}

	return layout.TotalHeight
}
