package layout

// NodeID is an index into the [Tree]'s flat node slice. It uniquely identifies
// a layout node within a single tree.
type NodeID uint32

// InvalidNode is a sentinel value meaning "no node". It is used for
// uninitialised parent/child/sibling links.
const InvalidNode NodeID = 0xFFFFFFFF

// LayoutNode stores the tree structure (parent/child/sibling links),
// styling information, and the computed layout rectangle for one node.
type LayoutNode struct {
	// Tree links — all set to InvalidNode when unlinked.
	Parent      NodeID
	FirstChild  NodeID
	LastChild   NodeID
	NextSibling NodeID
	PrevSibling NodeID
	ChildCount  uint32

	// Style holds the CSS Flexbox properties for this node.
	Style Style

	// Computed is the output rectangle, filled by [Tree.Compute].
	Computed Rect
}

// flexItem is scratch data kept per flex item during layout computation.
// It is stored in a reusable slice on [Tree] to avoid heap allocations.
type flexItem struct {
	node             NodeID
	flexBasis        float32
	hypotheticalMain float32
	flexGrow         float32
	flexShrink       float32
	frozen           bool
	targetMain       float32
	crossSize        float32
	mainOffset       float32
	crossOffset      float32
}

// flexLine tracks a single line of items in a multi-line (wrapping) flex
// container. Stored in a reusable scratch slice on [Tree].
type flexLine struct {
	startIdx   int     // index into scratch items slice
	count      int     // number of items in this line
	mainUsed   float32 // total hypothetical main size of items in this line
	crossSize  float32 // computed cross size of this line
	lineOffset float32 // cross-axis offset of this line
}

// gridItemScratch is scratch data kept per grid item during grid layout computation.
type gridItemScratch struct {
	node      NodeID
	colStart  int
	colSpan   int
	rowStart  int
	rowSpan   int
	definite  bool
}

// Tree is a flat, arena-allocated layout tree. Nodes are stored in a
// contiguous slice and referenced by [NodeID] indices. After the tree is
// built via [NewNode] and [AddChild], the [Compute] method performs the
// full layout pass with zero heap allocations.
type Tree struct {
	nodes []LayoutNode
	root  NodeID

	// Pre-allocated scratch space — reused across Compute() calls.
	scratch   []flexItem
	lines     []flexLine
	gridItems []gridItemScratch
	colSizes  []float32
	rowSizes  []float32
	occupied  []bool // grid occupancy map
}

// NewTree creates an empty layout tree.
func NewTree() *Tree {
	return &Tree{
		nodes:     make([]LayoutNode, 0, 64),
		root:      InvalidNode,
		scratch:   make([]flexItem, 0, 64),
		lines:     make([]flexLine, 0, 8),
		gridItems: make([]gridItemScratch, 0, 64),
		colSizes:  make([]float32, 0, 16),
		rowSizes:  make([]float32, 0, 16),
		occupied:  make([]bool, 0, 4096),
	}
}

// NewNode appends a new node with the given style to the tree and returns
// its [NodeID]. The first node created becomes the tree root.
func (t *Tree) NewNode(style Style) NodeID {
	id := NodeID(len(t.nodes))
	t.nodes = append(t.nodes, LayoutNode{
		Parent:      InvalidNode,
		FirstChild:  InvalidNode,
		LastChild:   InvalidNode,
		NextSibling: InvalidNode,
		PrevSibling: InvalidNode,
		Style:       style,
	})
	if t.root == InvalidNode {
		t.root = id
	}
	return id
}

// AddChild links child as the last child of parent. Both IDs must refer to
// existing nodes. Panics on out-of-range IDs.
func (t *Tree) AddChild(parent, child NodeID) {
	p := &t.nodes[parent]
	c := &t.nodes[child]

	c.Parent = parent
	c.PrevSibling = p.LastChild
	c.NextSibling = InvalidNode

	if p.FirstChild == InvalidNode {
		p.FirstChild = child
	} else {
		t.nodes[p.LastChild].NextSibling = child
	}
	p.LastChild = child
	p.ChildCount++
}

// ComputedRect returns the computed position and size of the node identified
// by id. The result is only meaningful after [Compute] has been called.
func (t *Tree) ComputedRect(id NodeID) Rect {
	return t.nodes[id].Computed
}

// Root returns the [NodeID] of the tree's root node. If no nodes have been
// added, it returns [InvalidNode].
func (t *Tree) Root() NodeID {
	return t.root
}

// NodeCount returns the total number of nodes in the tree.
func (t *Tree) NodeCount() int {
	return len(t.nodes)
}

// Children returns the immediate child node IDs of parent.
func (t *Tree) Children(parent NodeID) []NodeID {
	if int(parent) >= len(t.nodes) {
		return nil
	}
	var res []NodeID
	c := t.nodes[parent].FirstChild
	for c != InvalidNode {
		res = append(res, c)
		c = t.nodes[c].NextSibling
	}
	return res
}

// HitTest performs spatial lookup to find the deepest child node
// whose computed bounds contain the point (x, y).
func (t *Tree) HitTest(x, y float32) NodeID {
	if t.root == InvalidNode {
		return InvalidNode
	}
	return t.hitTestNode(t.root, x, y)
}

func (t *Tree) hitTestNode(id NodeID, x, y float32) NodeID {
	if int(id) >= len(t.nodes) {
		return InvalidNode
	}
	rect := t.nodes[id].Computed
	if x < rect.X || x > rect.X+rect.Width || y < rect.Y || y > rect.Y+rect.Height {
		return InvalidNode
	}

	// Check children in reverse order (topmost first)
	child := t.nodes[id].LastChild
	for child != InvalidNode {
		hit := t.hitTestNode(child, x, y)
		if hit != InvalidNode {
			return hit
		}
		child = t.nodes[child].PrevSibling
	}

	return id
}
