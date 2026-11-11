package geom

import "math"

// CapStyle determines how the open ends of a stroked sub-path are rendered.
type CapStyle uint8

const (
	// CapButt ends the stroke flush at the endpoint with no extension.
	CapButt CapStyle = iota
	// CapRound adds a semicircle of diameter equal to the stroke width.
	CapRound
	// CapSquare extends the stroke by half the stroke width as a rectangle.
	CapSquare
)

// JoinStyle determines how corners between consecutive stroked line segments
// are rendered.
type JoinStyle uint8

const (
	// JoinMiter extends the outer edges until they meet at a sharp point.
	// If the miter length exceeds [StrokeOptions.MiterLimit] × half-width,
	// a bevel is used instead.
	JoinMiter JoinStyle = iota
	// JoinRound fills the gap with a circular arc.
	JoinRound
	// JoinBevel connects the outer endpoints with a straight line.
	JoinBevel
)

// StrokeOptions configures how [Stroke] expands a path into a filled outline.
type StrokeOptions struct {
	// Width is the total stroke width in pixels. Must be > 0.
	Width float32
	// Cap selects the end-cap style.
	Cap CapStyle
	// Join selects the corner-join style.
	Join JoinStyle
	// MiterLimit is the maximum ratio of miter length to half-width before
	// a bevel is used. A value of 0 is treated as the SVG default of 4.0.
	MiterLimit float32
}

// Stroke expands the path p into a filled outline suitable for rasterization.
//
// The input is first flattened (curves → lines), then each line segment is
// offset by ±width/2 along the perpendicular. Adjacent segments are connected
// with the selected join style, and open sub-path ends receive the selected
// cap style. The result is one or more closed sub-paths.
func Stroke(p Path, opts StrokeOptions) Path {
	if opts.Width <= 0 {
		return Path{}
	}
	if opts.MiterLimit <= 0 {
		opts.MiterLimit = 4.0
	}
	halfW := opts.Width / 2

	flat := Flatten(p, 0.25)
	var out PathBuilder

	// Collect sub-paths: each is a slice of points (x,y pairs) and a closed flag.
	type subPath struct {
		pts    []float32 // packed x,y pairs
		closed bool
	}
	var subs []subPath
	var cur subPath

	pi := 0
	for _, v := range flat.Verbs {
		switch v {
		case VerbMove:
			if len(cur.pts) > 0 {
				subs = append(subs, cur)
			}
			x, y := flat.Points[pi], flat.Points[pi+1]
			pi += 2
			cur = subPath{pts: []float32{x, y}}

		case VerbLine:
			x, y := flat.Points[pi], flat.Points[pi+1]
			pi += 2
			cur.pts = append(cur.pts, x, y)

		case VerbClose:
			cur.closed = true
			subs = append(subs, cur)
			cur = subPath{}
		}
	}
	if len(cur.pts) > 0 {
		subs = append(subs, cur)
	}

	for _, sp := range subs {
		strokeSubPath(&out, sp.pts, sp.closed, halfW, opts)
	}

	return out.Finish()
}

// strokeSubPath strokes a single sub-path (a sequence of connected line
// segments) and appends the resulting closed outline to out.
func strokeSubPath(out *PathBuilder, pts []float32, closed bool, halfW float32, opts StrokeOptions) {
	n := len(pts) / 2
	if n < 2 {
		return
	}

	// Remove duplicate last point for closed paths.
	if closed && n >= 2 && pts[0] == pts[(n-1)*2] && pts[1] == pts[(n-1)*2+1] {
		n--
		pts = pts[:n*2]
	}
	if n < 2 {
		return
	}

	numSegs := n - 1
	if closed {
		numSegs = n
	}

	// Pre-compute normals for each segment.
	type vec2 struct{ x, y float32 }
	normals := make([]vec2, numSegs)
	for i := 0; i < numSegs; i++ {
		j := (i + 1) % n
		dx := pts[j*2] - pts[i*2]
		dy := pts[j*2+1] - pts[i*2+1]
		l := float32(math.Sqrt(float64(dx*dx + dy*dy)))
		if l < 1e-10 {
			normals[i] = vec2{0, 0}
			continue
		}
		normals[i] = vec2{-dy / l * halfW, dx / l * halfW}
	}

	// Build the left (positive offset) and right (negative offset) outlines.
	left := make([]float32, 0, n*2)
	right := make([]float32, 0, n*2)

	appendJoin := func(dst *[]float32, side float32, px, py float32, nPrev, nCur vec2) {
		// nPrev and nCur are the normal * halfW for the previous and current segment.
		// side is +1 for left, -1 for right.
		n1x := side * nPrev.x
		n1y := side * nPrev.y
		n2x := side * nCur.x
		n2y := side * nCur.y

		switch opts.Join {
		case JoinBevel:
			*dst = append(*dst, px+n1x, py+n1y)
			*dst = append(*dst, px+n2x, py+n2y)

		case JoinRound:
			*dst = append(*dst, px+n1x, py+n1y)
			addRoundJoin(dst, px, py, n1x, n1y, n2x, n2y)
			*dst = append(*dst, px+n2x, py+n2y)

		case JoinMiter:
			// Compute miter point by intersecting the two offset lines.
			// If the miter length exceeds the limit, fall back to bevel.
			cross := n1x*n2y - n1y*n2x
			if math.Abs(float64(cross)) < 1e-10 {
				// Parallel or anti-parallel → bevel.
				*dst = append(*dst, px+n1x, py+n1y)
				*dst = append(*dst, px+n2x, py+n2y)
			} else {
				// Miter point: intersection of the two offset edges.
				// Using the formula for line-line intersection.
				mx := n1x + n2x
				my := n1y + n2y
				dot := mx*mx + my*my
				miterLen := float32(math.Sqrt(float64(dot)))
				if miterLen > opts.MiterLimit*halfW {
					// Exceed limit → bevel.
					*dst = append(*dst, px+n1x, py+n1y)
					*dst = append(*dst, px+n2x, py+n2y)
				} else {
					// Scale miter vector to correct length.
					denom := float32(1) + (n1x*n2x+n1y*n2y)/(halfW*halfW)
					if math.Abs(float64(denom)) < 1e-10 {
						*dst = append(*dst, px+n1x, py+n1y)
						*dst = append(*dst, px+n2x, py+n2y)
					} else {
						scale := float32(1.0) / denom
						miterX := (n1x + n2x) * scale
						miterY := (n1y + n2y) * scale
						*dst = append(*dst, px+miterX, py+miterY)
					}
				}
			}
		}
	}

	for i := 0; i < n; i++ {
		px, py := pts[i*2], pts[i*2+1]

		if closed {
			// Every vertex has a predecessor and successor segment.
			prevSeg := (i - 1 + numSegs) % numSegs
			curSeg := i % numSegs
			appendJoin(&left, 1, px, py, normals[prevSeg], normals[curSeg])
			appendJoin(&right, -1, px, py, normals[prevSeg], normals[curSeg])
		} else {
			if i == 0 {
				// First point: no join, just offset by first segment's normal.
				left = append(left, px+normals[0].x, py+normals[0].y)
				right = append(right, px-normals[0].x, py-normals[0].y)
			} else if i == n-1 {
				// Last point: no join, just offset by last segment's normal.
				lastSeg := numSegs - 1
				left = append(left, px+normals[lastSeg].x, py+normals[lastSeg].y)
				right = append(right, px-normals[lastSeg].x, py-normals[lastSeg].y)
			} else {
				appendJoin(&left, 1, px, py, normals[i-1], normals[i])
				appendJoin(&right, -1, px, py, normals[i-1], normals[i])
			}
		}
	}

	if closed {
		// Closed path: left outline forward, right outline reversed, both closed.
		out.MoveTo(left[0], left[1])
		for i := 2; i < len(left); i += 2 {
			out.LineTo(left[i], left[i+1])
		}
		out.Close()

		out.MoveTo(right[len(right)-2], right[len(right)-1])
		for i := len(right) - 4; i >= 0; i -= 2 {
			out.LineTo(right[i], right[i+1])
		}
		out.Close()
	} else {
		// Open path: left forward → end cap → right reversed → start cap → close.
		out.MoveTo(left[0], left[1])
		for i := 2; i < len(left); i += 2 {
			out.LineTo(left[i], left[i+1])
		}
		// End cap.
		addCap(out, opts.Cap, halfW,
			pts[(n-1)*2], pts[(n-1)*2+1],
			left[len(left)-2], left[len(left)-1],
			right[len(right)-2], right[len(right)-1],
		)
		// Right outline reversed.
		for i := len(right) - 2; i >= 0; i -= 2 {
			out.LineTo(right[i], right[i+1])
		}
		// Start cap.
		addCap(out, opts.Cap, halfW,
			pts[0], pts[1],
			right[0], right[1],
			left[0], left[1],
		)
		out.Close()
	}
}

// addCap appends cap geometry from pFrom to pTo around the path endpoint (cx, cy).
func addCap(out *PathBuilder, cap CapStyle, halfW float32, cx, cy, fromX, fromY, toX, toY float32) {
	switch cap {
	case CapButt:
		out.LineTo(toX, toY)

	case CapSquare:
		// Extend by halfW in the direction of the last segment.
		dx := fromX - cx
		dy := fromY - cy
		// The perpendicular is the forward direction rotated 90°.
		// Actually, the forward direction at the endpoint can be deduced:
		// normal goes from center to left; forward is 90° CW from normal.
		fwdX := dy  // forward direction (from perpendicular)
		fwdY := -dx // note: this is the outward direction at the cap
		l := float32(math.Sqrt(float64(fwdX*fwdX + fwdY*fwdY)))
		if l > 1e-10 {
			fwdX /= l
			fwdY /= l
		}
		// We want to extend outward, so check direction. For the end cap,
		// fwd should point away from the path.
		out.LineTo(fromX+fwdX*halfW, fromY+fwdY*halfW)
		out.LineTo(toX+fwdX*halfW, toY+fwdY*halfW)
		out.LineTo(toX, toY)

	case CapRound:
		// Semicircle from fromX,fromY to toX,toY around cx,cy.
		out.ArcTo(halfW, halfW, 0, false, true,
			cx-(fromX-cx), cy-(fromY-cy))
		out.ArcTo(halfW, halfW, 0, false, true, toX, toY)
	}
}

// addRoundJoin appends points forming a circular arc between two normal
// directions on the given side.
func addRoundJoin(dst *[]float32, cx, cy, n1x, n1y, n2x, n2y float32) {
	// Angle between the two normals.
	angle1 := math.Atan2(float64(n1y), float64(n1x))
	angle2 := math.Atan2(float64(n2y), float64(n2x))
	diff := angle2 - angle1

	// Normalize to [-π, π].
	for diff > math.Pi {
		diff -= 2 * math.Pi
	}
	for diff < -math.Pi {
		diff += 2 * math.Pi
	}

	radius := float32(math.Sqrt(float64(n1x*n1x + n1y*n1y)))
	if radius < 1e-10 {
		return
	}

	// Number of segments for the arc (more for bigger angles).
	steps := int(math.Ceil(math.Abs(diff) / (math.Pi / 4)))
	if steps < 1 {
		steps = 1
	}
	stepAngle := diff / float64(steps)

	for i := 1; i < steps; i++ {
		a := angle1 + float64(i)*stepAngle
		px := cx + radius*float32(math.Cos(a))
		py := cy + radius*float32(math.Sin(a))
		*dst = append(*dst, px, py)
	}
}
