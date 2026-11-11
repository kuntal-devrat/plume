package text

import (
	"encoding/binary"
)

// Ligature represents an OpenType glyph substitution rule where a sequence
// of component glyph IDs is replaced by a single ligature glyph ID.
type Ligature struct {
	Components  []uint16 // Sequence of following glyph IDs to match
	Replacement uint16   // Ligature glyph ID
}

// LigatureMap maps a first glyph ID to a slice of possible ligatures.
type LigatureMap map[uint16][]Ligature

// parseGSUB attempts to parse Ligature Substitution (Lookup Type 4) from
// the font's OpenType GSUB table.
func (f *Font) parseGSUB() LigatureMap {
	gsubRec, ok := f.tables["GSUB"]
	if !ok || gsubRec.length < 10 {
		return nil
	}

	data := f.data[gsubRec.offset : gsubRec.offset+gsubRec.length]
	majorVersion := binary.BigEndian.Uint16(data[0:2])
	if majorVersion != 1 {
		return nil
	}

	lookupListOffset := binary.BigEndian.Uint16(data[8:10])
	if int(lookupListOffset)+2 > len(data) {
		return nil
	}

	llData := data[lookupListOffset:]
	lookupCount := int(binary.BigEndian.Uint16(llData[0:2]))
	if len(llData) < 2+lookupCount*2 {
		return nil
	}

	ligatures := make(LigatureMap)

	for i := 0; i < lookupCount; i++ {
		lOffset := int(binary.BigEndian.Uint16(llData[2+i*2 : 4+i*2]))
		if lOffset+6 > len(llData) {
			continue
		}
		lookupData := llData[lOffset:]
		lookupType := binary.BigEndian.Uint16(lookupData[0:2])
		subTableCount := int(binary.BigEndian.Uint16(lookupData[4:6]))

		if lookupType != 4 { // Type 4 is Ligature Substitution
			continue
		}

		for s := 0; s < subTableCount; s++ {
			if 6+(s+1)*2 > len(lookupData) {
				continue
			}
			stOffset := int(binary.BigEndian.Uint16(lookupData[6+s*2 : 8+s*2]))
			if stOffset+6 > len(lookupData) {
				continue
			}
			stData := lookupData[stOffset:]
			subFormat := binary.BigEndian.Uint16(stData[0:2])
			if subFormat != 1 {
				continue
			}

			coverageOffset := int(binary.BigEndian.Uint16(stData[2:4]))
			ligSetCount := int(binary.BigEndian.Uint16(stData[4:6]))
			if 6+ligSetCount*2 > len(stData) {
				continue
			}

			covGlyphs := parseCoverage(stData, coverageOffset)
			if len(covGlyphs) < ligSetCount {
				continue
			}

			for c := 0; c < ligSetCount; c++ {
				firstGlyph := covGlyphs[c]
				lsOffset := int(binary.BigEndian.Uint16(stData[6+c*2 : 8+c*2]))
				if lsOffset+2 > len(stData) {
					continue
				}
				lsData := stData[lsOffset:]
				ligCount := int(binary.BigEndian.Uint16(lsData[0:2]))

				for l := 0; l < ligCount; l++ {
					if 2+(l+1)*2 > len(lsData) {
						continue
					}
					ligOffset := int(binary.BigEndian.Uint16(lsData[2+l*2 : 4+l*2]))
					if ligOffset+4 > len(lsData) {
						continue
					}
					ligData := lsData[ligOffset:]
					ligGlyph := binary.BigEndian.Uint16(ligData[0:2])
					compCount := int(binary.BigEndian.Uint16(ligData[2:4]))
					if compCount < 2 || 4+(compCount-1)*2 > len(ligData) {
						continue
					}

					comps := make([]uint16, compCount-1)
					for k := 0; k < compCount-1; k++ {
						comps[k] = binary.BigEndian.Uint16(ligData[4+k*2 : 6+k*2])
					}

					ligatures[firstGlyph] = append(ligatures[firstGlyph], Ligature{
						Components:  comps,
						Replacement: ligGlyph,
					})
				}
			}
		}
	}

	if len(ligatures) == 0 {
		return nil
	}
	return ligatures
}

func parseCoverage(data []byte, offset int) []uint16 {
	if offset+4 > len(data) {
		return nil
	}
	covData := data[offset:]
	format := binary.BigEndian.Uint16(covData[0:2])

	if format == 1 {
		count := int(binary.BigEndian.Uint16(covData[2:4]))
		if 4+count*2 > len(covData) {
			return nil
		}
		glyphs := make([]uint16, count)
		for i := 0; i < count; i++ {
			glyphs[i] = binary.BigEndian.Uint16(covData[4+i*2 : 6+i*2])
		}
		return glyphs
	} else if format == 2 {
		rangeCount := int(binary.BigEndian.Uint16(covData[2:4]))
		if 4+rangeCount*6 > len(covData) {
			return nil
		}
		var glyphs []uint16
		for i := 0; i < rangeCount; i++ {
			start := binary.BigEndian.Uint16(covData[4+i*6 : 6+i*6])
			end := binary.BigEndian.Uint16(covData[6+i*6 : 8+i*6])
			for g := start; g <= end; g++ {
				glyphs = append(glyphs, g)
			}
		}
		return glyphs
	}
	return nil
}

// ApplyLigatures replaces sequences of glyph IDs with their OpenType ligature
// substitutions when defined in the font's GSUB table.
func (f *Font) ApplyLigatures(glyphs []uint16) []uint16 {
	if len(glyphs) < 2 {
		return glyphs
	}

	if f.ligatures == nil && f.gsubParsed {
		return glyphs
	}
	if !f.gsubParsed {
		f.ligatures = f.parseGSUB()
		f.gsubParsed = true
	}

	if f.ligatures == nil {
		return glyphs
	}

	out := make([]uint16, 0, len(glyphs))
	i := 0
	for i < len(glyphs) {
		g := glyphs[i]
		ligs, ok := f.ligatures[g]
		if !ok || i+1 >= len(glyphs) {
			out = append(out, g)
			i++
			continue
		}

		matched := false
		for _, lig := range ligs {
			compLen := len(lig.Components)
			if i+1+compLen > len(glyphs) {
				continue
			}
			match := true
			for k := 0; k < compLen; k++ {
				if glyphs[i+1+k] != lig.Components[k] {
					match = false
					break
				}
			}
			if match {
				out = append(out, lig.Replacement)
				i += 1 + compLen
				matched = true
				break
			}
		}

		if !matched {
			out = append(out, g)
			i++
		}
	}
	return out
}
