package geom

import (
	"math"
	"testing"
)

// ---------- helpers ----------

const epsilon = 1e-3

func approxEq(a, b, tol float32) bool {
	return math.Abs(float64(a-b)) <= float64(tol)
}

func requireApprox(t *testing.T, label string, got, want, tol float32) {
	t.Helper()
	if !approxEq(got, want, tol) {
		t.Errorf("%s: got %v, want ≈%v (±%v)", label, got, want, tol)
	}
}

// hasOnlyLineVerbs returns true if the path contains no Quad or Cubic verbs.
func hasOnlyLineVerbs(p Path) bool {
	for _, v := range p.Verbs {
		if v == VerbQuad || v == VerbCubic {
			return false
		}
	}
	return true
}

// ---------- PathBuilder ----------

func TestPathBuilderBasic(t *testing.T) {
	var b PathBuilder
	b.MoveTo(0, 0)
	b.LineTo(10, 0)
	b.LineTo(10, 10)
	b.Close()
	p := b.Finish()

	wantVerbs := []PathVerb{VerbMove, VerbLine, VerbLine, VerbClose}
	if len(p.Verbs) != len(wantVerbs) {
		t.Fatalf("verb count: got %d, want %d", len(p.Verbs), len(wantVerbs))
	}
	for i, v := range wantVerbs {
		if p.Verbs[i] != v {
			t.Errorf("verb[%d]: got %d, want %d", i, p.Verbs[i], v)
		}
	}

	// 2 + 2 + 2 = 6 points (MoveTo + LineTo + LineTo)
	if len(p.Points) != 6 {
		t.Fatalf("point count: got %d, want 6", len(p.Points))
	}

	wantPts := []float32{0, 0, 10, 0, 10, 10}
	for i, v := range wantPts {
		if p.Points[i] != v {
			t.Errorf("point[%d]: got %v, want %v", i, p.Points[i], v)
		}
	}
}

func TestPathBuilderQuadCubic(t *testing.T) {
	var b PathBuilder
	b.MoveTo(0, 0)
	b.QuadTo(5, 10, 10, 0)
	b.CubicTo(15, 10, 20, 10, 25, 0)
	p := b.Finish()

	wantVerbs := []PathVerb{VerbMove, VerbQuad, VerbCubic}
	if len(p.Verbs) != len(wantVerbs) {
		t.Fatalf("verb count: got %d, want %d", len(p.Verbs), len(wantVerbs))
	}
	for i, v := range wantVerbs {
		if p.Verbs[i] != v {
			t.Errorf("verb[%d]: got %d, want %d", i, p.Verbs[i], v)
		}
	}
	// 2 (Move) + 4 (Quad) + 6 (Cubic) = 12
	if len(p.Points) != 12 {
		t.Fatalf("point count: got %d, want 12", len(p.Points))
	}
}

func TestPathBuilderResets(t *testing.T) {
	var b PathBuilder
	b.MoveTo(1, 2)
	b.LineTo(3, 4)
	_ = b.Finish()

	// After Finish, builder is empty.
	b.MoveTo(10, 20)
	p := b.Finish()
	if len(p.Verbs) != 1 || p.Verbs[0] != VerbMove {
		t.Errorf("expected single MoveTo after reset, got %v", p.Verbs)
	}
}

// ---------- Convenience constructors ----------

func TestRect(t *testing.T) {
	p := Rect(10, 20, 100, 50)
	// Should be MoveTo + 3 LineTo + Close = 5 verbs
	if len(p.Verbs) != 5 {
		t.Fatalf("Rect verbs: got %d, want 5", len(p.Verbs))
	}
	if p.Verbs[0] != VerbMove || p.Verbs[4] != VerbClose {
		t.Error("Rect should start with Move and end with Close")
	}

	minX, minY, maxX, maxY := BoundingBox(p)
	requireApprox(t, "minX", minX, 10, epsilon)
	requireApprox(t, "minY", minY, 20, epsilon)
	requireApprox(t, "maxX", maxX, 110, epsilon)
	requireApprox(t, "maxY", maxY, 70, epsilon)
}

func TestCircle(t *testing.T) {
	p := Circle(50, 50, 25)
	// 4 cubics + MoveTo + Close.
	if len(p.Verbs) != 6 {
		t.Fatalf("Circle verbs: got %d, want 6", len(p.Verbs))
	}
	if p.Verbs[0] != VerbMove || p.Verbs[5] != VerbClose {
		t.Error("Circle should start with Move and end with Close")
	}
	for i := 1; i <= 4; i++ {
		if p.Verbs[i] != VerbCubic {
			t.Errorf("Circle verb[%d]: got %d, want VerbCubic", i, p.Verbs[i])
		}
	}

	minX, minY, maxX, maxY := BoundingBox(p)
	requireApprox(t, "minX", minX, 25, 0.5)
	requireApprox(t, "minY", minY, 25, 0.5)
	requireApprox(t, "maxX", maxX, 75, 0.5)
	requireApprox(t, "maxY", maxY, 75, 0.5)
}

func TestRoundedRect(t *testing.T) {
	p := RoundedRect(0, 0, 100, 60, 10)
	// Should contain arcs (cubics) and lines.
	hasCurve := false
	for _, v := range p.Verbs {
		if v == VerbCubic {
			hasCurve = true
			break
		}
	}
	if !hasCurve {
		t.Error("RoundedRect should contain cubic Bézier arcs")
	}

	minX, minY, maxX, maxY := BoundingBox(p)
	requireApprox(t, "minX", minX, 0, 0.5)
	requireApprox(t, "minY", minY, 0, 0.5)
	requireApprox(t, "maxX", maxX, 100, 0.5)
	requireApprox(t, "maxY", maxY, 60, 0.5)
}

func TestRoundedRectDegeneratesToRect(t *testing.T) {
	p := RoundedRect(10, 10, 50, 30, 0)
	// With zero radius, should be a plain rect with no cubics.
	for _, v := range p.Verbs {
		if v == VerbCubic || v == VerbQuad {
			t.Error("RoundedRect with radius=0 should not have curves")
			break
		}
	}
}

// ---------- Flatten ----------

func TestFlattenLines(t *testing.T) {
	// A path with only lines should be unchanged.
	p := Rect(0, 0, 10, 10)
	flat := Flatten(p, 0.25)
	if !hasOnlyLineVerbs(flat) {
		t.Error("Flatten of line-only path should contain no curves")
	}
	if len(flat.Verbs) != len(p.Verbs) {
		t.Errorf("Flatten of line-only path changed verb count: %d → %d",
			len(p.Verbs), len(flat.Verbs))
	}
}

func TestFlattenQuad(t *testing.T) {
	var b PathBuilder
	b.MoveTo(0, 0)
	b.QuadTo(50, 100, 100, 0)
	p := b.Finish()

	flat := Flatten(p, 0.25)
	if !hasOnlyLineVerbs(flat) {
		t.Error("Flatten should remove all Quad verbs")
	}
	if len(flat.Verbs) < 3 {
		t.Error("Flatten should produce multiple line segments for a quadratic")
	}
}

func TestFlattenCubic(t *testing.T) {
	var b PathBuilder
	b.MoveTo(0, 0)
	b.CubicTo(33, 100, 66, -100, 100, 0)
	p := b.Finish()

	flat := Flatten(p, 0.25)
	if !hasOnlyLineVerbs(flat) {
		t.Error("Flatten should remove all Cubic verbs")
	}
	if len(flat.Verbs) < 5 {
		t.Error("Flatten should produce many line segments for a complex cubic")
	}
}

func TestFlattenCircle(t *testing.T) {
	p := Circle(0, 0, 100)
	flat := Flatten(p, 0.1)
	if !hasOnlyLineVerbs(flat) {
		t.Error("Flatten(Circle) should contain only line verbs")
	}
	// A large circle flattened at tight tolerance should produce many segments.
	lineCount := 0
	for _, v := range flat.Verbs {
		if v == VerbLine {
			lineCount++
		}
	}
	if lineCount < 20 {
		t.Errorf("expected many line segments for flattened circle, got %d", lineCount)
	}
}

func TestFlattenDefaultTolerance(t *testing.T) {
	var b PathBuilder
	b.MoveTo(0, 0)
	b.QuadTo(50, 100, 100, 0)
	p := b.Finish()

	flat := Flatten(p, 0) // 0 should use default 0.25
	if !hasOnlyLineVerbs(flat) {
		t.Error("Flatten with 0 tolerance should still work")
	}
}

// ---------- BoundingBox ----------

func TestBoundingBoxRect(t *testing.T) {
	p := Rect(5, 10, 20, 30)
	minX, minY, maxX, maxY := BoundingBox(p)
	requireApprox(t, "minX", minX, 5, epsilon)
	requireApprox(t, "minY", minY, 10, epsilon)
	requireApprox(t, "maxX", maxX, 25, epsilon)
	requireApprox(t, "maxY", maxY, 40, epsilon)
}

func TestBoundingBoxQuad(t *testing.T) {
	// Quadratic where the control point extends beyond endpoints.
	var b PathBuilder
	b.MoveTo(0, 0)
	b.QuadTo(50, 100, 100, 0) // peak at y ≈ 50
	p := b.Finish()

	minX, minY, maxX, maxY := BoundingBox(p)
	requireApprox(t, "minX", minX, 0, epsilon)
	requireApprox(t, "maxX", maxX, 100, epsilon)
	requireApprox(t, "minY", minY, 0, epsilon)
	// The maximum y of a quadratic with p0=(0,0), p1=(50,100), p2=(100,0)
	// is at t=0.5 → y = 0.25*0 + 0.5*100 + 0.25*0 = 50
	requireApprox(t, "maxY", maxY, 50, 0.5)
}

func TestBoundingBoxCubic(t *testing.T) {
	var b PathBuilder
	b.MoveTo(0, 0)
	b.CubicTo(0, 100, 100, -100, 100, 0) // S-curve
	p := b.Finish()

	minX, minY, maxX, maxY := BoundingBox(p)
	requireApprox(t, "minX", minX, 0, epsilon)
	requireApprox(t, "maxX", maxX, 100, epsilon)
	// Should extend above and below the line y=0.
	if maxY <= 0 {
		t.Error("BoundingBox should detect positive Y extremum")
	}
	if minY >= 0 {
		t.Error("BoundingBox should detect negative Y extremum")
	}
}

func TestBoundingBoxEmpty(t *testing.T) {
	p := Path{}
	minX, minY, maxX, maxY := BoundingBox(p)
	if minX != 0 || minY != 0 || maxX != 0 || maxY != 0 {
		t.Error("BoundingBox of empty path should be all zeros")
	}
}

// ---------- ArcTo ----------

func TestArcToProducesCubics(t *testing.T) {
	var b PathBuilder
	b.MoveTo(0, 0)
	b.ArcTo(50, 50, 0, false, true, 100, 0)
	p := b.Finish()

	hasCubic := false
	for _, v := range p.Verbs {
		if v == VerbCubic {
			hasCubic = true
		}
	}
	if !hasCubic {
		t.Error("ArcTo should produce cubic Bézier approximations")
	}

	// The endpoint of the arc should be (100, 0).
	lastX := p.Points[len(p.Points)-2]
	lastY := p.Points[len(p.Points)-1]
	requireApprox(t, "arc end X", lastX, 100, 0.5)
	requireApprox(t, "arc end Y", lastY, 0, 0.5)
}

func TestArcToDegenerate(t *testing.T) {
	// Zero radii → lineTo.
	var b PathBuilder
	b.MoveTo(0, 0)
	b.ArcTo(0, 0, 0, false, true, 10, 0)
	p := b.Finish()

	if len(p.Verbs) != 2 || p.Verbs[1] != VerbLine {
		t.Error("ArcTo with zero radii should produce a LineTo")
	}
}

func TestArcToSamePoint(t *testing.T) {
	var b PathBuilder
	b.MoveTo(10, 10)
	b.ArcTo(5, 5, 0, false, true, 10, 10) // same point
	p := b.Finish()
	// Should be just the MoveTo.
	if len(p.Verbs) != 1 {
		t.Error("ArcTo to same point should produce no additional verbs")
	}
}

// ---------- Stroke ----------

func TestStrokeRect(t *testing.T) {
	p := Rect(0, 0, 100, 100)
	stroked := Stroke(p, StrokeOptions{Width: 10, Cap: CapButt, Join: JoinMiter})

	if len(stroked.Verbs) == 0 {
		t.Fatal("Stroke should produce output")
	}

	// Stroked closed path should itself be closed.
	hasClose := false
	for _, v := range stroked.Verbs {
		if v == VerbClose {
			hasClose = true
			break
		}
	}
	if !hasClose {
		t.Error("Stroke of closed path should produce closed output")
	}
}

func TestStrokeOpenLine(t *testing.T) {
	var b PathBuilder
	b.MoveTo(0, 0)
	b.LineTo(100, 0)
	p := b.Finish()

	for _, cap := range []CapStyle{CapButt, CapRound, CapSquare} {
		stroked := Stroke(p, StrokeOptions{Width: 10, Cap: cap, Join: JoinBevel})
		if len(stroked.Verbs) == 0 {
			t.Errorf("Stroke with cap=%d should produce output", cap)
		}
		// Should be a closed outline.
		hasClose := false
		for _, v := range stroked.Verbs {
			if v == VerbClose {
				hasClose = true
			}
		}
		if !hasClose {
			t.Errorf("Stroke with cap=%d should produce closed outline", cap)
		}
	}
}

func TestStrokeJoinStyles(t *testing.T) {
	var b PathBuilder
	b.MoveTo(0, 0)
	b.LineTo(50, 50)
	b.LineTo(100, 0)
	p := b.Finish()

	for _, join := range []JoinStyle{JoinMiter, JoinRound, JoinBevel} {
		stroked := Stroke(p, StrokeOptions{Width: 10, Cap: CapButt, Join: join})
		if len(stroked.Verbs) == 0 {
			t.Errorf("Stroke with join=%d should produce output", join)
		}
	}
}

func TestStrokeZeroWidth(t *testing.T) {
	p := Rect(0, 0, 10, 10)
	stroked := Stroke(p, StrokeOptions{Width: 0})
	if len(stroked.Verbs) != 0 {
		t.Error("Stroke with zero width should produce empty path")
	}
}

// ---------- Affine transforms ----------

func TestIdentity(t *testing.T) {
	id := Identity()
	x, y := id.TransformPoint(42, 99)
	requireApprox(t, "x", x, 42, epsilon)
	requireApprox(t, "y", y, 99, epsilon)
}

func TestTranslate(t *testing.T) {
	tr := Translate(10, 20)
	x, y := tr.TransformPoint(5, 5)
	requireApprox(t, "x", x, 15, epsilon)
	requireApprox(t, "y", y, 25, epsilon)
}

func TestScale(t *testing.T) {
	sc := Scale(2, 3)
	x, y := sc.TransformPoint(10, 10)
	requireApprox(t, "x", x, 20, epsilon)
	requireApprox(t, "y", y, 30, epsilon)
}

func TestRotate90(t *testing.T) {
	rot := Rotate(math.Pi / 2)
	x, y := rot.TransformPoint(1, 0)
	requireApprox(t, "x", x, 0, epsilon)
	requireApprox(t, "y", y, 1, epsilon)
}

func TestRotate180(t *testing.T) {
	rot := Rotate(math.Pi)
	x, y := rot.TransformPoint(1, 0)
	requireApprox(t, "x", x, -1, epsilon)
	requireApprox(t, "y", y, 0, epsilon)
}

func TestAffineCompose(t *testing.T) {
	// Scale(2,2).Then(Translate(10,0)) → first scale, then translate.
	composed := Scale(2, 2).Then(Translate(10, 0))
	x, y := composed.TransformPoint(5, 3)
	// First scale: (10, 6), then translate: (20, 6)
	requireApprox(t, "x", x, 20, epsilon)
	requireApprox(t, "y", y, 6, epsilon)
}

func TestAffineComposeOrder(t *testing.T) {
	// Translate then scale should differ from scale then translate.
	ts := Translate(10, 0).Then(Scale(2, 2))
	st := Scale(2, 2).Then(Translate(10, 0))

	x1, y1 := ts.TransformPoint(5, 0)
	x2, y2 := st.TransformPoint(5, 0)

	// ts: translate(5,0)→(15,0), scale→(30,0)
	requireApprox(t, "ts.x", x1, 30, epsilon)
	requireApprox(t, "ts.y", y1, 0, epsilon)

	// st: scale(5,0)→(10,0), translate→(20,0)
	requireApprox(t, "st.x", x2, 20, epsilon)
	requireApprox(t, "st.y", y2, 0, epsilon)
}

// ---------- TransformPath ----------

func TestTransformPathTranslate(t *testing.T) {
	p := Rect(0, 0, 10, 10)
	tp := TransformPath(p, Translate(100, 200))

	minX, minY, maxX, maxY := BoundingBox(tp)
	requireApprox(t, "minX", minX, 100, epsilon)
	requireApprox(t, "minY", minY, 200, epsilon)
	requireApprox(t, "maxX", maxX, 110, epsilon)
	requireApprox(t, "maxY", maxY, 210, epsilon)
}

func TestTransformPathScale(t *testing.T) {
	p := Rect(0, 0, 10, 10)
	tp := TransformPath(p, Scale(2, 3))

	minX, minY, maxX, maxY := BoundingBox(tp)
	requireApprox(t, "minX", minX, 0, epsilon)
	requireApprox(t, "minY", minY, 0, epsilon)
	requireApprox(t, "maxX", maxX, 20, epsilon)
	requireApprox(t, "maxY", maxY, 30, epsilon)
}

func TestTransformPathIdentity(t *testing.T) {
	p := Circle(50, 50, 25)
	tp := TransformPath(p, Identity())

	// Points should be identical.
	if len(tp.Points) != len(p.Points) {
		t.Fatal("Identity transform should not change point count")
	}
	for i := range p.Points {
		requireApprox(t, "point", tp.Points[i], p.Points[i], epsilon)
	}
}

func TestTransformPathPreservesVerbs(t *testing.T) {
	p := Circle(0, 0, 10)
	tp := TransformPath(p, Scale(2, 2))

	if len(tp.Verbs) != len(p.Verbs) {
		t.Fatal("Transform should not change verb count")
	}
	for i, v := range p.Verbs {
		if tp.Verbs[i] != v {
			t.Errorf("verb[%d]: got %d, want %d", i, tp.Verbs[i], v)
		}
	}
}

// ---------- Dash ----------

func TestDashBasic(t *testing.T) {
	var b PathBuilder
	b.MoveTo(0, 0)
	b.LineTo(100, 0) // length 100
	p := b.Finish()

	dashed := Dash(p, []float32{10, 5}, 0)

	// Pattern repeats: 10 on, 5 off, 10 on, 5 off, ...
	// Over 100 units: 10-5-10-5-10-5-10-5-10-5-10 → ~7 dashes
	moveCount := 0
	for _, v := range dashed.Verbs {
		if v == VerbMove {
			moveCount++
		}
	}
	// Each visible dash starts with a MoveTo.
	if moveCount < 5 {
		t.Errorf("expected at least 5 dash segments, got %d MoveTo verbs", moveCount)
	}
}

func TestDashEmptyPattern(t *testing.T) {
	p := Rect(0, 0, 10, 10)
	dashed := Dash(p, nil, 0)
	// Should return the original path.
	if len(dashed.Verbs) != len(p.Verbs) {
		t.Error("Dash with empty pattern should return path unchanged")
	}
}

func TestDashWithOffset(t *testing.T) {
	var b PathBuilder
	b.MoveTo(0, 0)
	b.LineTo(100, 0)
	p := b.Finish()

	d1 := Dash(p, []float32{10, 10}, 0)
	d2 := Dash(p, []float32{10, 10}, 5)

	// Shifted pattern should produce different output.
	if len(d1.Verbs) == len(d2.Verbs) && len(d1.Points) == len(d2.Points) {
		// They could theoretically have same counts but different points.
		// Just check they're not byte-identical.
		same := true
		for i := range d1.Points {
			if d1.Points[i] != d2.Points[i] {
				same = false
				break
			}
		}
		if same {
			t.Error("Dash with different offsets should produce different paths")
		}
	}
}

// ---------- PathVerb.PointCount ----------

func TestVerbPointCount(t *testing.T) {
	tests := []struct {
		verb PathVerb
		want int
	}{
		{VerbMove, 2},
		{VerbLine, 2},
		{VerbQuad, 4},
		{VerbCubic, 6},
		{VerbClose, 0},
	}
	for _, tc := range tests {
		if got := tc.verb.PointCount(); got != tc.want {
			t.Errorf("PathVerb(%d).PointCount() = %d, want %d", tc.verb, got, tc.want)
		}
	}
}
