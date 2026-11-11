package geom

import "math"

// Affine represents a 2D affine transformation matrix:
//
//	[A  B  Tx]
//	[C  D  Ty]
//	[0  0   1]
//
// Points are transformed as:
//
//	x' = A*x + B*y + Tx
//	y' = C*x + D*y + Ty
type Affine struct {
	A, B, C, D float32 // rotation / scale / skew
	Tx, Ty     float32 // translation
}

// Identity returns the identity affine transform (no transformation).
func Identity() Affine {
	return Affine{A: 1, B: 0, C: 0, D: 1, Tx: 0, Ty: 0}
}

// Translate returns an affine transform that translates by (tx, ty).
func Translate(tx, ty float32) Affine {
	return Affine{A: 1, B: 0, C: 0, D: 1, Tx: tx, Ty: ty}
}

// Scale returns an affine transform that scales by (sx, sy) about the origin.
func Scale(sx, sy float32) Affine {
	return Affine{A: sx, B: 0, C: 0, D: sy, Tx: 0, Ty: 0}
}

// Rotate returns an affine transform that rotates counter-clockwise by the
// given angle in radians about the origin.
func Rotate(radians float32) Affine {
	c := float32(math.Cos(float64(radians)))
	s := float32(math.Sin(float64(radians)))
	return Affine{A: c, B: -s, C: s, D: c, Tx: 0, Ty: 0}
}

// Then returns the composition of a followed by b (i.e. b∘a): the resulting
// transform first applies a, then applies b.
//
//	result = b · a
func (a Affine) Then(b Affine) Affine {
	return Affine{
		A:  b.A*a.A + b.B*a.C,
		B:  b.A*a.B + b.B*a.D,
		C:  b.C*a.A + b.D*a.C,
		D:  b.C*a.B + b.D*a.D,
		Tx: b.A*a.Tx + b.B*a.Ty + b.Tx,
		Ty: b.C*a.Tx + b.D*a.Ty + b.Ty,
	}
}

// TransformPoint applies the affine transform to the point (x, y) and returns
// the transformed coordinates.
func (a Affine) TransformPoint(x, y float32) (float32, float32) {
	return a.A*x + a.B*y + a.Tx, a.C*x + a.D*y + a.Ty
}

// TransformPath applies the affine transform t to every point in p and returns
// a new [Path] with the transformed coordinates. The verb sequence is unchanged.
func TransformPath(p Path, t Affine) Path {
	newPoints := make([]float32, len(p.Points))
	newVerbs := make([]PathVerb, len(p.Verbs))
	copy(newVerbs, p.Verbs)

	pi := 0
	for _, v := range p.Verbs {
		count := v.PointCount()
		for j := 0; j < count; j += 2 {
			x, y := p.Points[pi+j], p.Points[pi+j+1]
			nx, ny := t.TransformPoint(x, y)
			newPoints[pi+j] = nx
			newPoints[pi+j+1] = ny
		}
		pi += count
	}

	return Path{Verbs: newVerbs, Points: newPoints}
}
