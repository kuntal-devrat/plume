package geom

import "math"

// Flatten converts all quadratic and cubic Bézier segments in p into sequences
// of line segments using recursive De Casteljau subdivision. The returned path
// contains only [VerbMove], [VerbLine], and [VerbClose] verbs.
//
// tolerance is the maximum allowed deviation in pixels between the true curve
// and its linear approximation. A typical screen-resolution value is 0.25.
// Values ≤ 0 are treated as 0.25.
func Flatten(p Path, tolerance float32) Path {
	if tolerance <= 0 {
		tolerance = 0.25
	}
	tolSq := float64(tolerance) * float64(tolerance)

	var b PathBuilder
	var curX, curY float32
	pi := 0 // index into p.Points

	for _, v := range p.Verbs {
		switch v {
		case VerbMove:
			x, y := p.Points[pi], p.Points[pi+1]
			pi += 2
			b.MoveTo(x, y)
			curX, curY = x, y

		case VerbLine:
			x, y := p.Points[pi], p.Points[pi+1]
			pi += 2
			b.LineTo(x, y)
			curX, curY = x, y

		case VerbQuad:
			cx, cy := p.Points[pi], p.Points[pi+1]
			x, y := p.Points[pi+2], p.Points[pi+3]
			pi += 4
			flattenQuad(&b, tolSq,
				float64(curX), float64(curY),
				float64(cx), float64(cy),
				float64(x), float64(y))
			curX, curY = x, y

		case VerbCubic:
			c1x, c1y := p.Points[pi], p.Points[pi+1]
			c2x, c2y := p.Points[pi+2], p.Points[pi+3]
			x, y := p.Points[pi+4], p.Points[pi+5]
			pi += 6
			flattenCubic(&b, tolSq,
				float64(curX), float64(curY),
				float64(c1x), float64(c1y),
				float64(c2x), float64(c2y),
				float64(x), float64(y))
			curX, curY = x, y

		case VerbClose:
			b.Close()
			curX, curY = b.startX, b.startY
		}
	}
	return b.Finish()
}

// flattenQuad recursively subdivides a quadratic Bézier until the control point
// is within tolSq (squared tolerance) of the baseline.
func flattenQuad(b *PathBuilder, tolSq float64, x0, y0, cx, cy, x1, y1 float64) {
	// Distance from control point to midpoint of the baseline estimates error.
	mx := (x0 + x1) / 2
	my := (y0 + y1) / 2
	dx := cx - mx
	dy := cy - my
	if dx*dx+dy*dy <= tolSq {
		b.LineTo(float32(x1), float32(y1))
		return
	}
	// De Casteljau split at t = 0.5.
	q0x := (x0 + cx) / 2
	q0y := (y0 + cy) / 2
	q1x := (cx + x1) / 2
	q1y := (cy + y1) / 2
	midX := (q0x + q1x) / 2
	midY := (q0y + q1y) / 2

	flattenQuad(b, tolSq, x0, y0, q0x, q0y, midX, midY)
	flattenQuad(b, tolSq, midX, midY, q1x, q1y, x1, y1)
}

// flattenCubic recursively subdivides a cubic Bézier until both control points
// are within tolSq of the baseline.
func flattenCubic(b *PathBuilder, tolSq float64, x0, y0, c1x, c1y, c2x, c2y, x1, y1 float64) {
	// Maximum distance from either control point to the line x0→x1.
	d1 := pointToSegDistSq(c1x, c1y, x0, y0, x1, y1)
	d2 := pointToSegDistSq(c2x, c2y, x0, y0, x1, y1)
	if d1 <= tolSq && d2 <= tolSq {
		b.LineTo(float32(x1), float32(y1))
		return
	}
	// De Casteljau split at t = 0.5.
	m01x := (x0 + c1x) / 2
	m01y := (y0 + c1y) / 2
	m12x := (c1x + c2x) / 2
	m12y := (c1y + c2y) / 2
	m23x := (c2x + x1) / 2
	m23y := (c2y + y1) / 2

	ma01x := (m01x + m12x) / 2
	ma01y := (m01y + m12y) / 2
	ma12x := (m12x + m23x) / 2
	ma12y := (m12y + m23y) / 2

	midX := (ma01x + ma12x) / 2
	midY := (ma01y + ma12y) / 2

	flattenCubic(b, tolSq, x0, y0, m01x, m01y, ma01x, ma01y, midX, midY)
	flattenCubic(b, tolSq, midX, midY, ma12x, ma12y, m23x, m23y, x1, y1)
}

// pointToSegDistSq returns the squared distance from point (px, py) to the line
// segment (ax, ay)→(bx, by).
func pointToSegDistSq(px, py, ax, ay, bx, by float64) float64 {
	abx := bx - ax
	aby := by - ay
	lenSq := abx*abx + aby*aby
	if lenSq == 0 {
		dx := px - ax
		dy := py - ay
		return dx*dx + dy*dy
	}
	t := ((px-ax)*abx + (py-ay)*aby) / lenSq
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	cx := ax + t*abx
	cy := ay + t*aby
	dx := px - cx
	dy := py - cy
	return dx*dx + dy*dy
}

// BoundingBox computes the tight axis-aligned bounding box of p.
//
// For line segments the extrema are simply the endpoints. For quadratic and
// cubic Bézier curves the derivative is computed and its real roots in (0,1)
// are evaluated to find the true extrema.
//
// If the path has no points, all four return values are zero.
func BoundingBox(p Path) (minX, minY, maxX, maxY float32) {
	if len(p.Points) == 0 {
		return 0, 0, 0, 0
	}

	minX = float32(math.MaxFloat32)
	minY = float32(math.MaxFloat32)
	maxX = -float32(math.MaxFloat32)
	maxY = -float32(math.MaxFloat32)

	expandf := func(x, y float32) {
		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
	}
	expand := func(x, y float64) {
		expandf(float32(x), float32(y))
	}

	var curX, curY float32
	pi := 0

	for _, v := range p.Verbs {
		switch v {
		case VerbMove:
			curX, curY = p.Points[pi], p.Points[pi+1]
			pi += 2
			expandf(curX, curY)

		case VerbLine:
			curX, curY = p.Points[pi], p.Points[pi+1]
			pi += 2
			expandf(curX, curY)

		case VerbQuad:
			cx, cy := float64(p.Points[pi]), float64(p.Points[pi+1])
			x, y := float64(p.Points[pi+2]), float64(p.Points[pi+3])
			pi += 4
			expandf(float32(x), float32(y))
			// Derivative roots for x: d/dt [(1-t)²x0 + 2t(1-t)cx + t²x]
			//   = 2(cx-x0) + 2(x0 - 2cx + x)t = 0
			//   → t = (x0 - cx) / (x0 - 2cx + x)
			bbQuadExtrema(float64(curX), cx, x, expand, true)
			bbQuadExtrema(float64(curY), cy, y, expand, false)
			curX, curY = float32(x), float32(y)

		case VerbCubic:
			c1x, c1y := float64(p.Points[pi]), float64(p.Points[pi+1])
			c2x, c2y := float64(p.Points[pi+2]), float64(p.Points[pi+3])
			x, y := float64(p.Points[pi+4]), float64(p.Points[pi+5])
			pi += 6
			expandf(float32(x), float32(y))
			bbCubicExtrema(float64(curX), c1x, c2x, x, expand, true)
			bbCubicExtrema(float64(curY), c1y, c2y, y, expand, false)
			curX, curY = float32(x), float32(y)

		case VerbClose:
			// No points consumed.
		}
	}
	return
}

// bbQuadExtrema finds the parameter t ∈ (0,1) at which the quadratic
// B(t) = (1−t)²p0 + 2t(1−t)p1 + t²p2 reaches an extremum in the given
// axis, and expands the bounding box with that value.
func bbQuadExtrema(p0, p1, p2 float64, expand func(float64, float64), isX bool) {
	denom := p0 - 2*p1 + p2
	if denom == 0 {
		return
	}
	t := (p0 - p1) / denom
	if t <= 0 || t >= 1 {
		return
	}
	v := (1-t)*(1-t)*p0 + 2*(1-t)*t*p1 + t*t*p2
	if isX {
		expand(v, 0) // y doesn't matter, only x is used
	} else {
		expand(0, v)
	}
}

// bbCubicExtrema finds the parameters t ∈ (0,1) at which the cubic
// B(t) = (1−t)³p0 + 3t(1−t)²p1 + 3t²(1−t)p2 + t³p3 reaches extrema,
// and expands the bounding box.
func bbCubicExtrema(p0, p1, p2, p3 float64, expand func(float64, float64), isX bool) {
	// Derivative: 3[(p1-p0)(1-t)² + 2(p2-p1)(1-t)t + (p3-p2)t²]
	// Simplify to at² + bt + c = 0 where:
	a := -p0 + 3*p1 - 3*p2 + p3
	b := 2*p0 - 4*p1 + 2*p2
	c := p1 - p0

	evalAndExpand := func(t float64) {
		if t <= 0 || t >= 1 {
			return
		}
		u := 1 - t
		v := u*u*u*p0 + 3*u*u*t*p1 + 3*u*t*t*p2 + t*t*t*p3
		if isX {
			expand(v, 0)
		} else {
			expand(0, v)
		}
	}

	if math.Abs(a) < 1e-12 {
		// Linear derivative.
		if math.Abs(b) > 1e-12 {
			evalAndExpand(-c / b)
		}
		return
	}

	disc := b*b - 4*a*c
	if disc < 0 {
		return
	}
	sqrtDisc := math.Sqrt(disc)
	evalAndExpand((-b + sqrtDisc) / (2 * a))
	evalAndExpand((-b - sqrtDisc) / (2 * a))
}
