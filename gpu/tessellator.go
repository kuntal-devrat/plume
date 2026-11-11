package gpu

import (
	"github.com/plume-engine/plume"
	"github.com/plume-engine/plume/geom"
)

type point2D struct {
	x, y float32
}

// TessellatePath converts a vector [geom.Path] into an indexed triangle [Mesh].
// Non-linear segments are flattened with the given tolerance before triangulation.
func TessellatePath(p geom.Path, tolerance float32, col plume.Color, dst *Mesh) {
	if dst == nil {
		return
	}

	flat := geom.Flatten(p, tolerance)
	var currentPolygon []point2D
	idx := 0

	for _, v := range flat.Verbs {
		switch v {
		case geom.VerbMove:
			if len(currentPolygon) >= 3 {
				triangulatePolygon(currentPolygon, col, dst)
			}
			currentPolygon = currentPolygon[:0]
			currentPolygon = append(currentPolygon, point2D{flat.Points[idx], flat.Points[idx+1]})
			idx += 2

		case geom.VerbLine:
			currentPolygon = append(currentPolygon, point2D{flat.Points[idx], flat.Points[idx+1]})
			idx += 2

		case geom.VerbClose:
			if len(currentPolygon) >= 3 {
				triangulatePolygon(currentPolygon, col, dst)
			}
			currentPolygon = currentPolygon[:0]
		}
	}

	if len(currentPolygon) >= 3 {
		triangulatePolygon(currentPolygon, col, dst)
	}
}

// triangulatePolygon decomposes a simple polygon (convex or concave) into triangles
// using the ear-clipping algorithm.
func triangulatePolygon(pts []point2D, col plume.Color, dst *Mesh) {
	n := len(pts)
	if n < 3 {
		return
	}

	// Remove duplicate end point if closed
	if pts[0].x == pts[n-1].x && pts[0].y == pts[n-1].y {
		pts = pts[:n-1]
		n = len(pts)
		if n < 3 {
			return
		}
	}

	baseVertex := uint32(len(dst.Vertices))

	// Add vertices to mesh
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

	// Triangulate convex quad directly
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

	// Ear-clipping for general polygons
	indices := make([]int, n)
	for i := 0; i < n; i++ {
		indices[i] = i
	}

	// Ensure counter-clockwise orientation
	area := polygonArea(pts)
	if area < 0 {
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
			// Fallback: fan triangulation if degenerate/self-intersecting
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
