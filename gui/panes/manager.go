package panes

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

// node represents a node in the pane binary tree.
type node struct {
	// Leaf node fields
	pane *Pane

	// Split node fields
	split      *container.Split
	horizontal bool // true = HSplit, false = VSplit
	left       *node
	right      *node
	parent     *node
}

func (n *node) isLeaf() bool {
	return n.pane != nil
}

// Manager manages the binary tree of panes.
type Manager struct {
	root         *node
	focused      *node
	onFocus      func(*Pane) // callback when focus changes
	registerKeys ShortcutRegistrar
}

// NewManager creates a new pane manager with a single empty pane.
func NewManager(onFocus func(*Pane), registerKeys ShortcutRegistrar) *Manager {
	m := &Manager{onFocus: onFocus, registerKeys: registerKeys}
	pane := NewPane(func(p *Pane) {
		m.FocusPane(p)
	}, registerKeys)
	pane.SetFocused(true)
	m.root = &node{pane: pane}
	m.focused = m.root
	return m
}

// Widget returns the root fyne canvas object for the pane layout.
func (m *Manager) Widget() fyne.CanvasObject {
	return m.nodeWidget(m.root)
}

func (m *Manager) nodeWidget(n *node) fyne.CanvasObject {
	if n.isLeaf() {
		return n.pane.Widget()
	}
	return n.split
}

// FocusedPane returns the currently focused pane.
func (m *Manager) FocusedPane() *Pane {
	if m.focused != nil && m.focused.isLeaf() {
		return m.focused.pane
	}
	return nil
}

// FocusPane sets focus to the pane matching p.
func (m *Manager) FocusPane(p *Pane) {
	old := m.FocusedPane()
	if old != nil {
		old.SetFocused(false)
	}
	m.focused = m.findNode(m.root, p)
	if m.focused != nil && m.focused.isLeaf() {
		m.focused.pane.SetFocused(true)
	}
	if m.onFocus != nil {
		m.onFocus(p)
	}
}

// SplitVertical splits the focused pane vertically (side by side).
func (m *Manager) SplitVertical() *Pane {
	return m.split(true)
}

// SplitHorizontal splits the focused pane horizontally (top/bottom).
func (m *Manager) SplitHorizontal() *Pane {
	return m.split(false)
}

func (m *Manager) split(horizontal bool) *Pane {
	if m.focused == nil || !m.focused.isLeaf() {
		return nil
	}

	newPane := NewPane(func(p *Pane) {
		m.FocusPane(p)
	}, m.registerKeys)

	oldNode := m.focused
	parent := oldNode.parent

	// Create left and right leaf nodes
	leftNode := &node{pane: oldNode.pane}
	rightNode := &node{pane: newPane}

	// Create split container
	var splitContainer *container.Split
	if horizontal {
		splitContainer = container.NewHSplit(leftNode.pane.Widget(), rightNode.pane.Widget())
	} else {
		splitContainer = container.NewVSplit(leftNode.pane.Widget(), rightNode.pane.Widget())
	}
	splitContainer.SetOffset(0.5)

	// Create new split node replacing the old leaf
	splitNode := &node{
		split:      splitContainer,
		horizontal: horizontal,
		left:       leftNode,
		right:      rightNode,
		parent:     parent,
	}
	leftNode.parent = splitNode
	rightNode.parent = splitNode

	// Replace in parent
	if parent == nil {
		m.root = splitNode
	} else {
		if parent.left == oldNode {
			parent.left = splitNode
		} else {
			parent.right = splitNode
		}
		m.rebuildSplit(parent)
	}

	return newPane
}

// CloseFocused closes the focused pane and expands the sibling.
func (m *Manager) CloseFocused() {
	if m.focused == nil || !m.focused.isLeaf() {
		return
	}

	focusedNode := m.focused
	parent := focusedNode.parent

	// If this is the root (only pane), just clear it
	if parent == nil {
		focusedNode.pane.CloseSession()
		return
	}

	// Find sibling
	var sibling *node
	if parent.left == focusedNode {
		sibling = parent.right
	} else {
		sibling = parent.left
	}

	// Disconnect the closed pane
	focusedNode.pane.Disconnect()

	// Replace parent with sibling in grandparent
	grandparent := parent.parent
	sibling.parent = grandparent
	if grandparent == nil {
		m.root = sibling
	} else {
		if grandparent.left == parent {
			grandparent.left = sibling
		} else {
			grandparent.right = sibling
		}
		m.rebuildSplit(grandparent)
	}

	// Focus the first leaf of the sibling
	m.focused = m.firstLeaf(sibling)
	if m.focused.isLeaf() {
		m.focused.pane.SetFocused(true)
		if m.onFocus != nil {
			m.onFocus(m.focused.pane)
		}
	}
}

// NavigateLeft moves focus to the left pane.
func (m *Manager) NavigateLeft() { m.navigate(-1, 0) }

// NavigateRight moves focus to the right pane.
func (m *Manager) NavigateRight() { m.navigate(1, 0) }

// NavigateUp moves focus to the pane above.
func (m *Manager) NavigateUp() { m.navigate(0, -1) }

// NavigateDown moves focus to the pane below.
func (m *Manager) NavigateDown() { m.navigate(0, 1) }

func (m *Manager) navigate(dx, dy int) {
	if m.focused == nil || m.focused.parent == nil {
		return
	}
	parent := m.focused.parent

	// Simple navigation: if parent is a horizontal split, left/right navigate.
	// If vertical split, up/down navigate.
	if parent.horizontal && dx != 0 {
		if dx < 0 && parent.right == m.focused {
			m.FocusPane(m.lastLeaf(parent.left).pane)
		} else if dx > 0 && parent.left == m.focused {
			m.FocusPane(m.firstLeaf(parent.right).pane)
		}
	} else if !parent.horizontal && dy != 0 {
		if dy < 0 && parent.right == m.focused {
			m.FocusPane(m.lastLeaf(parent.left).pane)
		} else if dy > 0 && parent.left == m.focused {
			m.FocusPane(m.firstLeaf(parent.right).pane)
		}
	}
}

// AllPanes returns all leaf panes in the tree.
func (m *Manager) AllPanes() []*Pane {
	var panes []*Pane
	m.collectPanes(m.root, &panes)
	return panes
}

func (m *Manager) collectPanes(n *node, panes *[]*Pane) {
	if n == nil {
		return
	}
	if n.isLeaf() {
		*panes = append(*panes, n.pane)
		return
	}
	m.collectPanes(n.left, panes)
	m.collectPanes(n.right, panes)
}

// DisconnectAll disconnects all terminal connections.
func (m *Manager) DisconnectAll() {
	for _, p := range m.AllPanes() {
		p.Disconnect()
	}
}

func (m *Manager) findNode(n *node, p *Pane) *node {
	if n == nil {
		return nil
	}
	if n.isLeaf() && n.pane == p {
		return n
	}
	if found := m.findNode(n.left, p); found != nil {
		return found
	}
	return m.findNode(n.right, p)
}

func (m *Manager) firstLeaf(n *node) *node {
	if n.isLeaf() {
		return n
	}
	return m.firstLeaf(n.left)
}

func (m *Manager) lastLeaf(n *node) *node {
	if n.isLeaf() {
		return n
	}
	return m.lastLeaf(n.right)
}

func (m *Manager) rebuildSplit(n *node) {
	if n == nil || n.isLeaf() {
		return
	}
	leftWidget := m.nodeWidget(n.left)
	rightWidget := m.nodeWidget(n.right)
	n.split.Leading = leftWidget
	n.split.Trailing = rightWidget
	n.split.Refresh()
}
