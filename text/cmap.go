package text

import (
	"encoding/binary"
	"fmt"
)

// initCmap finds and caches offsets to format 4 and format 12 subtables in 'cmap'.
func (f *Font) initCmap() error {
	cmap, ok := f.tables["cmap"]
	if !ok || cmap.length < 4 {
		return fmt.Errorf("%w: 'cmap' table missing", ErrInvalidFont)
	}

	data := f.data[cmap.offset : cmap.offset+cmap.length]
	numSubtables := binary.BigEndian.Uint16(data[2:4])

	for i := 0; i < int(numSubtables); i++ {
		off := 4 + i*8
		if off+8 > len(data) {
			break
		}
		platformID := binary.BigEndian.Uint16(data[off : off+2])
		encodingID := binary.BigEndian.Uint16(data[off+2 : off+4])
		subtableOffset := binary.BigEndian.Uint32(data[off+4 : off+8])

		if subtableOffset+2 > uint32(len(data)) {
			continue
		}

		format := binary.BigEndian.Uint16(data[subtableOffset : subtableOffset+2])

		// Unicode platform (0) or Windows Unicode BMP (3, 1) or Windows Unicode full (3, 10)
		if (platformID == 0 || (platformID == 3 && encodingID == 1)) && format == 4 {
			if f.cmapFormat4Offset == 0 {
				f.cmapFormat4Offset = cmap.offset + subtableOffset
			}
		} else if (platformID == 0 || (platformID == 3 && encodingID == 10)) && format == 12 {
			if f.cmapFormat12Offset == 0 {
				f.cmapFormat12Offset = cmap.offset + subtableOffset
			}
		}
	}

	if f.cmapFormat4Offset == 0 && f.cmapFormat12Offset == 0 {
		return fmt.Errorf("%w: no supported Unicode subtable (format 4 or 12) in 'cmap'", ErrInvalidFont)
	}
	return nil
}

// GlyphIndex maps a Unicode rune to its glyph index in the font.
// Returns 0 (notdef glyph) if the character is not present in the font.
func (f *Font) GlyphIndex(r rune) uint16 {
	// 1. Try format 12 (full 32-bit Unicode)
	if f.cmapFormat12Offset != 0 {
		idx := f.lookupFormat12(uint32(r))
		if idx != 0 {
			return idx
		}
	}

	// 2. Try format 4 (BMP Unicode, 16-bit)
	if f.cmapFormat4Offset != 0 && uint32(r) <= 0xFFFF {
		return f.lookupFormat4(uint16(r))
	}

	return 0
}

// lookupFormat4 performs binary lookup in a Format 4 cmap subtable.
func (f *Font) lookupFormat4(c uint16) uint16 {
	data := f.data[f.cmapFormat4Offset:]
	if len(data) < 14 {
		return 0
	}

	segCount := int(binary.BigEndian.Uint16(data[6:8])) / 2
	if len(data) < 14+segCount*8 {
		return 0
	}

	endCountOff := 14
	startCountOff := endCountOff + segCount*2 + 2 // skip reservedPad
	idDeltaOff := startCountOff + segCount*2
	idRangeOffsetOff := idDeltaOff + segCount*2

	// Binary search through segment endCount
	lo := 0
	hi := segCount - 1
	seg := -1
	for lo <= hi {
		mid := (lo + hi) / 2
		endCode := binary.BigEndian.Uint16(data[endCountOff+mid*2 : endCountOff+mid*2+2])
		if c <= endCode {
			seg = mid
			hi = mid - 1
		} else {
			lo = mid + 1
		}
	}

	if seg == -1 {
		return 0
	}

	startCode := binary.BigEndian.Uint16(data[startCountOff+seg*2 : startCountOff+seg*2+2])
	if c < startCode {
		return 0
	}

	idDelta := int16(binary.BigEndian.Uint16(data[idDeltaOff+seg*2 : idDeltaOff+seg*2+2]))
	idRangeOffset := binary.BigEndian.Uint16(data[idRangeOffsetOff+seg*2 : idRangeOffsetOff+seg*2+2])

	if idRangeOffset == 0 {
		return uint16(int32(c) + int32(idDelta))
	}

	// idRangeOffset points into glyphIdArray
	ro := int(idRangeOffset)
	glyphOffset := idRangeOffsetOff + seg*2 + ro + int(c-startCode)*2
	if glyphOffset+2 > len(data) {
		return 0
	}
	glyphID := binary.BigEndian.Uint16(data[glyphOffset : glyphOffset+2])
	if glyphID != 0 {
		return uint16(int32(glyphID) + int32(idDelta))
	}
	return 0
}

// lookupFormat12 performs binary search in a Format 12 segmented coverage table.
func (f *Font) lookupFormat12(c uint32) uint16 {
	data := f.data[f.cmapFormat12Offset:]
	if len(data) < 16 {
		return 0
	}

	numGroups := binary.BigEndian.Uint32(data[12:16])
	groupsOff := 16
	if uint64(groupsOff)+uint64(numGroups)*12 > uint64(len(data)) {
		return 0
	}

	lo := uint32(0)
	hi := numGroups - 1
	for lo <= hi {
		mid := (lo + hi) / 2
		off := groupsOff + int(mid)*12
		startCharCode := binary.BigEndian.Uint32(data[off : off+4])
		endCharCode := binary.BigEndian.Uint32(data[off+4 : off+8])
		startGlyphID := binary.BigEndian.Uint32(data[off+8 : off+12])

		if c >= startCharCode && c <= endCharCode {
			return uint16(startGlyphID + (c - startCharCode))
		}
		if c < startCharCode {
			if mid == 0 {
				break
			}
			hi = mid - 1
		} else {
			lo = mid + 1
		}
	}
	return 0
}
