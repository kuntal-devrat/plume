package geom

import "math"

// Dash walks along the flattened version of p, alternating between visible and
// invisible segments according to pattern. The pattern is a repeating sequence
// of [dash, gap, dash, gap, ...] lengths. offset shifts the starting position
// into the pattern (useful for animated dashes).
//
// If pattern is empty, the original path is returned unchanged.
// If any pattern element is ≤ 0, it is treated as a very small positive value.
func Dash(p Path, pattern []float32, offset float32) Path {
	if len(pattern) == 0 {
		return p
	}

	flat := Flatten(p, 0.25)

	// Compute total pattern length for offset normalization.
	patternLen := float32(0)
	for _, v := range pattern {
		if v <= 0 {
			v = 1e-6
		}
		patternLen += v
	}
	if patternLen <= 0 {
		return flat
	}

	// Normalize offset into [0, patternLen).
	offset = float32(math.Mod(float64(offset), float64(patternLen)))
	if offset < 0 {
		offset += patternLen
	}

	// Determine starting pattern index and remaining distance.
	patIdx := 0
	remaining := pattern[0]
	if remaining <= 0 {
		remaining = 1e-6
	}
	consumed := float32(0)
	for consumed+remaining <= offset {
		consumed += remaining
		patIdx = (patIdx + 1) % len(pattern)
		remaining = pattern[patIdx]
		if remaining <= 0 {
			remaining = 1e-6
		}
	}
	remaining -= (offset - consumed)
	drawing := (patIdx % 2) == 0 // even indices are visible

	var out PathBuilder
	var curX, curY float32
	needsMove := true

	pi := 0
	for _, v := range flat.Verbs {
		switch v {
		case VerbMove:
			curX, curY = flat.Points[pi], flat.Points[pi+1]
			pi += 2
			needsMove = true
			// Reset dash state at each sub-path.
			patIdx = 0
			remaining = pattern[0]
			if remaining <= 0 {
				remaining = 1e-6
			}
			consumed = float32(0)
			for consumed+remaining <= offset {
				consumed += remaining
				patIdx = (patIdx + 1) % len(pattern)
				remaining = pattern[patIdx]
				if remaining <= 0 {
					remaining = 1e-6
				}
			}
			remaining -= (offset - consumed)
			drawing = (patIdx % 2) == 0

		case VerbLine:
			lx, ly := flat.Points[pi], flat.Points[pi+1]
			pi += 2
			dashSegment(&out, &curX, &curY, lx, ly, pattern, &patIdx, &remaining, &drawing, &needsMove)

		case VerbClose:
			// Closing line back to sub-path start is implicit; we just
			// end the current dash sub-path if drawing.
			needsMove = true
		}
	}

	return out.Finish()
}

// dashSegment walks along a single line segment from (curX, curY) to (lx, ly),
// splitting it into dashes and gaps.
func dashSegment(out *PathBuilder, curX, curY *float32, lx, ly float32,
	pattern []float32, patIdx *int, remaining *float32, drawing *bool, needsMove *bool) {

	dx := lx - *curX
	dy := ly - *curY
	segLen := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	if segLen < 1e-10 {
		*curX, *curY = lx, ly
		return
	}

	// Unit direction.
	ux := dx / segLen
	uy := dy / segLen
	walked := float32(0)

	for walked < segLen {
		step := segLen - walked
		if *remaining < step {
			step = *remaining
		}
		endX := *curX + ux*step
		endY := *curY + uy*step

		if *drawing {
			if *needsMove {
				out.MoveTo(*curX, *curY)
				*needsMove = false
			}
			out.LineTo(endX, endY)
		}

		walked += step
		*remaining -= step
		*curX = endX
		*curY = endY

		if *remaining <= 1e-10 {
			// Advance to next pattern element.
			*patIdx = (*patIdx + 1) % len(pattern)
			*remaining = pattern[*patIdx]
			if *remaining <= 0 {
				*remaining = 1e-6
			}
			*drawing = !*drawing
			if !*drawing {
				*needsMove = true
			}
		}
	}
}
