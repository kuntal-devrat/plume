package layout

import "testing"

func TestGridBasicFixedTracks(t *testing.T) {
	tree := NewTree()
	root := tree.NewNode(Style{
		Display:             DisplayGrid,
		Width:               Px(300),
		Height:              Px(200),
		GridTemplateColumns: []Dimension{Px(100), Px(200)},
		GridTemplateRows:    []Dimension{Px(50), Px(150)},
	})

	c1 := tree.NewNode(Style{})
	c2 := tree.NewNode(Style{})
	c3 := tree.NewNode(Style{})
	c4 := tree.NewNode(Style{})
	tree.AddChild(root, c1)
	tree.AddChild(root, c2)
	tree.AddChild(root, c3)
	tree.AddChild(root, c4)

	tree.Compute(300, 200)

	assertRect(t, tree, c1, "c1 (row 0, col 0)", 0, 0, 100, 50)
	assertRect(t, tree, c2, "c2 (row 0, col 1)", 100, 0, 200, 50)
	assertRect(t, tree, c3, "c3 (row 1, col 0)", 0, 50, 100, 150)
	assertRect(t, tree, c4, "c4 (row 1, col 1)", 100, 50, 200, 150)
}

func TestGridFrTracksWithGap(t *testing.T) {
	tree := NewTree()
	root := tree.NewNode(Style{
		Display:             DisplayGrid,
		Width:               Px(320),
		Height:              Px(100),
		GridTemplateColumns: []Dimension{Fr(1), Fr(2)},
		ColumnGap:           Px(20),
	})

	// Total width 320, gap 20 -> available space 300.
	// 1fr + 2fr = 3fr -> 1fr = 100px, 2fr = 200px.
	c1 := tree.NewNode(Style{})
	c2 := tree.NewNode(Style{})
	tree.AddChild(root, c1)
	tree.AddChild(root, c2)

	tree.Compute(320, 100)

	assertRect(t, tree, c1, "c1", 0, 0, 100, 100)
	assertRect(t, tree, c2, "c2", 120, 0, 200, 100)
}

func TestGridItemSpans(t *testing.T) {
	tree := NewTree()
	root := tree.NewNode(Style{
		Display:             DisplayGrid,
		Width:               Px(300),
		Height:              Px(200),
		GridTemplateColumns: []Dimension{Fr(1), Fr(1), Fr(1)},
		GridTemplateRows:    []Dimension{Px(50), Px(150)},
	})

	// Header spans across all 3 columns
	header := tree.NewNode(Style{
		GridColumnStart: 1,
		GridColumnSpan:  3,
	})
	col1 := tree.NewNode(Style{})
	col2 := tree.NewNode(Style{GridColumnSpan: 2})

	tree.AddChild(root, header)
	tree.AddChild(root, col1)
	tree.AddChild(root, col2)

	tree.Compute(300, 200)

	assertRect(t, tree, header, "header (spans 3 cols)", 0, 0, 300, 50)
	assertRect(t, tree, col1, "col1 (1 col)", 0, 50, 100, 150)
	assertRect(t, tree, col2, "col2 (spans 2 cols)", 100, 50, 200, 150)
}

func TestGridAutoFlowColumn(t *testing.T) {
	tree := NewTree()
	root := tree.NewNode(Style{
		Display:          DisplayGrid,
		Width:            Px(200),
		Height:           Px(200),
		GridAutoFlow:     GridAutoFlowColumn,
		GridTemplateRows: []Dimension{Px(100), Px(100)},
	})

	c1 := tree.NewNode(Style{})
	c2 := tree.NewNode(Style{})
	c3 := tree.NewNode(Style{})
	tree.AddChild(root, c1)
	tree.AddChild(root, c2)
	tree.AddChild(root, c3)

	tree.Compute(200, 200)

	// In column flow with 2 rows:
	// c1: col 0, row 0
	// c2: col 0, row 1
	// c3: col 1, row 0
	assertRect(t, tree, c1, "c1", 0, 0, 100, 100)
	assertRect(t, tree, c2, "c2", 0, 100, 100, 100)
	assertRect(t, tree, c3, "c3", 100, 0, 100, 100)
}

func BenchmarkGrid500Cells(b *testing.B) {
	tree := NewTree()
	root := tree.NewNode(Style{
		Display: DisplayGrid,
		Width:   Px(1920),
		Height:  Px(1080),
		GridTemplateColumns: []Dimension{
			Fr(1), Fr(2), Fr(1), Fr(1), Fr(2),
			Fr(1), Fr(1), Fr(2), Fr(1), Fr(1),
		}, // 10 columns
	})

	for i := 0; i < 500; i++ {
		item := tree.NewNode(Style{
			Height: Px(30),
		})
		tree.AddChild(root, item)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		tree.Compute(1920, 1080)
	}
}
