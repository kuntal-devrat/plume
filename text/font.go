// Package text provides a pure-Go TrueType / OpenType font parser,
// glyph outline decoder, horizontal text shaper, and LRU glyph cache
// for the Plume rendering engine.
//
// It operates with strictly zero Cgo and zero external dependencies,
// emitting vector paths directly as [geom.Path] structures.
package text

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
)

var (
	ErrInvalidFont = errors.New("text: invalid font format")
	ErrTableNotFound = errors.New("text: required SFNT table not found")
	ErrGlyphNotFound = errors.New("text: glyph not found")
)

// LineMetrics represents vertical font metrics scaled to a given font size.
type LineMetrics struct {
	Ascent      float32 // Distance from baseline to highest ascender (positive).
	Descent     float32 // Distance from baseline to lowest descender (negative or positive downward).
	LineGap     float32 // Recommended spacing between lines.
	LineHeight  float32 // Ascent - Descent + LineGap.
}

// TableRecord describes an SFNT table location within the font data.
type tableRecord struct {
	offset uint32
	length uint32
}

// Font represents a parsed TrueType / OpenType font.
type Font struct {
	data []byte

	// Table index by 4-byte ASCII tag (e.g. "head", "cmap", "glyf", "loca").
	tables map[string]tableRecord

	// 'head' table data
	unitsPerEm        uint16
	indexToLocFormat   int16 // 0 for short offsets (16-bit / 2), 1 for long (32-bit)

	// 'maxp' table data
	numGlyphs uint16

	// 'hhea' table data
	ascent               int16
	descent              int16
	lineGap              int16
	numOfLongHorMetrics  uint16

	// 'hmtx' cached metrics
	hmtxOffset uint32

	// 'loca' and 'glyf' offsets
	locaOffset uint32
	glyfOffset uint32

	// 'cmap' parsed subtables
	cmapFormat4Offset uint32
	cmapFormat12Offset uint32

	// 'kern' table offset
	kernOffset uint32
}

// ParseFont parses a TrueType or OpenType font from byte slice.
func ParseFont(data []byte) (*Font, error) {
	if len(data) < 12 {
		return nil, ErrInvalidFont
	}

	// Verify SFNT signature: 0x00010000 (TrueType), 0x74727565 ('true'), or 0x4F54544F ('OTTO')
	sfntVersion := binary.BigEndian.Uint32(data[0:4])
	if sfntVersion != 0x00010000 && sfntVersion != 0x74727565 && sfntVersion != 0x4F54544F {
		return nil, fmt.Errorf("%w: unrecognized SFNT header 0x%08X", ErrInvalidFont, sfntVersion)
	}

	numTables := int(binary.BigEndian.Uint16(data[4:6]))
	if len(data) < 12+numTables*16 {
		return nil, fmt.Errorf("%w: truncated table directory", ErrInvalidFont)
	}

	tables := make(map[string]tableRecord, numTables)
	for i := 0; i < numTables; i++ {
		offset := 12 + i*16
		tag := string(data[offset : offset+4])
		tableOff := binary.BigEndian.Uint32(data[offset+8 : offset+12])
		tableLen := binary.BigEndian.Uint32(data[offset+12 : offset+16])
		if uint64(tableOff)+uint64(tableLen) > uint64(len(data)) {
			return nil, fmt.Errorf("%w: table %q exceeds font boundaries", ErrInvalidFont, tag)
		}
		tables[tag] = tableRecord{offset: tableOff, length: tableLen}
	}

	f := &Font{
		data:   data,
		tables: tables,
	}

	// 1. Parse 'head'
	head, ok := tables["head"]
	if !ok || head.length < 54 {
		return nil, fmt.Errorf("%w: 'head' table missing or too short", ErrInvalidFont)
	}
	f.unitsPerEm = binary.BigEndian.Uint16(data[head.offset+18 : head.offset+20])
	f.indexToLocFormat = int16(binary.BigEndian.Uint16(data[head.offset+50 : head.offset+52]))

	// 2. Parse 'maxp'
	maxp, ok := tables["maxp"]
	if !ok || maxp.length < 6 {
		return nil, fmt.Errorf("%w: 'maxp' table missing or too short", ErrInvalidFont)
	}
	f.numGlyphs = binary.BigEndian.Uint16(data[maxp.offset+4 : maxp.offset+6])

	// 3. Parse 'hhea'
	hhea, ok := tables["hhea"]
	if !ok || hhea.length < 36 {
		return nil, fmt.Errorf("%w: 'hhea' table missing or too short", ErrInvalidFont)
	}
	f.ascent = int16(binary.BigEndian.Uint16(data[hhea.offset+4 : hhea.offset+6]))
	f.descent = int16(binary.BigEndian.Uint16(data[hhea.offset+6 : hhea.offset+8]))
	f.lineGap = int16(binary.BigEndian.Uint16(data[hhea.offset+8 : hhea.offset+10]))
	f.numOfLongHorMetrics = binary.BigEndian.Uint16(data[hhea.offset+34 : hhea.offset+36])

	// 4. 'hmtx'
	hmtx, ok := tables["hmtx"]
	if !ok {
		return nil, fmt.Errorf("%w: 'hmtx' table missing", ErrInvalidFont)
	}
	f.hmtxOffset = hmtx.offset

	// 5. 'loca' and 'glyf'
	if loca, ok := tables["loca"]; ok {
		f.locaOffset = loca.offset
	}
	if glyf, ok := tables["glyf"]; ok {
		f.glyfOffset = glyf.offset
	}

	// 6. 'cmap'
	if err := f.initCmap(); err != nil {
		return nil, err
	}

	// 7. Optional 'kern'
	if kern, ok := tables["kern"]; ok {
		f.kernOffset = kern.offset
	}

	return f, nil
}

// LoadFontFile reads and parses a font from the filesystem.
func LoadFontFile(path string) (*Font, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("text.LoadFontFile: %w", err)
	}
	return ParseFont(bytes)
}

// UnitsPerEm returns the design grid units per em.
func (f *Font) UnitsPerEm() uint16 {
	return f.unitsPerEm
}

// NumGlyphs returns the total number of glyphs in the font.
func (f *Font) NumGlyphs() uint16 {
	return f.numGlyphs
}

// LineMetrics calculates vertical metrics scaled to the specified fontSize in pixels.
func (f *Font) LineMetrics(fontSize float32) LineMetrics {
	scale := fontSize / float32(f.unitsPerEm)
	asc := float32(f.ascent) * scale
	desc := float32(f.descent) * scale
	gap := float32(f.lineGap) * scale
	return LineMetrics{
		Ascent:     asc,
		Descent:    desc,
		LineGap:    gap,
		LineHeight: asc - desc + gap,
	}
}
