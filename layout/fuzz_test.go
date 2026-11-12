package layout

import (
	"math"
	"os"
	"testing"
)

func FuzzLayoutEngine(f *testing.F) {
	// Seed corpus with interesting initial patterns
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19})
	f.Add([]byte{128, 64, 32, 16, 8, 4, 2, 1, 255, 254, 253, 252, 251, 250, 249, 248})
	f.Add([]byte{42, 42, 42, 42, 100, 200, 50, 25, 10, 5, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4})

	taffyBin := "/tmp/taffy_compare/target/release/taffy_compare"
	hasTaffy := false
	if _, err := os.Stat(taffyBin); err == nil && os.Getenv("TAFFY_DIFF") == "1" {
		hasTaffy = true
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		treeDef, tree, ok := GenerateTreeFromBytes(data)
		if !ok {
			return
		}

		// 1. Invariance Test: Must not panic
		tree.Compute(treeDef.ViewportW, treeDef.ViewportH)

		// 2. Numerical Invariant Tests
		count := tree.NodeCount()
		firstPassRects := make([]Rect, count)
		for i := 0; i < count; i++ {
			rect := tree.ComputedRect(NodeID(i))
			firstPassRects[i] = rect

			if math.IsNaN(float64(rect.X)) || math.IsNaN(float64(rect.Y)) ||
				math.IsNaN(float64(rect.Width)) || math.IsNaN(float64(rect.Height)) {
				t.Fatalf("Layout produced NaN for node %d: %+v", i, rect)
			}
			if math.IsInf(float64(rect.X), 0) || math.IsInf(float64(rect.Y), 0) ||
				math.IsInf(float64(rect.Width), 0) || math.IsInf(float64(rect.Height), 0) {
				t.Fatalf("Layout produced Inf for node %d: %+v", i, rect)
			}
			if rect.Width < 0 || rect.Height < 0 {
				t.Fatalf("Layout produced negative dimensions for node %d: %+v", i, rect)
			}
		}

		// 3. Idempotence Invariant Test: A re-run must yield identical results
		tree.Compute(treeDef.ViewportW, treeDef.ViewportH)
		for i := 0; i < count; i++ {
			rect2 := tree.ComputedRect(NodeID(i))
			r1 := firstPassRects[i]
			if r1.X != rect2.X || r1.Y != rect2.Y || r1.Width != rect2.Width || r1.Height != rect2.Height {
				t.Fatalf("Layout is non-idempotent on node %d: first=%+v second=%+v", i, r1, rect2)
			}
		}

		// 4. Differential Comparison Oracle against Rust Taffy
		if hasTaffy {
			ok, diff := DifferentialCheckWithTaffy(taffyBin, treeDef, tree)
			if !ok {
				t.Logf("Differential notice: %s", diff)
			}
		}
	})
}
