package ebiten

import (
	"github.com/kuntal-devrat/plume/layout"
)

// ClickHandler is invoked when a pointer click is completed on a layout node.
type ClickHandler func()

// HoverHandler is invoked when pointer hover state changes on a layout node.
type HoverHandler func(hovered bool)

// UIManager manages layout trees, mouse hit-testing, and event routing
// for interactive game UI elements.
type UIManager struct {
	tree           *layout.Tree
	clickHandlers  map[layout.NodeID]ClickHandler
	hoverHandlers  map[layout.NodeID]HoverHandler
	hoveredNode    layout.NodeID
	activeNode     layout.NodeID
}

// NewUIManager creates a UI manager bound to the given layout tree.
func NewUIManager(tree *layout.Tree) *UIManager {
	return &UIManager{
		tree:          tree,
		clickHandlers: make(map[layout.NodeID]ClickHandler),
		hoverHandlers: make(map[layout.NodeID]HoverHandler),
		hoveredNode:   layout.InvalidNode,
		activeNode:    layout.InvalidNode,
	}
}

// Tree returns the bound layout tree.
func (m *UIManager) Tree() *layout.Tree {
	return m.tree
}

// OnClick registers a callback for click events on the specified node.
func (m *UIManager) OnClick(node layout.NodeID, handler ClickHandler) {
	m.clickHandlers[node] = handler
}

// OnHover registers a callback for hover transitions on the specified node.
func (m *UIManager) OnHover(node layout.NodeID, handler HoverHandler) {
	m.hoverHandlers[node] = handler
}

// HitTest performs reverse-order spatial lookup to find the deepest
// child node whose computed bounds contain the point (x, y).
func (m *UIManager) HitTest(x, y float32) layout.NodeID {
	if m.tree == nil {
		return layout.InvalidNode
	}
	return m.tree.HitTest(x, y)
}

// SetActive sets the tree currently managed.
func (m *UIManager) SetTree(t *layout.Tree) {
	m.tree = t
}

// HandlePointerMove updates hover states and triggers hover callbacks.
func (m *UIManager) HandlePointerMove(x, y float32) layout.NodeID {
	hit := m.HitTest(x, y)

	if hit != m.hoveredNode {
		if m.hoveredNode != layout.InvalidNode {
			if h, ok := m.hoverHandlers[m.hoveredNode]; ok {
				h(false)
			}
		}
		m.hoveredNode = hit
		if hit != layout.InvalidNode {
			if h, ok := m.hoverHandlers[hit]; ok {
				h(true)
			}
		}
	}
	return hit
}

// HandlePointerDown records active pressed node.
func (m *UIManager) HandlePointerDown(x, y float32) layout.NodeID {
	hit := m.HitTest(x, y)
	m.activeNode = hit
	return hit
}

// HandlePointerUp triggers ClickHandler if released on the active node.
func (m *UIManager) HandlePointerUp(x, y float32) bool {
	hit := m.HitTest(x, y)
	if hit != layout.InvalidNode && hit == m.activeNode {
		if handler, ok := m.clickHandlers[hit]; ok {
			handler()
			m.activeNode = layout.InvalidNode
			return true
		}
	}
	m.activeNode = layout.InvalidNode
	return false
}
