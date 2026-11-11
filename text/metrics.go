package text

import (
	"encoding/binary"
)

// GlyphAdvance returns the horizontal advance width of a glyph in pixels.
func (f *Font) GlyphAdvance(glyphID uint16, fontSize float32) float32 {
	if glyphID >= f.numGlyphs || f.hmtxOffset == 0 {
		return 0
	}

	scale := fontSize / float32(f.unitsPerEm)

	// If glyphID < numOfLongHorMetrics, read advance directly.
	// Otherwise, it shares the advance of the last long horizontal metric.
	var advanceWidth uint16
	if glyphID < f.numOfLongHorMetrics {
		off := f.hmtxOffset + uint32(glyphID)*4
		if int(off+2) <= len(f.data) {
			advanceWidth = binary.BigEndian.Uint16(f.data[off : off+2])
		}
	} else {
		lastOff := f.hmtxOffset + uint32(f.numOfLongHorMetrics-1)*4
		if int(lastOff+2) <= len(f.data) {
			advanceWidth = binary.BigEndian.Uint16(f.data[lastOff : lastOff+2])
		}
	}

	return float32(advanceWidth) * scale
}

// Kerning returns horizontal kerning offset between left and right glyphs in pixels.
func (f *Font) Kerning(left, right uint16, fontSize float32) float32 {
	if f.kernOffset == 0 {
		return 0
	}

	data := f.data[f.kernOffset:]
	if len(data) < 4 {
		return 0
	}

	nTables := binary.BigEndian.Uint16(data[2:4])
	offset := 4

	for i := 0; i < int(nTables); i++ {
		if offset+6 > len(data) {
			break
		}
		subLen := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))
		coverage := binary.BigEndian.Uint16(data[offset+4 : offset+6])

		// Format 0 horizontal kerning
		format := coverage >> 8
		isHorizontal := (coverage & 1) != 0
		if format == 0 && isHorizontal && offset+14 <= len(data) {
			nPairs := int(binary.BigEndian.Uint16(data[offset+6 : offset+8]))
			pairsData := data[offset+14 : offset+subLen]

			target := (uint32(left) << 16) | uint32(right)

			// Binary search pairs
			lo := 0
			hi := nPairs - 1
			for lo <= hi {
				mid := (lo + hi) / 2
				pOff := mid * 6
				if pOff+6 > len(pairsData) {
					break
				}
				pairVal := binary.BigEndian.Uint32(pairsData[pOff : pOff+4])
				if pairVal == target {
					val := int16(binary.BigEndian.Uint16(pairsData[pOff+4 : pOff+6]))
					return float32(val) * (fontSize / float32(f.unitsPerEm))
				}
				if pairVal < target {
					lo = mid + 1
				} else {
					hi = mid - 1
				}
			}
		}

		offset += subLen
	}

	return 0
}
