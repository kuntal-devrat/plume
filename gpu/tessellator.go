package gpu

import (
	"github.com/kuntal-devrat/plume"
	"github.com/kuntal-devrat/plume/geom"
)

type point2D struct {
	x, y float32
}

// TessellatePath converts a vector [geom.Path] into an indexed triangle [Mesh].
// Multiple contours and holes are recognized and triangulated cleanly.
func TessellatePath(p geom.Path, tolerance float32, col plume.Color, dst *Mesh) {
	if dst == nil {
		return
	}

	flat := geom.Flatten(p, tolerance)
	var contours [][]point2D
	var currentPolygon []point2D
	idx := 0

	for _, v := range flat.Verbs {
		switch v {
		case geom.VerbMove:
			if len(currentPolygon) >= 3 {
				contours = append(contours, currentPolygon)
				currentPolygon = nil
			}
			currentPolygon = append(currentPolygon, point2D{flat.Points[idx], flat.Points[idx+1]})
			idx += 2

		case geom.VerbLine:
			currentPolygon = append(currentPolygon, point2D{flat.Points[idx], flat.Points[idx+1]})
			idx += 2

		case geom.VerbClose:
			if len(currentPolygon) >= 3 {
				contours = append(contours, currentPolygon)
				currentPolygon = nil
			}
		}
	}

	if len(currentPolygon) >= 3 {
		contours = append(contours, currentPolygon)
	}

	if len(contours) == 0 {
		return
	}

	// Separate outer contours from hole contours based on signed area
	var outers [][]point2D
	var holes [][]point2D

	for _, c := range contours {
		clean := cleanPolygon(c)
		if len(clean) < 3 {
			continue
		}
		if polygonArea(clean) >= 0 {
			outers = append(outers, clean)
		} else {
			holes = append(holes, clean)
		}
	}

	// If no outer contours were detected (e.g. all clockwise), treat the largest as outer
	if len(outers) == 0 && len(holes) > 0 {
		outers = append(outers, holes[0])
		holes = holes[1:]
	}

	// Triangulate each outer contour (merging associated holes)
	for _, outer := range outers {
		merged := outer
		for _, hole := range holes {
			if isHoleInside(hole, merged) {
				merged = bridgeHole(merged, hole)
			}
		}
		triangulateSimplePolygon(merged, col, dst)
	}
}

func cleanPolygon(pts []point2D) []point2D {
	n := len(pts)
	if n < 3 {
		return pts
	}
	if pts[0].x == pts[n-1].x && pts[0].y == pts[n-1].y {
		pts = pts[:n-1]
	}
	return pts
}

func isHoleInside(hole, outer []point2D) bool {
	if len(hole) == 0 || len(outer) == 0 {
		return false
	}
	// Test centroid of hole against outer polygon
	var cx, cy float32
	for _, p := range hole {
		cx += p.x
		cy += p.y
	}
	cx /= float32(len(hole))
	cy /= float32(len(hole))
	return pointInPolygon(point2D{cx, cy}, outer)
}

func pointInPolygon(p point2D, poly []point2D) bool {
	inside := false
	n := len(poly)
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		if ((poly[i].y > p.y) != (poly[j].y > p.y)) &&
			(p.x < (poly[j].x-poly[i].x)*(p.y-poly[i].y)/(poly[j].y-poly[i].y)+poly[i].x) {
			inside = !inside
		}
	}
	return inside
}

// bridgeHole connects a hole contour to an outer contour via a pair of bridge edges.
func bridgeHole(outer, hole []point2D) []point2D {
	if len(outer) == 0 {
		return hole
	}
	if len(hole) == 0 {
		return outer
	}

	// Find hole point with minimum X
	holeMinIdx := 0
	for i := 1; i < len(hole); i++ {
		if hole[i].x < hole[holeMinIdx].x {
			holeMinIdx = i
		}
	}
	hPt := hole[holeMinIdx]

	// Find closest vertex in outer polygon
	outerBestIdx := 0
	var bestDistSq float32 = 1e30
	for i, oPt := range outer {
		dx := oPt.x - hPt.x
		dy := oPt.y - hPt.y
		dist := dx*dx + dy*dy
		if dist < bestDistSq {
			bestDistSq = dist
			outerBestIdx = i
		}
	}

	// Construct joined polygon:
	// outer[0..outerBestIdx] -> hole[holeMinIdx..end] -> hole[0..holeMinIdx] -> outer[outerBestIdx..end]
	res := make([]point2D, 0, len(outer)+len(hole)+2)
	res = append(res, outer[:outerBestIdx+1]...)

	for i := 0; i < len(hole); i++ {
		idx := (holeMinIdx + i) % len(hole)
		res = append(res, hole[idx])
	}
	// Return to hole start
	res = append(res, hole[holeMinIdx])
	// Return to outer bridge point
	res = append(res, outer[outerBestIdx:]...)

	return res
}

// triangulateSimplePolygon decomposes a simple counter-clockwise polygon into triangles using ear-clipping.
func triangulateSimplePolygon(pts []point2D, col plume.Color, dst *Mesh) {
	n := len(pts)
	if n < 3 {
		return
	}

	baseVertex := uint32(len(dst.Vertices))

	for _, p := range pts {
		dst.Vertices = append(dst.Vertices, Vertex{
			X: p.x,
			Y: p.y,
			R: col.R,
			G: col.G,
			B: col.B,
			A: col.A,
		})
	}

	if n == 3 {
		dst.Indices = append(dst.Indices, baseVertex, baseVertex+1, baseVertex+2)
		return
	}

	if n == 4 {
		dst.Indices = append(dst.Indices,
			baseVertex, baseVertex+1, baseVertex+2,
			baseVertex, baseVertex+2, baseVertex+3,
		)
		return
	}

	indices := make([]int, n)
	for i := 0; i < n; i++ {
		indices[i] = i
	}

	// Ensure counter-clockwise
	if polygonArea(pts) < 0 {
		for i, j := 0, n-1; i < j; i, j = i+1, j-1 {
			indices[i], indices[j] = indices[j], indices[i]
		}
	}

	for len(indices) > 2 {
		earFound := false
		m := len(indices)

		for i := 0; i < m; i++ {
			prev := indices[(i-1+m)%m]
			curr := indices[i]
			next := indices[(i+1)%m]

			a := pts[prev]
			b := pts[curr]
			c := pts[next]

			if isConvex(a, b, c) && !hasPointInside(a, b, c, pts, indices, prev, curr, next) {
				dst.Indices = append(dst.Indices,
					baseVertex+uint32(prev),
					baseVertex+uint32(curr),
					baseVertex+uint32(next),
				)
				indices = append(indices[:i], indices[i+1:]...)
				earFound = true
				break
			}
		}

		if !earFound {
			// Fallback fan triangulation if degenerate
			for i := 1; i < len(indices)-1; i++ {
				dst.Indices = append(dst.Indices,
					baseVertex+uint32(indices[0]),
					baseVertex+uint32(indices[i]),
					baseVertex+uint32(indices[i+1]),
				)
			}
			break
		}
	}
}

func polygonArea(pts []point2D) float32 {
	var area float32
	n := len(pts)
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		area += pts[i].x*pts[j].y - pts[j].x*pts[i].y
	}
	return area * 0.5
}

func isConvex(a, b, c point2D) bool {
	return (b.x-a.x)*(c.y-a.y)-(b.y-a.y)*(c.x-a.x) >= 0
}

func hasPointInside(a, b, c point2D, allPts []point2D, indices []int, prev, curr, next int) bool {
	for _, idx := range indices {
		if idx == prev || idx == curr || idx == next {
			continue
		}
		p := allPts[idx]
		if pointInTriangle(p, a, b, c) {
			return true
		}
	}
	return false
}

func pointInTriangle(p, a, b, c point2D) bool {
	d1 := (p.x-b.x)*(a.y-b.y) - (a.x-b.x)*(p.y-b.y)
	d2 := (p.x-c.x)*(b.y-c.y) - (b.x-c.x)*(p.y-c.y)
	d3 := (p.x-a.x)*(c.y-a.y) - (c.x-a.x)*(p.y-a.y)

	hasNeg := (d1 < 0) || (d2 < 0) || (d3 < 0)
	hasPos := (d1 > 0) || (d2 > 0) || (d3 > 0)

	return !(hasNeg && hasPos)
}
