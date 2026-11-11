package layout

import (
	"math"
	"testing"
)

// almostEqual compares two float32 values within a tolerance of 0.01.
func almostEqual(a, b float32) bool {
	return float32(math.Abs(float64(a-b))) < 0.01
}

// assertRect checks that a node's computed rect matches expected values.
func assertRect(t *testing.T, tree *Tree, id NodeID, label string, x, y, w, h float32) {
	t.Helper()
	r := tree.ComputedRect(id)
	if !almostEqual(r.X, x) || !almostEqual(r.Y, y) || !almostEqual(r.Width, w) || !almostEqual(r.Height, h) {
		t.Errorf("%s: got {X:%.2f Y:%.2f W:%.2f H:%.2f}, want {X:%.2f Y:%.2f W:%.2f H:%.2f}",
			label, r.X, r.Y, r.Width, r.Height, x, y, w, h)
	}
}

// ---------------------------------------------------------------------------
// 1. Basic row layout
// ---------------------------------------------------------------------------

func TestBasicRow(t *testing.T) {
	tree := NewTree()
	root := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionRow,
		Width:         Px(600),
		Height:        Px(100),
	})
	c1 := tree.NewNode(Style{Width: Px(100), Height: Px(100)})
	c2 := tree.NewNode(Style{Width: Px(100), Height: Px(100)})
	c3 := tree.NewNode(Style{Width: Px(100), Height: Px(100)})
	tree.AddChild(root, c1)
	tree.AddChild(root, c2)
	tree.AddChild(root, c3)
	tree.Compute(600, 100)

	assertRect(t, tree, c1, "c1", 0, 0, 100, 100)
	assertRect(t, tree, c2, "c2", 100, 0, 100, 100)
	assertRect(t, tree, c3, "c3", 200, 0, 100, 100)
}

// ---------------------------------------------------------------------------
// 2. Basic column layout
// ---------------------------------------------------------------------------

func TestBasicColumn(t *testing.T) {
	tree := NewTree()
	root := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionColumn,
		Width:         Px(100),
		Height:        Px(600),
	})
	c1 := tree.NewNode(Style{Width: Px(100), Height: Px(100)})
	c2 := tree.NewNode(Style{Width: Px(100), Height: Px(100)})
	c3 := tree.NewNode(Style{Width: Px(100), Height: Px(100)})
	tree.AddChild(root, c1)
	tree.AddChild(root, c2)
	tree.AddChild(root, c3)
	tree.Compute(100, 600)

	assertRect(t, tree, c1, "c1", 0, 0, 100, 100)
	assertRect(t, tree, c2, "c2", 0, 100, 100, 100)
	assertRect(t, tree, c3, "c3", 0, 200, 100, 100)
}

// ---------------------------------------------------------------------------
// 3. FlexGrow
// ---------------------------------------------------------------------------

func TestFlexGrow(t *testing.T) {
	tree := NewTree()
	root := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionRow,
		Width:         Px(300),
		Height:        Px(100),
	})
	// Two items: grow 1 and grow 2 → 100px and 200px.
	c1 := tree.NewNode(Style{FlexGrow: 1})
	c2 := tree.NewNode(Style{FlexGrow: 2})
	tree.AddChild(root, c1)
	tree.AddChild(root, c2)
	tree.Compute(300, 100)

	assertRect(t, tree, c1, "grow-1", 0, 0, 100, 100)
	assertRect(t, tree, c2, "grow-2", 100, 0, 200, 100)
}

// ---------------------------------------------------------------------------
// 4. FlexShrink
// ---------------------------------------------------------------------------

func TestFlexShrink(t *testing.T) {
	tree := NewTree()
	root := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionRow,
		Width:         Px(200),
		Height:        Px(100),
	})
	// Two items each 150px wide → 300px total, but container is 200px.
	// Both have FlexShrink=1, same basis → each shrinks equally to 100px.
	c1 := tree.NewNode(Style{Width: Px(150), FlexShrink: 1})
	c2 := tree.NewNode(Style{Width: Px(150), FlexShrink: 1})
	tree.AddChild(root, c1)
	tree.AddChild(root, c2)
	tree.Compute(200, 100)

	assertRect(t, tree, c1, "shrink-1", 0, 0, 100, 100)
	assertRect(t, tree, c2, "shrink-2", 100, 0, 100, 100)
}

// ---------------------------------------------------------------------------
// 5. JustifyContent — all 6 values
// ---------------------------------------------------------------------------

func TestJustifyContent(t *testing.T) {
	tests := []struct {
		name    string
		justify JustifyContent
		// Expected X positions of 3 items each 50px wide in a 300px container.
		x1, x2, x3 float32
	}{
		{"FlexStart", JustifyFlexStart, 0, 50, 100},
		{"FlexEnd", JustifyFlexEnd, 150, 200, 250},
		{"Center", JustifyCenter, 75, 125, 175},
		{"SpaceBetween", JustifySpaceBetween, 0, 125, 250},
		// SpaceAround: space = 150/3 = 50 each → half-space = 25.
		// positions: 25, 25+50+50=125, 125+50+50=225
		{"SpaceAround", JustifySpaceAround, 25, 125, 225},
		// SpaceEvenly: space = 150/4 = 37.5.
		// positions: 37.5, 37.5+50+37.5=125, 125+50+37.5=212.5
		{"SpaceEvenly", JustifySpaceEvenly, 37.5, 125, 212.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := NewTree()
			root := tree.NewNode(Style{
				Display:        DisplayFlex,
				FlexDirection:  FlexDirectionRow,
				JustifyContent: tt.justify,
				Width:          Px(300),
				Height:         Px(100),
			})
			c1 := tree.NewNode(Style{Width: Px(50), Height: Px(50)})
			c2 := tree.NewNode(Style{Width: Px(50), Height: Px(50)})
			c3 := tree.NewNode(Style{Width: Px(50), Height: Px(50)})
			tree.AddChild(root, c1)
			tree.AddChild(root, c2)
			tree.AddChild(root, c3)
			tree.Compute(300, 100)

			r1 := tree.ComputedRect(c1)
			r2 := tree.ComputedRect(c2)
			r3 := tree.ComputedRect(c3)

			if !almostEqual(r1.X, tt.x1) {
				t.Errorf("c1.X = %.2f, want %.2f", r1.X, tt.x1)
			}
			if !almostEqual(r2.X, tt.x2) {
				t.Errorf("c2.X = %.2f, want %.2f", r2.X, tt.x2)
			}
			if !almostEqual(r3.X, tt.x3) {
				t.Errorf("c3.X = %.2f, want %.2f", r3.X, tt.x3)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 6. AlignItems
// ---------------------------------------------------------------------------

func TestAlignItems(t *testing.T) {
	tests := []struct {
		name   string
		align  AlignItems
		yPos   float32 // expected Y of a 50px-tall item in a 100px container
		height float32 // expected height
	}{
		{"Stretch", AlignStretch, 0, 100},
		{"FlexStart", AlignFlexStart, 0, 50},
		{"FlexEnd", AlignFlexEnd, 50, 50},
		{"Center", AlignCenter, 25, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := NewTree()
			root := tree.NewNode(Style{
				Display:       DisplayFlex,
				FlexDirection: FlexDirectionRow,
				AlignItems:    tt.align,
				Width:         Px(300),
				Height:        Px(100),
			})
			var childStyle Style
			childStyle.Width = Px(50)
			if tt.align != AlignStretch {
				childStyle.Height = Px(50)
			}
			c := tree.NewNode(childStyle)
			tree.AddChild(root, c)
			tree.Compute(300, 100)

			r := tree.ComputedRect(c)
			if !almostEqual(r.Y, tt.yPos) {
				t.Errorf("Y = %.2f, want %.2f", r.Y, tt.yPos)
			}
			if !almostEqual(r.Height, tt.height) {
				t.Errorf("Height = %.2f, want %.2f", r.Height, tt.height)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 7. Padding
// ---------------------------------------------------------------------------

func TestPadding(t *testing.T) {
	tree := NewTree()
	root := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionRow,
		Width:         Px(200),
		Height:        Px(200),
		Padding:       Uniform(20),
	})
	c := tree.NewNode(Style{Width: Px(50), Height: Px(50)})
	tree.AddChild(root, c)
	tree.Compute(200, 200)

	// Child should be offset by padding.
	assertRect(t, tree, c, "padded-child", 20, 20, 50, 50)
}

// ---------------------------------------------------------------------------
// 8. Gap
// ---------------------------------------------------------------------------

func TestGap(t *testing.T) {
	tree := NewTree()
	root := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionRow,
		Width:         Px(300),
		Height:        Px(100),
		Gap:           Px(10),
	})
	c1 := tree.NewNode(Style{Width: Px(50), Height: Px(50)})
	c2 := tree.NewNode(Style{Width: Px(50), Height: Px(50)})
	c3 := tree.NewNode(Style{Width: Px(50), Height: Px(50)})
	tree.AddChild(root, c1)
	tree.AddChild(root, c2)
	tree.AddChild(root, c3)
	tree.Compute(300, 100)

	assertRect(t, tree, c1, "gap-c1", 0, 0, 50, 50)
	assertRect(t, tree, c2, "gap-c2", 60, 0, 50, 50)
	assertRect(t, tree, c3, "gap-c3", 120, 0, 50, 50)
}

// ---------------------------------------------------------------------------
// 9. Nested flex: parent = column, child row = row
// ---------------------------------------------------------------------------

func TestNestedFlex(t *testing.T) {
	tree := NewTree()
	root := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionColumn,
		Width:         Px(400),
		Height:        Px(200),
	})
	// Row container as first child.
	row := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionRow,
		FlexGrow:      1,
	})
	tree.AddChild(root, row)

	a := tree.NewNode(Style{FlexGrow: 1})
	b := tree.NewNode(Style{FlexGrow: 1})
	tree.AddChild(row, a)
	tree.AddChild(row, b)

	tree.Compute(400, 200)

	// Row should fill the entire parent (grow=1 in a single-child column).
	rowRect := tree.ComputedRect(row)
	if !almostEqual(rowRect.Width, 400) {
		t.Errorf("row width = %.2f, want 400", rowRect.Width)
	}
	if !almostEqual(rowRect.Height, 200) {
		t.Errorf("row height = %.2f, want 200", rowRect.Height)
	}

	// Each child of the row should be 200px wide (half of 400).
	assertRect(t, tree, a, "nested-a", 0, 0, 200, 200)
	assertRect(t, tree, b, "nested-b", 200, 0, 200, 200)
}

// ---------------------------------------------------------------------------
// 10. FlexWrap
// ---------------------------------------------------------------------------

func TestFlexWrap(t *testing.T) {
	tree := NewTree()
	root := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionRow,
		FlexWrap:      FlexWrapWrap,
		AlignContent:  AlignContentFlexStart,
		Width:         Px(200),
		Height:        Px(200),
		AlignItems:    AlignFlexStart,
	})
	// 3 items each 100px → first 2 fit on line 1, third wraps.
	c1 := tree.NewNode(Style{Width: Px(100), Height: Px(50)})
	c2 := tree.NewNode(Style{Width: Px(100), Height: Px(50)})
	c3 := tree.NewNode(Style{Width: Px(100), Height: Px(50)})
	tree.AddChild(root, c1)
	tree.AddChild(root, c2)
	tree.AddChild(root, c3)
	tree.Compute(200, 200)

	r1 := tree.ComputedRect(c1)
	r2 := tree.ComputedRect(c2)
	r3 := tree.ComputedRect(c3)

	// Line 1: c1 at x=0, c2 at x=100, both y=0.
	if !almostEqual(r1.X, 0) || !almostEqual(r1.Y, 0) {
		t.Errorf("c1 pos = (%.2f, %.2f), want (0, 0)", r1.X, r1.Y)
	}
	if !almostEqual(r2.X, 100) || !almostEqual(r2.Y, 0) {
		t.Errorf("c2 pos = (%.2f, %.2f), want (100, 0)", r2.X, r2.Y)
	}
	// Line 2: c3 at x=0, y=50 (line 1 cross size = 50).
	if !almostEqual(r3.X, 0) || !almostEqual(r3.Y, 50) {
		t.Errorf("c3 pos = (%.2f, %.2f), want (0, 50)", r3.X, r3.Y)
	}
}

// ---------------------------------------------------------------------------
// 11. Percentage dimensions
// ---------------------------------------------------------------------------

func TestPercentageDimensions(t *testing.T) {
	tree := NewTree()
	root := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionRow,
		Width:         Px(400),
		Height:        Px(200),
		AlignItems:    AlignFlexStart,
	})
	// Child with 50% width.
	c := tree.NewNode(Style{Width: Percent(50), Height: Px(50)})
	tree.AddChild(root, c)
	tree.Compute(400, 200)

	r := tree.ComputedRect(c)
	if !almostEqual(r.Width, 200) {
		t.Errorf("width = %.2f, want 200", r.Width)
	}
}

// ---------------------------------------------------------------------------
// 12. Min/Max constraints
// ---------------------------------------------------------------------------

func TestMinMaxConstraints(t *testing.T) {
	tree := NewTree()
	root := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionRow,
		Width:         Px(600),
		Height:        Px(100),
	})
	// Item wants to grow, but is clamped at max-width 150.
	c1 := tree.NewNode(Style{
		FlexGrow: 1,
		MaxWidth: Px(150),
	})
	// Item wants to shrink, but is clamped at min-width 100.
	c2 := tree.NewNode(Style{
		FlexGrow: 1,
	})
	tree.AddChild(root, c1)
	tree.AddChild(root, c2)
	tree.Compute(600, 100)

	r1 := tree.ComputedRect(c1)
	if r1.Width > 150.01 {
		t.Errorf("c1 width = %.2f, want <= 150", r1.Width)
	}
}

// ---------------------------------------------------------------------------
// 13. DisplayNone
// ---------------------------------------------------------------------------

func TestDisplayNone(t *testing.T) {
	tree := NewTree()
	root := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionRow,
		Width:         Px(300),
		Height:        Px(100),
	})
	c1 := tree.NewNode(Style{Width: Px(100), Height: Px(100)})
	cHidden := tree.NewNode(Style{Display: DisplayNone, Width: Px(100), Height: Px(100)})
	c3 := tree.NewNode(Style{Width: Px(100), Height: Px(100)})
	tree.AddChild(root, c1)
	tree.AddChild(root, cHidden)
	tree.AddChild(root, c3)
	tree.Compute(300, 100)

	// Hidden item should have zero rect.
	rH := tree.ComputedRect(cHidden)
	if rH.Width != 0 || rH.Height != 0 {
		t.Errorf("hidden item rect = %+v, want zero", rH)
	}

	// c3 should be right after c1, skipping the hidden item.
	assertRect(t, tree, c1, "c1", 0, 0, 100, 100)
	assertRect(t, tree, c3, "c3", 100, 0, 100, 100)
}

// ---------------------------------------------------------------------------
// 14. Reverse directions
// ---------------------------------------------------------------------------

func TestFlexDirectionRowReverse(t *testing.T) {
	tree := NewTree()
	root := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionRowReverse,
		Width:         Px(300),
		Height:        Px(100),
	})
	c1 := tree.NewNode(Style{Width: Px(100), Height: Px(100)})
	c2 := tree.NewNode(Style{Width: Px(100), Height: Px(100)})
	tree.AddChild(root, c1)
	tree.AddChild(root, c2)
	tree.Compute(300, 100)

	// In row-reverse, first child is at the right end.
	r1 := tree.ComputedRect(c1)
	r2 := tree.ComputedRect(c2)

	if !almostEqual(r1.X, 200) {
		t.Errorf("c1.X = %.2f, want 200", r1.X)
	}
	if !almostEqual(r2.X, 100) {
		t.Errorf("c2.X = %.2f, want 100", r2.X)
	}
}

func TestFlexDirectionColumnReverse(t *testing.T) {
	tree := NewTree()
	root := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionColumnReverse,
		Width:         Px(100),
		Height:        Px(300),
	})
	c1 := tree.NewNode(Style{Width: Px(100), Height: Px(100)})
	c2 := tree.NewNode(Style{Width: Px(100), Height: Px(100)})
	tree.AddChild(root, c1)
	tree.AddChild(root, c2)
	tree.Compute(100, 300)

	r1 := tree.ComputedRect(c1)
	r2 := tree.ComputedRect(c2)

	if !almostEqual(r1.Y, 200) {
		t.Errorf("c1.Y = %.2f, want 200", r1.Y)
	}
	if !almostEqual(r2.Y, 100) {
		t.Errorf("c2.Y = %.2f, want 100", r2.Y)
	}
}

// ---------------------------------------------------------------------------
// 15. AlignSelf override
// ---------------------------------------------------------------------------

func TestAlignSelfOverride(t *testing.T) {
	tree := NewTree()
	root := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionRow,
		AlignItems:    AlignFlexStart,
		Width:         Px(300),
		Height:        Px(100),
	})
	// Default alignment (FlexStart) → y=0.
	c1 := tree.NewNode(Style{Width: Px(50), Height: Px(30)})
	// Override to center → y = (100 - 30) / 2 = 35.
	c2 := tree.NewNode(Style{Width: Px(50), Height: Px(30), AlignSelf: AlignSelfCenter})
	// Override to flex-end → y = 100 - 30 = 70.
	c3 := tree.NewNode(Style{Width: Px(50), Height: Px(30), AlignSelf: AlignSelfFlexEnd})
	tree.AddChild(root, c1)
	tree.AddChild(root, c2)
	tree.AddChild(root, c3)
	tree.Compute(300, 100)

	r1 := tree.ComputedRect(c1)
	r2 := tree.ComputedRect(c2)
	r3 := tree.ComputedRect(c3)

	if !almostEqual(r1.Y, 0) {
		t.Errorf("c1.Y = %.2f, want 0", r1.Y)
	}
	if !almostEqual(r2.Y, 35) {
		t.Errorf("c2.Y = %.2f, want 35", r2.Y)
	}
	if !almostEqual(r3.Y, 70) {
		t.Errorf("c3.Y = %.2f, want 70", r3.Y)
	}
}

// ---------------------------------------------------------------------------
// Margin test
// ---------------------------------------------------------------------------

func TestMargin(t *testing.T) {
	tree := NewTree()
	root := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionRow,
		Width:         Px(300),
		Height:        Px(100),
		AlignItems:    AlignFlexStart,
	})
	c := tree.NewNode(Style{
		Width:  Px(50),
		Height: Px(50),
		Margin: EdgesPx(10, 20, 10, 20),
	})
	tree.AddChild(root, c)
	tree.Compute(300, 100)

	r := tree.ComputedRect(c)
	// Left margin pushes the item right.
	if !almostEqual(r.X, 20) {
		t.Errorf("X = %.2f, want 20", r.X)
	}
	if !almostEqual(r.Y, 10) {
		t.Errorf("Y = %.2f, want 10", r.Y)
	}
}

// ---------------------------------------------------------------------------
// DefaultStyle sanity check
// ---------------------------------------------------------------------------

func TestDefaultStyle(t *testing.T) {
	s := DefaultStyle()
	if s.Display != DisplayFlex {
		t.Error("default Display should be DisplayFlex")
	}
	if !almostEqual(s.FlexShrink, 1) {
		t.Errorf("default FlexShrink = %.2f, want 1", s.FlexShrink)
	}
	if s.FlexGrow != 0 {
		t.Errorf("default FlexGrow = %.2f, want 0", s.FlexGrow)
	}
}
