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
