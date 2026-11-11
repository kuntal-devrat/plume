// Package layout implements a pure-Go W3C CSS Flexbox layout engine.
//
// The engine uses a flat arena-based tree structure ([Tree]) for cache-friendly
// traversal and zero-allocation layout computation after initial tree construction.
//
// Usage:
//
//	tree := layout.NewTree()
//	root := tree.NewNode(layout.Style{
//	    Display:       layout.DisplayFlex,
//	    FlexDirection: layout.FlexDirectionRow,
//	    Width:         layout.Px(800),
//	    Height:        layout.Px(600),
//	})
//	child := tree.NewNode(layout.Style{FlexGrow: 1})
//	tree.AddChild(root, child)
//	tree.Compute(800, 600)
//	fmt.Println(tree.ComputedRect(child)) // {0 0 800 600}
package layout

// Rect represents the computed position and size of a layout node
// after the layout algorithm has run. All values are in absolute pixels
// relative to the node's parent.
type Rect struct {
	X      float32 // Horizontal offset from the parent's content-box left edge.
	Y      float32 // Vertical offset from the parent's content-box top edge.
	Width  float32 // Computed width in pixels.
	Height float32 // Computed height in pixels.
}
