// Package geom provides a pure-Go 2D vector geometry library for the Plume engine.
//
// It includes path construction, flattening, stroking, dashing, and affine
// transforms — everything needed to prepare vector graphics for rasterization.
// All coordinates use float32 for GPU friendliness, and the Path type uses a
// Structure-of-Arrays layout (separate verb and point slices) for cache-friendly
// iteration.
package geom

import "math"

// PathVerb represents a path drawing command.
type PathVerb uint8

const (
	// VerbMove starts a new sub-path at (x, y). Consumes 2 floats.
	VerbMove PathVerb = iota
	// VerbLine draws a straight line to (x, y). Consumes 2 floats.
	VerbLine
	// VerbQuad draws a quadratic Bézier to (x, y) with control point (cx, cy).
	// Consumes 4 floats: cx, cy, x, y.
	VerbQuad
	// VerbCubic draws a cubic Bézier to (x, y) with control points (c1x, c1y)
	// and (c2x, c2y). Consumes 6 floats: c1x, c1y, c2x, c2y, x, y.
	VerbCubic
	// VerbClose closes the current sub-path back to the last MoveTo.
	// Consumes 0 floats.
	VerbClose
)

// verbPointCounts maps each PathVerb to the number of float32 values it consumes
// from the Points slice.
var verbPointCounts = [5]int{2, 2, 4, 6, 0}

// PointCount returns the number of float32 values this verb consumes from the
// Points slice.
func (v PathVerb) PointCount() int {
	if int(v) < len(verbPointCounts) {
		return verbPointCounts[v]
	}
	return 0
}

// Path is the core 2D path data structure. It uses a Structure-of-Arrays layout
// for cache-friendly iteration by the rasterizer: verbs and point data live in
// separate slices so that a verb-scanning loop touches only compact memory.
type Path struct {
	// Verbs is the sequence of drawing commands.
	Verbs []PathVerb
	// Points is the packed coordinate data referenced by the verbs.
	// Each verb consumes a fixed number of floats (see PathVerb docs).
	Points []float32
}

// PathBuilder accumulates path commands and produces an immutable Path.
// A zero-value PathBuilder is ready to use.
type PathBuilder struct {
	verbs  []PathVerb
	points []float32
	// startX, startY track the position of the most recent MoveTo so that
	// Close can emit the correct coordinates.
	startX, startY float32
	// curX, curY track the current pen position for arc conversion.
	curX, curY float32
}

// MoveTo starts a new sub-path at (x, y).
func (b *PathBuilder) MoveTo(x, y float32) {
	b.verbs = append(b.verbs, VerbMove)
	b.points = append(b.points, x, y)
	b.startX, b.startY = x, y
	b.curX, b.curY = x, y
}

// LineTo appends a straight line segment to (x, y).
func (b *PathBuilder) LineTo(x, y float32) {
	b.verbs = append(b.verbs, VerbLine)
	b.points = append(b.points, x, y)
	b.curX, b.curY = x, y
}

// QuadTo appends a quadratic Bézier curve with control point (cx, cy) and
// endpoint (x, y).
func (b *PathBuilder) QuadTo(cx, cy, x, y float32) {
	b.verbs = append(b.verbs, VerbQuad)
	b.points = append(b.points, cx, cy, x, y)
	b.curX, b.curY = x, y
}

// CubicTo appends a cubic Bézier curve with control points (c1x, c1y),
// (c2x, c2y) and endpoint (x, y).
func (b *PathBuilder) CubicTo(c1x, c1y, c2x, c2y, x, y float32) {
	b.verbs = append(b.verbs, VerbCubic)
	b.points = append(b.points, c1x, c1y, c2x, c2y, x, y)
	b.curX, b.curY = x, y
}

// ArcTo appends an SVG-style elliptical arc from the current point to (x, y).
// The arc is defined by radii (rx, ry), an x-axis rotation in radians, and the
// largeArc and sweep flags. Internally the arc is converted to one or more
// cubic Bézier curves using the standard endpoint-to-center parameterization
// (see SVG spec §F.6).
func (b *PathBuilder) ArcTo(rx, ry, xRotation float32, largeArc, sweep bool, x, y float32) {
	// Degenerate: current point equals target — nothing to draw.
	if b.curX == x && b.curY == y {
		return
	}
	// Degenerate: zero radii — treat as line.
	if rx == 0 || ry == 0 {
		b.LineTo(x, y)
		return
	}
	// Ensure positive radii.
	if rx < 0 {
		rx = -rx
	}
	if ry < 0 {
		ry = -ry
	}
	arcToCubics(b, float64(b.curX), float64(b.curY), float64(rx), float64(ry),
		float64(xRotation), largeArc, sweep, float64(x), float64(y))
	b.curX, b.curY = x, y
}

// arcToCubics performs the endpoint-to-center arc parameterization and emits
// cubic Bézier approximations into the builder.
func arcToCubics(b *PathBuilder, x1, y1, rxIn, ryIn, phi float64, fA, fS bool, x2, y2 float64) {
	cosPhi := math.Cos(phi)
	sinPhi := math.Sin(phi)

	// Step 1: compute (x1', y1') — rotated midpoint.
	dx := (x1 - x2) / 2
	dy := (y1 - y2) / 2
	x1p := cosPhi*dx + sinPhi*dy
	y1p := -sinPhi*dx + cosPhi*dy

	// Step 2: correct out-of-range radii.
	rx, ry := rxIn, ryIn
	x1pSq := x1p * x1p
	y1pSq := y1p * y1p
	rxSq := rx * rx
	rySq := ry * ry

	lambda := x1pSq/rxSq + y1pSq/rySq
	if lambda > 1 {
		s := math.Sqrt(lambda)
		rx *= s
		ry *= s
		rxSq = rx * rx
		rySq = ry * ry
	}

	// Step 3: compute center (cx', cy').
	num := rxSq*rySq - rxSq*y1pSq - rySq*x1pSq
	den := rxSq*y1pSq + rySq*x1pSq
	if den == 0 {
		b.LineTo(float32(x2), float32(y2))
		return
	}
	sq := num / den
	if sq < 0 {
		sq = 0
	}
	root := math.Sqrt(sq)
	if fA == fS {
		root = -root
	}
	cxp := root * rx * y1p / ry
	cyp := -root * ry * x1p / rx

	// Step 4: compute center (cx, cy) in original coords.
	cx := cosPhi*cxp - sinPhi*cyp + (x1+x2)/2
	cy := sinPhi*cxp + cosPhi*cyp + (y1+y2)/2

	// Step 5: compute start angle and sweep angle.
	ux := (x1p - cxp) / rx
	uy := (y1p - cyp) / ry
	vx := (-x1p - cxp) / rx
	vy := (-y1p - cyp) / ry

	theta1 := vecAngle(1, 0, ux, uy)
	dTheta := vecAngle(ux, uy, vx, vy)

	if !fS && dTheta > 0 {
		dTheta -= 2 * math.Pi
	} else if fS && dTheta < 0 {
		dTheta += 2 * math.Pi
	}

	// Step 6: split into segments of at most π/2 and approximate each with a cubic.
	segments := int(math.Ceil(math.Abs(dTheta) / (math.Pi / 2)))
	if segments == 0 {
		segments = 1
	}
	segAngle := dTheta / float64(segments)

	for i := 0; i < segments; i++ {
		t1 := theta1 + float64(i)*segAngle
		t2 := t1 + segAngle
		emitArcSegment(b, cx, cy, rx, ry, cosPhi, sinPhi, t1, t2)
	}
}

// vecAngle returns the angle (in radians) between vectors (ux, uy) and (vx, vy).
func vecAngle(ux, uy, vx, vy float64) float64 {
	dot := ux*vx + uy*vy
	lenU := math.Sqrt(ux*ux + uy*uy)
	lenV := math.Sqrt(vx*vx + vy*vy)
	cos := dot / (lenU * lenV)
	// Clamp for numerical safety.
	if cos < -1 {
		cos = -1
	} else if cos > 1 {
		cos = 1
	}
	angle := math.Acos(cos)
	if ux*vy-uy*vx < 0 {
		angle = -angle
	}
	return angle
}

// emitArcSegment approximates a single arc segment [t1, t2] with a cubic
// Bézier and appends it to the builder.
func emitArcSegment(b *PathBuilder, cx, cy, rx, ry, cosPhi, sinPhi, t1, t2 float64) {
	dt := t2 - t1
	alpha := math.Sin(dt) * (math.Sqrt(4+3*math.Pow(math.Tan(dt/2), 2)) - 1) / 3

	cos1 := math.Cos(t1)
	sin1 := math.Sin(t1)
	cos2 := math.Cos(t2)
	sin2 := math.Sin(t2)

	// Endpoint on the unit-radius arc, then scale by rx, ry.
	ex1 := rx * cos1
	ey1 := ry * sin1
	ex2 := rx * cos2
	ey2 := ry * sin2

	// Derivatives at endpoints.
	dx1 := -rx * sin1
	dy1 := ry * cos1
	dx2 := -rx * sin2
	dy2 := ry * cos2

	// Control points before rotation/translation.
	cp1x := ex1 + alpha*dx1
	cp1y := ey1 + alpha*dy1
	cp2x := ex2 - alpha*dx2
	cp2y := ey2 - alpha*dy2

	// Rotate and translate.
	rotTransX := func(px, py float64) float32 {
		return float32(cosPhi*px - sinPhi*py + cx)
	}
	rotTransY := func(px, py float64) float32 {
		return float32(sinPhi*px + cosPhi*py + cy)
	}

	b.CubicTo(
		rotTransX(cp1x, cp1y), rotTransY(cp1x, cp1y),
		rotTransX(cp2x, cp2y), rotTransY(cp2x, cp2y),
		rotTransX(ex2, ey2), rotTransY(ex2, ey2),
	)
}

// Close closes the current sub-path by drawing a straight line back to the
// last MoveTo position.
func (b *PathBuilder) Close() {
	b.verbs = append(b.verbs, VerbClose)
	b.curX, b.curY = b.startX, b.startY
}

// Finish returns the built [Path] and resets the builder for reuse.
func (b *PathBuilder) Finish() Path {
	p := Path{
		Verbs:  b.verbs,
		Points: b.points,
	}
	b.verbs = nil
	b.points = nil
	b.startX, b.startY = 0, 0
	b.curX, b.curY = 0, 0
	return p
}

// RoundedRect returns a [Path] for a rectangle with uniformly rounded corners.
// The rectangle's top-left corner is at (x, y) with size (w, h). The corner
// radius is clamped so it never exceeds half the shorter side.
func RoundedRect(x, y, w, h, radius float32) Path {
	// Clamp radius.
	maxR := w / 2
	if h/2 < maxR {
		maxR = h / 2
	}
	if radius > maxR {
		radius = maxR
	}
	if radius <= 0 {
		return Rect(x, y, w, h)
	}

	var b PathBuilder

	// Start at top edge, past top-left arc.
	b.MoveTo(x+radius, y)

	// Top-right corner.
	b.LineTo(x+w-radius, y)
	b.ArcTo(radius, radius, 0, false, true, x+w, y+radius)

	// Bottom-right corner.
	b.LineTo(x+w, y+h-radius)
	b.ArcTo(radius, radius, 0, false, true, x+w-radius, y+h)

	// Bottom-left corner.
	b.LineTo(x+radius, y+h)
	b.ArcTo(radius, radius, 0, false, true, x, y+h-radius)

	// Top-left corner.
	b.LineTo(x, y+radius)
	b.ArcTo(radius, radius, 0, false, true, x+radius, y)

	b.Close()
	return b.Finish()
}

// Circle returns a [Path] approximating a circle centered at (cx, cy) with the
// given radius. The circle is constructed from four cubic Bézier curves using
// the standard kappa constant (4*(√2−1)/3 ≈ 0.5522847498).
func Circle(cx, cy, radius float32) Path {
	// kappa: the magic constant for circular arc approximation with cubics.
	const kappa = float32(0.5522847498)
	k := radius * kappa

	var b PathBuilder
	// Start at rightmost point.
	b.MoveTo(cx+radius, cy)
	// Bottom-right quadrant.
	b.CubicTo(cx+radius, cy+k, cx+k, cy+radius, cx, cy+radius)
	// Bottom-left quadrant.
	b.CubicTo(cx-k, cy+radius, cx-radius, cy+k, cx-radius, cy)
	// Top-left quadrant.
	b.CubicTo(cx-radius, cy-k, cx-k, cy-radius, cx, cy-radius)
	// Top-right quadrant.
	b.CubicTo(cx+k, cy-radius, cx+radius, cy-k, cx+radius, cy)
	b.Close()
	return b.Finish()
}

// Rect returns a [Path] for a simple axis-aligned rectangle with top-left
// corner at (x, y) and dimensions (w, h).
func Rect(x, y, w, h float32) Path {
	var b PathBuilder
	b.MoveTo(x, y)
	b.LineTo(x+w, y)
	b.LineTo(x+w, y+h)
	b.LineTo(x, y+h)
	b.Close()
	return b.Finish()
}
