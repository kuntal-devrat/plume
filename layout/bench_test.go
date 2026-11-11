package layout

import "testing"

// BenchmarkFlexbox1000Nodes benchmarks layout computation of a tree with
// ~1000 nodes arranged as 20 rows × 50 items under a column root.
// Target: < 0.2ms per operation, 0 allocs/op.
func BenchmarkFlexbox1000Nodes(b *testing.B) {
	tree := NewTree()
	root := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionColumn,
		Width:         Px(1920),
		Height:        Px(1080),
	})
	for i := 0; i < 20; i++ {
		row := tree.NewNode(Style{
			Display:       DisplayFlex,
			FlexDirection: FlexDirectionRow,
			FlexGrow:      1,
		})
		tree.AddChild(root, row)
		for j := 0; j < 50; j++ {
			item := tree.NewNode(Style{
				FlexGrow: 1,
				Height:   Px(20),
			})
			tree.AddChild(row, item)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		tree.Compute(1920, 1080)
	}
}

// BenchmarkFlexboxDeep benchmarks a deeply nested tree (10 levels).
func BenchmarkFlexboxDeep(b *testing.B) {
	tree := NewTree()
	parent := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionColumn,
		Width:         Px(800),
		Height:        Px(600),
	})
	for depth := 0; depth < 10; depth++ {
		child := tree.NewNode(Style{
			Display:       DisplayFlex,
			FlexDirection: FlexDirectionRow,
			FlexGrow:      1,
		})
		tree.AddChild(parent, child)
		parent = child
	}
	// Add leaf items to the deepest container.
	for j := 0; j < 10; j++ {
		item := tree.NewNode(Style{FlexGrow: 1, Height: Px(20)})
		tree.AddChild(parent, item)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		tree.Compute(800, 600)
	}
}
