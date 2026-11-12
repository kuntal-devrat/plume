package layout

import (
	"testing"
)

// ---------------------------------------------------------------------------
// 1. Shrink-to-Fit Container Auto Width & Height (Taffy Alignment)
// ---------------------------------------------------------------------------

func TestShrinkToFitContainerAutoWidth(t *testing.T) {
	tree := NewTree()

	// Container with Width: Auto, Height: Auto
	root := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionRow,
		Gap:           Px(10),
		Width:         Auto(),
		Height:        Auto(),
	})

	child1 := tree.NewNode(Style{
		Width:  Px(50),
		Height: Px(30),
	})
	child2 := tree.NewNode(Style{
		Width:  Px(70),
		Height: Px(30),
	})

	tree.AddChild(root, child1)
	tree.AddChild(root, child2)

	tree.Compute(800, 600)

	// Container should shrink tightly around children: 50 + 10 (gap) + 70 = 130px width, 30px height.
	assertRect(t, tree, root, "shrink-container", 0, 0, 130, 30)
	assertRect(t, tree, child1, "child1", 0, 0, 50, 30)
	assertRect(t, tree, child2, "child2", 60, 0, 70, 30)
}

// ---------------------------------------------------------------------------
// 2. Aspect Ratio on Flex Items
// ---------------------------------------------------------------------------

func TestAspectRatioFlexItem(t *testing.T) {
	tree := NewTree()

	// 16:9 card inside a 320px wide row container
	root := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionRow,
		Width:         Px(320),
		Height:        Auto(),
	})

	card := tree.NewNode(Style{
		FlexGrow:    1,
		Width:       Auto(),
		Height:      Auto(),
		AspectRatio: 16.0 / 9.0,
	})

	tree.AddChild(root, card)
	tree.Compute(320, 600)

	// 320 / (16/9) = 180px height
	assertRect(t, tree, card, "aspect-ratio-card", 0, 0, 320, 180)
	assertRect(t, tree, root, "aspect-ratio-root", 0, 0, 320, 180)
}

// ---------------------------------------------------------------------------
// 3. Negative Margins (Overlapping Avatar Stack)
// ---------------------------------------------------------------------------

func TestNegativeMarginAvatarStack(t *testing.T) {
	tree := NewTree()

	root := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionRow,
		Width:         Auto(),
		Height:        Auto(),
	})

	avatars := make([]NodeID, 4)
	for i := 0; i < 4; i++ {
		marginLeft := float32(0)
		if i > 0 {
			marginLeft = -8
		}
		avatars[i] = tree.NewNode(Style{
			Width:  Px(32),
			Height: Px(32),
			Margin: Edges{
				Left: Px(marginLeft),
			},
		})
		tree.AddChild(root, avatars[i])
	}

	tree.Compute(800, 600)

	// Expected positions:
	// Avatar 0: X = 0
	// Avatar 1: X = 24 (32 - 8)
	// Avatar 2: X = 48 (24 + 32 - 8)
	// Avatar 3: X = 72 (48 + 32 - 8)
	// Container Width: 32 + 3*24 = 104px
	assertRect(t, tree, avatars[0], "avatar-0", 0, 0, 32, 32)
	assertRect(t, tree, avatars[1], "avatar-1", 24, 0, 32, 32)
	assertRect(t, tree, avatars[2], "avatar-2", 48, 0, 32, 32)
	assertRect(t, tree, avatars[3], "avatar-3", 72, 0, 32, 32)
	assertRect(t, tree, root, "avatar-root", 0, 0, 104, 32)
}

// ---------------------------------------------------------------------------
// 4. Flexbox Automatic Minimum Size with Content (W3C §4.5)
// ---------------------------------------------------------------------------

func TestFlexboxAutomaticMinSizeWithContent(t *testing.T) {
	tree := NewTree()

	root := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionRow,
		Width:         Px(200),
		Height:        Px(100),
	})

	// c1 has an inner child of 120px width
	c1 := tree.NewNode(Style{
		Display:    DisplayFlex,
		Width:      Px(150),
		Height:     Px(100),
		FlexShrink: 1,
	})
	inner := tree.NewNode(Style{
		Width:  Px(120),
		Height: Px(50),
	})
	tree.AddChild(c1, inner)

	// c2 is an empty leaf with 150px width
	c2 := tree.NewNode(Style{
		Width:      Px(150),
		Height:     Px(100),
		FlexShrink: 1,
	})

	tree.AddChild(root, c1)
	tree.AddChild(root, c2)

	tree.Compute(200, 100)

	// In Taffy Case 5: c1 cannot shrink below its content (120px), so c1 = 120px and c2 = 80px.
	assertRect(t, tree, c1, "c1-clamped-by-content", 0, 0, 120, 100)
	assertRect(t, tree, c2, "c2-shrunk-further", 120, 0, 80, 100)
}

// ---------------------------------------------------------------------------
// 5. CSS Grid Negative Line Indexing (-1 = Last Line)
// ---------------------------------------------------------------------------

func TestGridNegativeLineIndexing(t *testing.T) {
	tree := NewTree()

	// 3 equal 100px columns
	grid := tree.NewNode(Style{
		Display: DisplayGrid,
		Width:   Px(300),
		Height:  Px(200),
		GridTemplateColumns: []Dimension{
			Px(100),
			Px(100),
			Px(100),
		},
		GridTemplateRows: []Dimension{
			Px(100),
			Px(100),
		},
	})

	// Header spanning full width (1 / -1)
	header := tree.NewNode(Style{
		GridColumnStart: 1,
		GridColumnEnd:   -1,
		GridRowStart:    1,
		GridRowEnd:      2,
	})

	// Sidebar in last column (-2 / -1)
	lastColItem := tree.NewNode(Style{
		GridColumnStart: -2,
		GridColumnEnd:   -1,
		GridRowStart:    2,
		GridRowEnd:      3,
	})

	tree.AddChild(grid, header)
	tree.AddChild(grid, lastColItem)

	tree.Compute(300, 200)

	// Header spans tracks 1 to 4 -> X=0, W=300
	assertRect(t, tree, header, "grid-full-span", 0, 0, 300, 100)
	// lastColItem spans track 3 to 4 -> X=200, W=100
	assertRect(t, tree, lastColItem, "grid-last-col", 200, 100, 100, 100)
}

// ---------------------------------------------------------------------------
// 6. Deferred Percentage Dimension Resolution in Auto Container
// ---------------------------------------------------------------------------

func TestDeferredPercentageInAutoContainer(t *testing.T) {
	tree := NewTree()

	root := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionRow,
		Width:         Px(400),
		Height:        Auto(),
	})

	// Fixed height child defining the container height
	fixedChild := tree.NewNode(Style{
		Width:  Px(100),
		Height: Px(200),
	})

	// Percentage child resolving against container height
	percentChild := tree.NewNode(Style{
		Width:  Px(100),
		Height: Percent(50),
	})

	tree.AddChild(root, fixedChild)
	tree.AddChild(root, percentChild)

	tree.Compute(400, 600)

	assertRect(t, tree, root, "deferred-root", 0, 0, 400, 200)
	assertRect(t, tree, fixedChild, "fixed-child", 0, 0, 100, 200)
	assertRect(t, tree, percentChild, "percent-child", 100, 0, 100, 100)
}

// ---------------------------------------------------------------------------
// 7. Deeply Nested Flexbox Hierarchy (10 Levels of Nesting)
// ---------------------------------------------------------------------------

func TestDeeplyNestedFlexHierarchy(t *testing.T) {
	tree := NewTree()

	current := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionColumn,
		Width:         Px(1000),
		Height:        Px(1000),
		Padding:       Edges{Top: Px(10), Left: Px(10), Right: Px(10), Bottom: Px(10)},
	})
	root := current

	// Build 10 levels of alternating row/col flex containers
	for depth := 1; depth <= 10; depth++ {
		dir := FlexDirectionRow
		if depth%2 == 1 {
			dir = FlexDirectionColumn
		}
		child := tree.NewNode(Style{
			Display:       DisplayFlex,
			FlexDirection: dir,
			FlexGrow:      1,
			Padding:       Edges{Top: Px(5), Left: Px(5), Right: Px(5), Bottom: Px(5)},
		})
		tree.AddChild(current, child)
		current = child
	}

	// Leaf item
	leaf := tree.NewNode(Style{
		FlexGrow: 1,
	})
	tree.AddChild(current, leaf)

	tree.Compute(1000, 1000)

	// Verify root and leaf rects are sane and positive
	rRoot := tree.ComputedRect(root)
	if rRoot.Width != 1000 || rRoot.Height != 1000 {
		t.Fatalf("root size mismatch: got %+v", rRoot)
	}

	rLeaf := tree.ComputedRect(leaf)
	if rLeaf.Width <= 0 || rLeaf.Height <= 0 {
		t.Fatalf("leaf got invalid dimensions: %+v", rLeaf)
	}
}

// ---------------------------------------------------------------------------
// 8. Complex Real-World Dashboard (Grid + Flex + Avatars + Aspect Ratio)
// ---------------------------------------------------------------------------

func TestComplexDashboardGridFlexHybrid(t *testing.T) {
	tree := NewTree()

	// Outer Grid: Sidebar (240px) + Main Content (1fr)
	root := tree.NewNode(Style{
		Display: DisplayGrid,
		Width:   Px(1280),
		Height:  Px(800),
		GridTemplateColumns: []Dimension{
			Px(240),
			Fr(1),
		},
		GridTemplateRows: []Dimension{
			Px(64), // Header
			Fr(1),  // Content
		},
	})

	// Sidebar (Col 1, spanning rows 1 to -1)
	sidebar := tree.NewNode(Style{
		GridColumnStart: 1,
		GridColumnEnd:   2,
		GridRowStart:    1,
		GridRowEnd:      -1,
		Display:         DisplayFlex,
		FlexDirection:   FlexDirectionColumn,
		Padding:         Edges{Top: Px(20), Left: Px(16), Right: Px(16), Bottom: Px(20)},
		Gap:             Px(12),
	})
	tree.AddChild(root, sidebar)

	// 5 Nav items inside sidebar
	for i := 0; i < 5; i++ {
		navItem := tree.NewNode(Style{
			Height: Px(40),
			Width:  Percent(100),
		})
		tree.AddChild(sidebar, navItem)
	}

	// Header (Col 2, Row 1)
	header := tree.NewNode(Style{
		GridColumnStart: 2,
		GridColumnEnd:   -1,
		GridRowStart:    1,
		GridRowEnd:      2,
		Display:         DisplayFlex,
		FlexDirection:   FlexDirectionRow,
		AlignItems:      AlignCenter,
		JustifyContent:  JustifySpaceBetween,
		Padding:         Edges{Left: Px(24), Right: Px(24)},
	})
	tree.AddChild(root, header)

	// Search bar in header (grows)
	searchBar := tree.NewNode(Style{
		FlexGrow: 1,
		Height:   Px(36),
		MaxWidth: Px(400),
	})
	tree.AddChild(header, searchBar)

	// User Avatar stack in header (overlapping negative margins)
	avatarStack := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionRow,
		Width:         Auto(),
		Height:        Auto(),
	})
	tree.AddChild(header, avatarStack)

	for i := 0; i < 3; i++ {
		marginLeft := float32(0)
		if i > 0 {
			marginLeft = -10
		}
		av := tree.NewNode(Style{
			Width:  Px(36),
			Height: Px(36),
			Margin: Edges{Left: Px(marginLeft)},
		})
		tree.AddChild(avatarStack, av)
	}

	// Main content area (Col 2, Row 2)
	content := tree.NewNode(Style{
		GridColumnStart: 2,
		GridColumnEnd:   -1,
		GridRowStart:    2,
		GridRowEnd:      3,
		Display:         DisplayFlex,
		FlexDirection:   FlexDirectionColumn,
		Padding:         Edges{Top: Px(24), Left: Px(24), Right: Px(24), Bottom: Px(24)},
		Gap:             Px(20),
	})
	tree.AddChild(root, content)

	// Cards row: 3 aspect-ratio cards
	cardsRow := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionRow,
		Gap:           Px(16),
		Width:         Percent(100),
	})
	tree.AddChild(content, cardsRow)

	var cardIDs [3]NodeID
	for i := 0; i < 3; i++ {
		cardIDs[i] = tree.NewNode(Style{
			FlexGrow:    1,
			AspectRatio: 16.0 / 9.0,
		})
		tree.AddChild(cardsRow, cardIDs[i])
	}

	tree.Compute(1280, 800)

	// Verify Sidebar: X=0, W=240, H=800
	assertRect(t, tree, sidebar, "sidebar", 0, 0, 240, 800)

	// Verify Header: X=240, Y=0, W=1040 (1280 - 240), H=64
	assertRect(t, tree, header, "header", 240, 0, 1040, 64)

	// Verify Avatar stack width: 36 + 26 + 26 = 88px
	assertRect(t, tree, avatarStack, "avatarStack", 928, 14, 88, 36)

	// Verify Cards Row: width = 1040 - 48 (padding) = 992px
	// 3 cards + 2 gaps (16*2=32) -> 992 - 32 = 960 -> 320px per card!
	// 320 / (16/9) = 180px height per card!
	assertRect(t, tree, cardIDs[0], "card-0", 0, 0, 320, 180)
	assertRect(t, tree, cardIDs[1], "card-1", 336, 0, 320, 180)
	assertRect(t, tree, cardIDs[2], "card-2", 672, 0, 320, 180)
}

// ---------------------------------------------------------------------------
// 9. Extreme Constraints & Zero-size Edge Cases
// ---------------------------------------------------------------------------

func TestStressAndExtremeConstraints(t *testing.T) {
	tree := NewTree()

	// 0x0 container with negative margins and max/min violations
	root := tree.NewNode(Style{
		Display:   DisplayFlex,
		Width:     Px(0),
		Height:    Px(0),
		MinWidth:  Px(100),
		MaxWidth:  Px(50), // Min > Max inverted constraint
		MinHeight: Px(100),
		MaxHeight: Px(50),
	})

	child := tree.NewNode(Style{
		Width:  Px(200),
		Height: Px(200),
		Margin: Edges{Left: Px(-50), Top: Px(-50)},
	})
	tree.AddChild(root, child)

	tree.Compute(0, 0)

	r := tree.ComputedRect(root)
	if r.Width < 0 || r.Height < 0 {
		t.Fatalf("negative rect produced: %+v", r)
	}
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkLayoutComplexDashboard(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		tree := NewTree()
		root := tree.NewNode(Style{
			Display: DisplayGrid,
			Width:   Px(1280),
			Height:  Px(800),
			GridTemplateColumns: []Dimension{
				Px(240),
				Fr(1),
			},
			GridTemplateRows: []Dimension{
				Px(64),
				Fr(1),
			},
		})
		sidebar := tree.NewNode(Style{
			GridColumnStart: 1,
			GridColumnEnd:   2,
			GridRowStart:    1,
			GridRowEnd:      -1,
			Display:         DisplayFlex,
			FlexDirection:   FlexDirectionColumn,
		})
		tree.AddChild(root, sidebar)
		for j := 0; j < 10; j++ {
			navItem := tree.NewNode(Style{Height: Px(40)})
			tree.AddChild(sidebar, navItem)
		}

		header := tree.NewNode(Style{
			GridColumnStart: 2,
			GridColumnEnd:   -1,
			GridRowStart:    1,
			GridRowEnd:      2,
			Display:         DisplayFlex,
			FlexDirection:   FlexDirectionRow,
		})
		tree.AddChild(root, header)

		content := tree.NewNode(Style{
			GridColumnStart: 2,
			GridColumnEnd:   -1,
			GridRowStart:    2,
			GridRowEnd:      3,
			Display:         DisplayFlex,
			FlexDirection:   FlexDirectionRow,
		})
		tree.AddChild(root, content)

		for j := 0; j < 4; j++ {
			card := tree.NewNode(Style{
				FlexGrow:    1,
				AspectRatio: 16.0 / 9.0,
			})
			tree.AddChild(content, card)
		}

		tree.Compute(1280, 800)
	}
}

// ---------------------------------------------------------------------------
// 10. Navbar Pattern: margin-left: auto on Flex Items (W3C §8.1)
// ---------------------------------------------------------------------------

func TestNavbarMarginAutoMainAxis(t *testing.T) {
	tree := NewTree()

	nav := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionRow,
		Width:         Px(800),
		Height:        Px(60),
	})

	logo := tree.NewNode(Style{
		Width:  Px(100),
		Height: Px(40),
	})

	login := tree.NewNode(Style{
		Width:  Px(80),
		Height: Px(40),
		Margin: Edges{Left: Auto()},
	})

	tree.AddChild(nav, logo)
	tree.AddChild(nav, login)

	tree.Compute(800, 60)

	// In 800px bar: logo at 0 (100px), login pushed to 800 - 80 = 720px
	assertRect(t, tree, logo, "navbar-logo", 0, 0, 100, 40)
	assertRect(t, tree, login, "navbar-login-auto-margin", 720, 0, 80, 40)
}

// ---------------------------------------------------------------------------
// 11. Cross-Axis margin: auto Vertical Centering (W3C §8.1)
// ---------------------------------------------------------------------------

func TestCrossAxisMarginAutoCentering(t *testing.T) {
	tree := NewTree()

	container := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionRow,
		Width:         Px(500),
		Height:        Px(200),
	})

	item := tree.NewNode(Style{
		Width:  Px(100),
		Height: Px(40),
		Margin: Edges{Top: Auto(), Bottom: Auto()},
	})

	tree.AddChild(container, item)
	tree.Compute(500, 200)

	// Centered vertically: (200 - 40) / 2 = 80px
	assertRect(t, tree, item, "vertical-centered-margin-auto", 0, 80, 100, 40)
}

// ---------------------------------------------------------------------------
// 12. FlexWrap with RowGap vs ColumnGap
// ---------------------------------------------------------------------------

func TestFlexWrapRowAndColumnGaps(t *testing.T) {
	tree := NewTree()

	container := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionRow,
		FlexWrap:      FlexWrapWrap,
		RowGap:        Px(20),
		ColumnGap:     Px(10),
		Width:         Px(200),
		Height:        Auto(),
	})

	i1 := tree.NewNode(Style{Width: Px(90), Height: Px(50)})
	i2 := tree.NewNode(Style{Width: Px(90), Height: Px(50)})
	i3 := tree.NewNode(Style{Width: Px(90), Height: Px(50)})

	tree.AddChild(container, i1)
	tree.AddChild(container, i2)
	tree.AddChild(container, i3)

	tree.Compute(200, 600)

	// Line 1: i1 at X=0, i2 at X=100 (90 + 10). Height = 50.
	// Line 2: i3 at X=0, Y=70 (50 + 20 row-gap). Height = 50.
	// Container total height = 50 + 20 + 50 = 120px.
	assertRect(t, tree, container, "wrap-container", 0, 0, 200, 120)
	assertRect(t, tree, i1, "wrap-i1", 0, 0, 90, 50)
	assertRect(t, tree, i2, "wrap-i2", 100, 0, 90, 50)
	assertRect(t, tree, i3, "wrap-i3", 0, 70, 90, 50)
}

// ---------------------------------------------------------------------------
// 13. Absolute Modal Centering (inset: 0, margin: auto)
// ---------------------------------------------------------------------------

func TestAbsoluteModalInsetMarginAuto(t *testing.T) {
	tree := NewTree()

	root := tree.NewNode(Style{
		Display: DisplayFlex,
		Width:   Px(800),
		Height:  Px(600),
	})

	modal := tree.NewNode(Style{
		Position: PositionAbsolute,
		Inset:    Edges{Left: Px(0), Right: Px(0), Top: Px(0), Bottom: Px(0)},
		Width:    Px(300),
		Height:   Px(200),
		Margin:   Edges{Left: Auto(), Right: Auto(), Top: Auto(), Bottom: Auto()},
	})

	tree.AddChild(root, modal)
	tree.Compute(800, 600)

	// Centered: X = (800 - 300) / 2 = 250, Y = (600 - 200) / 2 = 200
	assertRect(t, tree, modal, "centered-modal", 250, 200, 300, 200)
}

// ---------------------------------------------------------------------------
// 14. CSS Grid Auto Track Content Sizing
// ---------------------------------------------------------------------------

func TestGridAutoTracksContentSized(t *testing.T) {
	tree := NewTree()

	grid := tree.NewNode(Style{
		Display: DisplayGrid,
		Width:   Px(400),
		Height:  Px(100),
		GridTemplateColumns: []Dimension{
			Auto(),
			Auto(),
		},
	})

	badge := tree.NewNode(Style{
		Width:  Px(60),
		Height: Px(40),
	})
	card := tree.NewNode(Style{
		Width:  Px(200),
		Height: Px(40),
	})

	tree.AddChild(grid, badge)
	tree.AddChild(grid, card)

	tree.Compute(400, 100)

	// Auto tracks size to fit items: Col 0 = 60px, Col 1 = 200px
	assertRect(t, tree, badge, "grid-badge-auto-track", 0, 0, 60, 40)
	assertRect(t, tree, card, "grid-card-auto-track", 60, 0, 200, 40)
}

// ---------------------------------------------------------------------------
// 15. min-width: 0 Explicit Override Allowing Flex Item Shrinking
// ---------------------------------------------------------------------------

func TestFlexItemExplicitMinZeroTextTruncation(t *testing.T) {
	tree := NewTree()

	root := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionRow,
		Width:         Px(200),
		Height:        Px(50),
	})

	// flex_item has an inner child of 300px, but min-width: 0 allows it to shrink
	flexItem := tree.NewNode(Style{
		Display:    DisplayFlex,
		Width:      Px(200),
		Height:     Px(50),
		MinWidth:   Px(0),
		FlexShrink: 1,
	})
	inner := tree.NewNode(Style{
		Width:  Px(300),
		Height: Px(50),
	})
	tree.AddChild(flexItem, inner)

	other := tree.NewNode(Style{
		Width:      Px(100),
		Height:     Px(50),
		FlexShrink: 1,
	})

	tree.AddChild(root, flexItem)
	tree.AddChild(root, other)

	tree.Compute(200, 50)

	// Available: 200. Total flex basis = 200 + 100 = 300.
	// Overflows by 100. Shrink factors weighted: 200*1 + 100*1 = 300.
	// flexItem shrinks by 100 * (200/300) = 66.67 -> width 133.33.
	// other shrinks by 100 * (100/300) = 33.33 -> width 66.67.
	assertRect(t, tree, flexItem, "flex-item-shrunk-below-content", 0, 0, 133.33, 50)
	assertRect(t, tree, other, "other-item", 133.33, 0, 66.67, 50)
}

// ---------------------------------------------------------------------------
// 16. Grid Item Margins & Auto-Margin Centering (W3C Grid §10.1)
// ---------------------------------------------------------------------------

func TestGridItemMarginsAndCentering(t *testing.T) {
	tree := NewTree()

	grid := tree.NewNode(Style{
		Display:             DisplayGrid,
		Width:               Px(200),
		Height:              Px(200),
		GridTemplateColumns: []Dimension{Px(200)},
		GridTemplateRows:    []Dimension{Px(200)},
	})

	// Fixed margins
	item1 := tree.NewNode(Style{
		Width:  Px(100),
		Height: Px(100),
		Margin: EdgesPx(15, 20, 25, 10), // Top 15, Right 20, Bottom 25, Left 10
	})

	tree.AddChild(grid, item1)
	tree.Compute(200, 200)

	// In 200x200 cell, item at (10, 15) with size 100x100
	assertRect(t, tree, item1, "grid-item-margins", 10, 15, 100, 100)

	// Test margin: auto centering in Grid
	tree2 := NewTree()
	grid2 := tree2.NewNode(Style{
		Display:             DisplayGrid,
		Width:               Px(200),
		Height:              Px(200),
		GridTemplateColumns: []Dimension{Px(200)},
		GridTemplateRows:    []Dimension{Px(200)},
	})
	item2 := tree2.NewNode(Style{
		Width:  Px(100),
		Height: Px(100),
		Margin: Edges{Left: Auto(), Right: Auto(), Top: Auto(), Bottom: Auto()},
	})
	tree2.AddChild(grid2, item2)
	tree2.Compute(200, 200)

	// Auto margins center: X = (200 - 100)/2 = 50, Y = (200 - 100)/2 = 50
	assertRect(t, tree2, item2, "grid-item-margin-auto-center", 50, 50, 100, 100)

	// Test align/justify center with margins
	tree3 := NewTree()
	grid3 := tree3.NewNode(Style{
		Display:             DisplayGrid,
		Width:               Px(200),
		Height:              Px(200),
		GridTemplateColumns: []Dimension{Px(200)},
		GridTemplateRows:    []Dimension{Px(200)},
		AlignItems:          AlignCenter,
		JustifyItems:        AlignCenter,
	})
	item3 := tree3.NewNode(Style{
		Width:  Px(100),
		Height: Px(100),
		Margin: EdgesPx(15, 20, 25, 10),
	})
	tree3.AddChild(grid3, item3)
	tree3.Compute(200, 200)

	// Center with margins: X = 10 + (200 - 30 - 100)*0.5 = 45, Y = 15 + (200 - 40 - 100)*0.5 = 45
	assertRect(t, tree3, item3, "grid-item-center-with-margins", 45, 45, 100, 100)
}

// ---------------------------------------------------------------------------
// 17. Grid Item Aspect Ratio
// ---------------------------------------------------------------------------

func TestGridItemAspectRatio(t *testing.T) {
	tree := NewTree()

	grid := tree.NewNode(Style{
		Display:             DisplayGrid,
		Width:               Px(400),
		Height:              Px(300),
		GridTemplateColumns: []Dimension{Px(200), Px(200)},
		GridTemplateRows:    []Dimension{Px(300)},
		AlignItems:          AlignFlexStart,
	})

	// W=160, H=auto, AR=16/9 -> H=90
	item1 := tree.NewNode(Style{
		Width:       Px(160),
		Height:      Auto(),
		AspectRatio: 16.0 / 9.0,
	})

	// H=90, W=auto, AR=16/9 -> W=160
	item2 := tree.NewNode(Style{
		Width:       Auto(),
		Height:      Px(90),
		AspectRatio: 16.0 / 9.0,
	})

	tree.AddChild(grid, item1)
	tree.AddChild(grid, item2)
	tree.Compute(400, 300)

	assertRect(t, tree, item1, "grid-aspect-ratio-height-from-width", 0, 0, 160, 90)
	assertRect(t, tree, item2, "grid-aspect-ratio-width-from-height", 200, 0, 160, 90)
}

// ---------------------------------------------------------------------------
// 18. Grid Item Position Relative Offset
// ---------------------------------------------------------------------------

func TestGridItemPositionRelativeOffset(t *testing.T) {
	tree := NewTree()

	grid := tree.NewNode(Style{
		Display:             DisplayGrid,
		Width:               Px(200),
		Height:              Px(200),
		GridTemplateColumns: []Dimension{Px(200)},
		GridTemplateRows:    []Dimension{Px(200)},
	})

	item := tree.NewNode(Style{
		Position: PositionRelative,
		Inset:    Edges{Left: Px(15), Top: Px(25), Right: Auto(), Bottom: Auto()},
		Width:    Px(100),
		Height:   Px(100),
	})

	tree.AddChild(grid, item)
	tree.Compute(200, 200)

	assertRect(t, tree, item, "grid-item-relative-offset", 15, 25, 100, 100)
}

// ---------------------------------------------------------------------------
// 19. FlexBasis Priority Over AspectRatio
// ---------------------------------------------------------------------------

func TestFlexBasisPriorityOverAspectRatio(t *testing.T) {
	tree := NewTree()

	root := tree.NewNode(Style{
		Display: DisplayFlex,
		Width:   Px(500),
		Height:  Px(200),
	})

	// FlexBasis is 200px, which must take precedence over Height(100)*AR(16/9)=177.7px
	item := tree.NewNode(Style{
		FlexBasis:   Px(200),
		Width:       Auto(),
		Height:      Px(100),
		AspectRatio: 16.0 / 9.0,
		FlexGrow:    0,
		FlexShrink:  0,
	})

	tree.AddChild(root, item)
	tree.Compute(500, 200)

	assertRect(t, tree, item, "flex-basis-priority", 0, 0, 200, 100)
}

