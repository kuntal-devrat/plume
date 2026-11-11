package text

import (
	"encoding/binary"
	"fmt"

	"github.com/plume-engine/plume/geom"
)

// Flags used in simple glyphs
const (
	flagOnCurve    = 0x01
	flagXShort     = 0x02
	flagYShort     = 0x04
	flagRepeat     = 0x08
	flagXSameOrPos = 0x10
	flagYSameOrPos = 0x20
)

// Flags used in composite glyphs
const (
	compArg1And2AreWords    = 0x0001
	compArgsAreXYValues     = 0x0002
	compRoundXYToGrid       = 0x0004
	compWeHaveAScale        = 0x0008
	compMoreComponents      = 0x0020
	compWeHaveAnXAndYScale  = 0x0040
	compWeHaveATwoByTwo     = 0x0080
	compWeHaveInstructions  = 0x0100
	compUseMyMetrics        = 0x0200
	compOverlapCompound     = 0x0400
)

type glyphPoint struct {
	x, y    int32
	onCurve bool
}

// GlyphPath decodes the vector outline of a glyph scaled to fontSize (in pixels).
// Baseline is at y = 0, ascenders are at y < 0 (canvas coordinates where Y increases down).
func (f *Font) GlyphPath(glyphID uint16, fontSize float32) (geom.Path, error) {
	if glyphID >= f.numGlyphs || f.glyfOffset == 0 || f.locaOffset == 0 {
		return geom.Path{}, ErrGlyphNotFound
	}

	offset, length := f.glyphOffset(glyphID)
	if length == 0 {
		// Empty glyph (e.g. space)
		return geom.Path{}, nil
	}

	data := f.data[offset : offset+length]
	if len(data) < 10 {
		return geom.Path{}, fmt.Errorf("%w: glyph header too short", ErrInvalidFont)
	}

	numberOfContours := int16(binary.BigEndian.Uint16(data[0:2]))
	scale := fontSize / float32(f.unitsPerEm)

	if numberOfContours >= 0 {
		return f.decodeSimpleGlyph(data, numberOfContours, scale)
	}

	return f.decodeCompositeGlyph(data, scale, fontSize)
}

func (f *Font) glyphOffset(glyphID uint16) (uint32, uint32) {
	var start, end uint32
	if f.indexToLocFormat == 0 {
		// 16-bit offset / 2
		off := f.locaOffset + uint32(glyphID)*2
		start = uint32(binary.BigEndian.Uint16(f.data[off:off+2])) * 2
		end = uint32(binary.BigEndian.Uint16(f.data[off+2:off+4])) * 2
	} else {
		// 32-bit offset
		off := f.locaOffset + uint32(glyphID)*4
		start = binary.BigEndian.Uint32(f.data[off : off+4])
		end = binary.BigEndian.Uint32(f.data[off+4 : off+8])
	}

	if start >= end || f.glyfOffset+end > uint32(len(f.data)) {
		return 0, 0
	}
	return f.glyfOffset + start, end - start
}

func (f *Font) decodeSimpleGlyph(data []byte, numContours int16, scale float32) (geom.Path, error) {
	if numContours == 0 {
		return geom.Path{}, nil
	}

	// 1. End points of contours
	contourEnds := make([]uint16, numContours)
	for i := 0; i < int(numContours); i++ {
		contourEnds[i] = binary.BigEndian.Uint16(data[10+i*2 : 12+i*2])
	}
	numPoints := int(contourEnds[numContours-1]) + 1

	// Skip instructions
	insLenOffset := 10 + int(numContours)*2
	if insLenOffset+2 > len(data) {
		return geom.Path{}, ErrInvalidFont
	}
	instructionLen := int(binary.BigEndian.Uint16(data[insLenOffset : insLenOffset+2]))
	flagsOffset := insLenOffset + 2 + instructionLen

	if flagsOffset > len(data) {
		return geom.Path{}, ErrInvalidFont
	}

	// 2. Read flags
	flags := make([]byte, numPoints)
	pos := flagsOffset
	for i := 0; i < numPoints; {
		if pos >= len(data) {
			return geom.Path{}, ErrInvalidFont
		}
		flag := data[pos]
		pos++
		flags[i] = flag
		i++
		if (flag & flagRepeat) != 0 {
			if pos >= len(data) {
				return geom.Path{}, ErrInvalidFont
			}
			repeatCount := int(data[pos])
			pos++
			for r := 0; r < repeatCount && i < numPoints; r++ {
				flags[i] = flag
				i++
			}
		}
	}

	// 3. Read X coordinates
	points := make([]glyphPoint, numPoints)
	var curX int32
	for i := 0; i < numPoints; i++ {
		flag := flags[i]
		points[i].onCurve = (flag & flagOnCurve) != 0

		if (flag & flagXShort) != 0 {
			if pos >= len(data) {
				return geom.Path{}, ErrInvalidFont
			}
			val := int32(data[pos])
			pos++
			if (flag & flagXSameOrPos) == 0 {
				val = -val
			}
			curX += val
		} else {
			if (flag & flagXSameOrPos) == 0 {
				if pos+2 > len(data) {
					return geom.Path{}, ErrInvalidFont
				}
				val := int32(int16(binary.BigEndian.Uint16(data[pos : pos+2])))
				pos += 2
				curX += val
			}
			// else curX remains unchanged
		}
		points[i].x = curX
	}

	// 4. Read Y coordinates
	var curY int32
	for i := 0; i < numPoints; i++ {
		flag := flags[i]
		if (flag & flagYShort) != 0 {
			if pos >= len(data) {
				return geom.Path{}, ErrInvalidFont
			}
			val := int32(data[pos])
			pos++
			if (flag & flagYSameOrPos) == 0 {
				val = -val
			}
			curY += val
		} else {
			if (flag & flagYSameOrPos) == 0 {
				if pos+2 > len(data) {
					return geom.Path{}, ErrInvalidFont
				}
				val := int32(int16(binary.BigEndian.Uint16(data[pos : pos+2])))
				pos += 2
				curY += val
			}
			// else curY remains unchanged
		}
		points[i].y = curY
	}

	// 5. Convert contours to vector path
	var pb geom.PathBuilder
	startIndex := 0

	for _, endIndex := range contourEnds {
		pts := points[startIndex : int(endIndex)+1]
		startIndex = int(endIndex) + 1
		if len(pts) == 0 {
			continue
		}

		f.emitContour(&pb, pts, scale)
	}

	return pb.Finish(), nil
}

func (f *Font) emitContour(pb *geom.PathBuilder, pts []glyphPoint, scale float32) {
	n := len(pts)
	if n == 0 {
		return
	}

	toScreen := func(pt glyphPoint) (float32, float32) {
		return float32(pt.x) * scale, -float32(pt.y) * scale
	}

	// Determine starting on-curve point
	var startX, startY float32
	startIdx := 0

	if pts[0].onCurve {
		startX, startY = toScreen(pts[0])
		startIdx = 1
	} else if pts[n-1].onCurve {
		startX, startY = toScreen(pts[n-1])
		startIdx = 0
	} else {
		// Both first and last are off-curve: start at their midpoint
		x0, y0 := toScreen(pts[0])
		xn, yn := toScreen(pts[n-1])
		startX, startY = (x0+xn)*0.5, (y0+yn)*0.5
		startIdx = 0
	}

	pb.MoveTo(startX, startY)

	for i := 0; i < n; i++ {
		idx := (startIdx + i) % n
		pt := pts[idx]

		if pt.onCurve {
			x, y := toScreen(pt)
			pb.LineTo(x, y)
		} else {
			// Off-curve point: look ahead to next point
			nextPt := pts[(idx+1)%n]
			cx, cy := toScreen(pt)
			if nextPt.onCurve {
				nx, ny := toScreen(nextPt)
				pb.QuadTo(cx, cy, nx, ny)
				i++ // skip next point since it was consumed
			} else {
				// Consecutive off-curve: implicit on-curve point at midpoint
				nx, ny := toScreen(nextPt)
				midX, midY := (cx+nx)*0.5, (cy+ny)*0.5
				pb.QuadTo(cx, cy, midX, midY)
			}
		}
	}

	pb.Close()
}

func (f *Font) decodeCompositeGlyph(data []byte, scale, fontSize float32) (geom.Path, error) {
	var pb geom.PathBuilder
	pos := 10 // skip header

	for {
		if pos+4 > len(data) {
			break
		}
		flags := binary.BigEndian.Uint16(data[pos : pos+2])
		glyphIndex := binary.BigEndian.Uint16(data[pos+2 : pos+4])
		pos += 4

		// Argument 1 & 2
		var arg1, arg2 int16
		if (flags & compArg1And2AreWords) != 0 {
			if pos+4 > len(data) {
				break
			}
			arg1 = int16(binary.BigEndian.Uint16(data[pos : pos+2]))
			arg2 = int16(binary.BigEndian.Uint16(data[pos+2 : pos+4]))
			pos += 4
		} else {
			if pos+2 > len(data) {
				break
			}
			arg1 = int16(int8(data[pos]))
			arg2 = int16(int8(data[pos+1]))
			pos += 2
		}

		// Transformation matrix
		var a, b, c, d float32 = 1.0, 0.0, 0.0, 1.0
		if (flags & compWeHaveAScale) != 0 {
			if pos+2 > len(data) {
				break
			}
			s := float32(int16(binary.BigEndian.Uint16(data[pos:pos+2]))) / 16384.0
			a, d = s, s
			pos += 2
		} else if (flags & compWeHaveAnXAndYScale) != 0 {
			if pos+4 > len(data) {
				break
			}
			a = float32(int16(binary.BigEndian.Uint16(data[pos:pos+2]))) / 16384.0
			d = float32(int16(binary.BigEndian.Uint16(data[pos+2:pos+4]))) / 16384.0
			pos += 4
		} else if (flags & compWeHaveATwoByTwo) != 0 {
			if pos+8 > len(data) {
				break
			}
			a = float32(int16(binary.BigEndian.Uint16(data[pos:pos+2]))) / 16384.0
			b = float32(int16(binary.BigEndian.Uint16(data[pos+2:pos+4]))) / 16384.0
			c = float32(int16(binary.BigEndian.Uint16(data[pos+4:pos+6]))) / 16384.0
			d = float32(int16(binary.BigEndian.Uint16(data[pos+6:pos+8]))) / 16384.0
			pos += 8
		}

		var dx, dy float32
		if (flags & compArgsAreXYValues) != 0 {
			dx = float32(arg1) * scale
			dy = -float32(arg2) * scale
		}

		// Recursively decode child glyph
		childPath, err := f.GlyphPath(glyphIndex, fontSize)
		if err == nil && len(childPath.Verbs) > 0 {
			// Apply 2D affine transform to child path
			aff := geom.Affine{
				A:  a,
				B:  b,
				C:  c,
				D:  d,
				Tx: dx,
				Ty: dy,
			}
			transPath := geom.TransformPath(childPath, aff)
			f.appendPath(&pb, transPath)
		}

		if (flags & compMoreComponents) == 0 {
			break
		}
	}

	return pb.Finish(), nil
}

func (f *Font) appendPath(pb *geom.PathBuilder, p geom.Path) {
	idx := 0
	for _, v := range p.Verbs {
		switch v {
		case geom.VerbMove:
			pb.MoveTo(p.Points[idx], p.Points[idx+1])
			idx += 2
		case geom.VerbLine:
			pb.LineTo(p.Points[idx], p.Points[idx+1])
			idx += 2
		case geom.VerbQuad:
			pb.QuadTo(p.Points[idx], p.Points[idx+1], p.Points[idx+2], p.Points[idx+3])
			idx += 4
		case geom.VerbCubic:
			pb.CubicTo(p.Points[idx], p.Points[idx+1], p.Points[idx+2], p.Points[idx+3], p.Points[idx+4], p.Points[idx+5])
			idx += 6
		case geom.VerbClose:
			pb.Close()
		}
	}
}
