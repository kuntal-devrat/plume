package layout

import (
	"fmt"
	"math"
	"testing"
)

func assertValidRect(t *testing.T, name string, r Rect) {
	t.Helper()
	if math.IsNaN(float64(r.X)) || math.IsNaN(float64(r.Y)) || math.IsNaN(float64(r.Width)) || math.IsNaN(float64(r.Height)) {
		t.Fatalf("%s rect contains NaN: %+v", name, r)
	}
	if math.IsInf(float64(r.X), 0) || math.IsInf(float64(r.Y), 0) || math.IsInf(float64(r.Width), 0) || math.IsInf(float64(r.Height), 0) {
		t.Fatalf("%s rect contains Inf: %+v", name, r)
	}
}

// TestCrampedMobileLayout tests flexbox wrapping and bounds in extremely tight viewport (320x568)
func TestCrampedMobileLayout(t *testing.T) {
	w, h := float32(320), float32(568)
	tree := NewTree()
	root := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionColumn,
		Width:         Px(w),
		Height:        Px(h),
		Padding:       Uniform(16),
		Gap:           Px(12),
	})

	cardsCont := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionRow,
		FlexWrap:      FlexWrapWrap,
		Gap:           Px(8),
		FlexGrow:      1,
	})
	tree.AddChild(root, cardsCont)

	var cards []NodeID
	for i := 0; i < 4; i++ {
		c := tree.NewNode(Style{
			Width:  Px(136),
			Height: Px(90),
		})
		tree.AddChild(cardsCont, c)
		cards = append(cards, c)
	}

	bottomBar := tree.NewNode(Style{
		Height: Px(48),
		Width:  Percent(100),
	})
	tree.AddChild(root, bottomBar)

	tree.Compute(w, h)

	for i, c := range cards {
		rect := tree.AbsoluteRect(c)
		assertValidRect(t, fmt.Sprintf("card_%d", i), rect)
		if rect.Width <= 0 || rect.Height <= 0 {
			t.Errorf("card %d has non-positive dimension: %+v", i, rect)
		}
	}

	bRect := tree.AbsoluteRect(bottomBar)
	assertValidRect(t, "bottomBar", bRect)
	if bRect.Y <= 0 {
		t.Errorf("bottomBar has invalid Y offset: %+v", bRect)
	}
}

// TestDeepNestingCoordinates verifies that AbsoluteRect correctly accumulates parent offsets across 4 levels
func TestDeepNestingCoordinates(t *testing.T) {
	w, h := float32(600), float32(400)
	tree := NewTree()
	root := tree.NewNode(Style{
		Width:   Px(w),
		Height:  Px(h),
		Padding: Uniform(30),
	})
	l1 := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionColumn,
		Padding:       Uniform(20),
		FlexGrow:      1,
	})
	tree.AddChild(root, l1)

	l2 := tree.NewNode(Style{
		Display:        DisplayFlex,
		FlexDirection:  FlexDirectionRow,
		JustifyContent: JustifySpaceBetween,
		AlignItems:     AlignCenter,
		Padding:        Uniform(15),
		Height:         Px(80),
	})
	tree.AddChild(l1, l2)

	l3 := tree.NewNode(Style{
		Width:   Px(100),
		Height:  Px(36),
		Padding: Uniform(8),
	})
	tree.AddChild(l2, l3)

	tree.Compute(w, h)

	rRoot := tree.AbsoluteRect(root)
	rL1 := tree.AbsoluteRect(l1)
	rL2 := tree.AbsoluteRect(l2)
	rL3 := tree.AbsoluteRect(l3)

	assertValidRect(t, "Root", rRoot)
	assertValidRect(t, "L1", rL1)
	assertValidRect(t, "L2", rL2)
	assertValidRect(t, "L3", rL3)

	if rL1.X < rRoot.X || rL1.Y < rRoot.Y {
		t.Errorf("L1 not within Root: L1=%+v Root=%+v", rL1, rRoot)
	}
	if rL2.X < rL1.X || rL2.Y < rL1.Y {
		t.Errorf("L2 not within L1: L2=%+v L1=%+v", rL2, rL1)
	}
	if rL3.X < rL2.X || rL3.Y < rL2.Y {
		t.Errorf("L3 not within L2: L3=%+v L2=%+v", rL3, rL2)
	}
}
