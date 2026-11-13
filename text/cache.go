package text

import (
	"container/list"
	"math"

	"github.com/kuntal-devrat/plume/geom"
	"github.com/kuntal-devrat/plume/raster"
)

// CachedGlyph contains pre-rasterized alpha mask data and placement offsets.
type CachedGlyph struct {
	Width    int     // Width of alpha mask in pixels
	Height   int     // Height of alpha mask in pixels
	BearingX float32 // Horizontal offset from glyph origin to left of mask
	BearingY float32 // Vertical offset from glyph origin (baseline) to top of mask (usually negative)
	Alpha    []uint8 // 8-bit coverage mask, size = Width * Height
}

type cacheKey struct {
	font    *Font
	glyphID uint16
	sizeKey uint32 // fontSize * 64 for subpixel binning
}

type cacheEntry struct {
	key   cacheKey
	glyph CachedGlyph
}

// GlyphCache provides fixed-capacity LRU caching for pre-rasterized glyphs.
type GlyphCache struct {
	capacity int
	entries  map[cacheKey]*list.Element
	lruList  *list.List
}

// NewGlyphCache creates an LRU glyph cache with the specified capacity (e.g. 2048).
func NewGlyphCache(capacity int) *GlyphCache {
	if capacity <= 0 {
		capacity = 2048
	}
	return &GlyphCache{
		capacity: capacity,
		entries:  make(map[cacheKey]*list.Element, capacity),
		lruList:  list.New(),
	}
}

// GetOrRasterize retrieves a cached glyph mask or rasterizes it via the analytical rasterizer.
func (c *GlyphCache) GetOrRasterize(font *Font, glyphID uint16, fontSize float32) CachedGlyph {
	key := cacheKey{
		font:    font,
		glyphID: glyphID,
		sizeKey: uint32(fontSize * 64.0),
	}

	if elem, ok := c.entries[key]; ok {
		c.lruList.MoveToFront(elem)
		return elem.Value.(*cacheEntry).glyph
	}

	// Rasterize on cache miss
	cg := c.rasterizeGlyph(font, glyphID, fontSize)

	// Insert into LRU
	entry := &cacheEntry{key: key, glyph: cg}
	elem := c.lruList.PushFront(entry)
	c.entries[key] = elem

	if c.lruList.Len() > c.capacity {
		oldest := c.lruList.Back()
		if oldest != nil {
			c.lruList.Remove(oldest)
			delete(c.entries, oldest.Value.(*cacheEntry).key)
		}
	}

	return cg
}

func (c *GlyphCache) rasterizeGlyph(font *Font, glyphID uint16, fontSize float32) CachedGlyph {
	path, err := font.GlyphPath(glyphID, fontSize)
	if err != nil || len(path.Verbs) == 0 {
		return CachedGlyph{}
	}

	minX, minY, maxX, maxY := geom.BoundingBox(path)
	if maxX <= minX || maxY <= minY {
		return CachedGlyph{}
	}

	// Add 1px safety padding for anti-aliasing edges
	x0 := float32(math.Floor(float64(minX))) - 1
	y0 := float32(math.Floor(float64(minY))) - 1
	x1 := float32(math.Ceil(float64(maxX))) + 1
	y1 := float32(math.Ceil(float64(maxY))) + 1

	w := int(x1 - x0)
	h := int(y1 - y0)
	if w <= 0 || h <= 0 {
		return CachedGlyph{}
	}

	// Translate path so its bounding box starts at (0, 0)
	trans := geom.Translate(-x0, -y0)
	shiftedPath := geom.TransformPath(path, trans)

	r := raster.NewRasterizer(w, h)
	r.AddPath(shiftedPath)

	alpha := make([]uint8, w*h)
	r.Rasterize(alpha)

	return CachedGlyph{
		Width:    w,
		Height:   h,
		BearingX: x0,
		BearingY: y0,
		Alpha:    alpha,
	}
}
