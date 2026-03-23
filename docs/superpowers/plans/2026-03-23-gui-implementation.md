# Claude Squad GUI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a native macOS GUI for claude-squad using Fyne + fyne-io/terminal, providing a multi-pane IDE-style interface with embedded terminals and a persistent session sidebar.

**Architecture:** Fyne GUI app launched via `cs gui` subcommand. Reuses existing session/tmux/git/storage/config packages. Each terminal pane runs a `fyne-io/terminal` widget connected to a tmux session PTY via `ConnectPTY()`. Panes are arranged in a binary tree of HSplit/VSplit containers. Sidebar shows grouped session list with status polling.

**Tech Stack:** Go, Fyne v2 (`fyne.io/fyne/v2`), fyne-io/terminal (`fyne.io/terminal`), existing tmux/session/git packages.

**Spec:** `docs/superpowers/specs/2026-03-23-gui-secondary-ui-design.md`

---

## File Structure

```
gui/
  app.go              - Fyne app setup, window creation, main event loop, status polling
  theme.go            - Custom dark Fyne theme (Catppuccin Mocha)
  hotkeys.go          - Ctrl+Shift shortcut registration and dispatch
  sidebar/
    sidebar.go        - Sidebar container (header + session list + bottom bar)
    session_list.go   - Grouped session list widget with Active/Paused sections
  panes/
    manager.go        - Binary tree pane layout, split/close/navigate operations
    pane.go           - Single pane widget (header bar + terminal or empty state)
    terminal_conn.go  - Connects fyne-io/terminal to tmux PTY, handles resize propagation
  dialogs/
    new_session.go    - New session dialog (name, prompt, branch, profile)
    confirm.go        - Generic confirmation dialog
session/tmux/
  tmux.go             - Add ConnectPTY() and DisconnectPTY() methods (existing file, minor additions)
main.go              - Add `gui` subcommand to cobra (existing file, minor addition)
```

---

### Task 1: Add Dependencies and GUI Subcommand

**Files:**
- Modify: `main.go:146-163` (add gui subcommand in `init()`)
- Modify: `go.mod` (add fyne dependencies)

- [ ] **Step 1: Add Fyne dependencies**

```bash
cd /Users/jadams/go/src/bitbucket.org/vervemotion/claude-squad
go get fyne.io/fyne/v2@latest
go get fyne.io/terminal@latest
```

- [ ] **Step 2: Create minimal gui/app.go placeholder**

Create `gui/app.go`:
```go
package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/widget"
)

// Run starts the GUI application.
func Run(program string, autoYes bool) error {
	a := app.New()
	w := a.NewWindow("Claude Squad")
	w.SetContent(widget.NewLabel("Claude Squad GUI - Coming Soon"))
	w.Resize(fyne.NewSize(1200, 800))
	w.ShowAndRun()
	return nil
}
```

- [ ] **Step 3: Add gui subcommand to main.go**

Add the following in `main.go` — add `guiCmd` variable after `versionCmd` and register it in `init()`:

```go
guiCmd = &cobra.Command{
	Use:   "gui",
	Short: "Launch the Claude Squad GUI",
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Initialize(false)
		defer log.Close()

		currentDir, err := filepath.Abs(".")
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		if !git.IsGitRepo(currentDir) {
			return fmt.Errorf("error: claude-squad must be run from within a git repository")
		}

		cfg := config.LoadConfig()
		program := cfg.GetProgram()
		if programFlag != "" {
			program = programFlag
		}
		autoYes := cfg.AutoYes
		if autoYesFlag {
			autoYes = true
		}

		return gui.Run(program, autoYes)
	},
}
```

In `init()`, add: `rootCmd.AddCommand(guiCmd)`

Import `"claude-squad/gui"`.

- [ ] **Step 4: Verify it compiles and runs**

```bash
go build -o cs . && ./cs gui
```

Expected: A Fyne window opens with "Claude Squad GUI - Coming Soon" label.

- [ ] **Step 5: Commit**

```bash
git add gui/app.go main.go go.mod go.sum
git commit -m "feat(gui): add Fyne dependencies and gui subcommand scaffold"
```

---

### Task 2: Custom Dark Theme

**Files:**
- Create: `gui/theme.go`

- [ ] **Step 1: Create the custom theme**

Create `gui/theme.go` implementing `fyne.Theme` with Catppuccin Mocha colors:

```go
package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Color constants (Catppuccin Mocha palette)
var (
	colorBase     = color.NRGBA{R: 0x1e, G: 0x1e, B: 0x2e, A: 0xff}
	colorMantle   = color.NRGBA{R: 0x18, G: 0x18, B: 0x25, A: 0xff}
	colorSurface0 = color.NRGBA{R: 0x31, G: 0x32, B: 0x44, A: 0xff}
	colorText     = color.NRGBA{R: 0xcd, G: 0xd6, B: 0xf4, A: 0xff}
	colorSubtext  = color.NRGBA{R: 0xa6, G: 0xad, B: 0xc8, A: 0xff}
	colorOverlay  = color.NRGBA{R: 0x6c, G: 0x70, B: 0x86, A: 0xff}
	colorMauve    = color.NRGBA{R: 0xcb, G: 0xa6, B: 0xf7, A: 0xff}
	colorGreen    = color.NRGBA{R: 0xa6, G: 0xe3, B: 0xa1, A: 0xff}
	colorYellow   = color.NRGBA{R: 0xf9, G: 0xe2, B: 0xaf, A: 0xff}
	colorRed      = color.NRGBA{R: 0xf3, G: 0x8b, B: 0xa8, A: 0xff}
	colorBlue     = color.NRGBA{R: 0x89, G: 0xb4, B: 0xfa, A: 0xff}
)

type squadTheme struct{}

var _ fyne.Theme = (*squadTheme)(nil)

func (s *squadTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return colorBase
	case theme.ColorNameForeground:
		return colorText
	case theme.ColorNamePrimary:
		return colorMauve
	case theme.ColorNameFocus:
		return colorMauve
	case theme.ColorNameSelection:
		return colorSurface0
	case theme.ColorNameSeparator:
		return colorSurface0
	case theme.ColorNameInputBackground:
		return colorMantle
	case theme.ColorNameMenuBackground:
		return colorMantle
	case theme.ColorNameOverlayBackground:
		return colorMantle
	case theme.ColorNameHeaderBackground:
		return colorMantle
	case theme.ColorNameButton:
		return colorSurface0
	case theme.ColorNameScrollBar:
		return colorOverlay
	case theme.ColorNameHover:
		return colorSurface0
	case theme.ColorNameDisabled:
		return colorOverlay
	case theme.ColorNameError:
		return colorRed
	default:
		return theme.DefaultTheme().Color(name, theme.VariantDark)
	}
}

func (s *squadTheme) Font(style fyne.TextStyle) fyne.Resource {
	if style.Monospace {
		return theme.DefaultTheme().Font(style)
	}
	return theme.DefaultTheme().Font(style)
}

func (s *squadTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (s *squadTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}
```

- [ ] **Step 2: Apply the theme in gui/app.go**

Update `gui/app.go` — after `app.New()`, add:
```go
a.Settings().SetTheme(&squadTheme{})
```

- [ ] **Step 3: Verify the theme renders**

```bash
go build -o cs . && ./cs gui
```

Expected: Window opens with dark background, purple accents.

- [ ] **Step 4: Commit**

```bash
git add gui/theme.go gui/app.go
git commit -m "feat(gui): add custom Catppuccin Mocha dark theme"
```

---

### Task 3: ConnectPTY / DisconnectPTY on TmuxSession

**Files:**
- Modify: `session/tmux/tmux.go` (add two new methods)
- Create: `session/tmux/tmux_gui_test.go`

- [ ] **Step 1: Write the test for ConnectPTY**

Create `session/tmux/tmux_gui_test.go`:

```go
package tmux

import (
	"os/exec"
	"testing"
)

func TestConnectPTY(t *testing.T) {
	// Create a real tmux session for testing
	sessionName := "test_connectpty"
	tmuxName := toClaudeSquadTmuxName(sessionName)

	// Clean up any leftover session
	_ = exec.Command("tmux", "kill-session", "-t", tmuxName).Run()

	ts := NewTmuxSession(sessionName, "bash")
	tmpDir := t.TempDir()
	if err := ts.Start(tmpDir); err != nil {
		t.Fatalf("failed to start tmux session: %v", err)
	}
	defer ts.Close()

	// ConnectPTY should return a valid file
	ptmx, err := ts.ConnectPTY()
	if err != nil {
		t.Fatalf("ConnectPTY failed: %v", err)
	}
	if ptmx == nil {
		t.Fatal("ConnectPTY returned nil file")
	}

	// Should be able to write to the PTY
	_, err = ptmx.Write([]byte("echo hello\n"))
	if err != nil {
		t.Fatalf("failed to write to PTY: %v", err)
	}

	// DisconnectPTY should clean up without error
	if err := ts.DisconnectPTY(ptmx); err != nil {
		t.Fatalf("DisconnectPTY failed: %v", err)
	}

	// Session should still exist after disconnect
	if !ts.DoesSessionExist() {
		t.Fatal("tmux session should still exist after DisconnectPTY")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd /Users/jadams/go/src/bitbucket.org/vervemotion/claude-squad
go test ./session/tmux/ -run TestConnectPTY -v
```

Expected: FAIL — `ConnectPTY` not defined.

- [ ] **Step 3: Implement ConnectPTY and DisconnectPTY**

Add to `session/tmux/tmux.go`, after the `Restore()` method:

```go
// ConnectPTY creates a new tmux attach-session PTY for use by an external
// terminal widget (e.g., fyne-io/terminal). The caller reads/writes directly
// to the returned *os.File. Call DisconnectPTY() when done.
func (t *TmuxSession) ConnectPTY() (*os.File, error) {
	ptmx, err := t.ptyFactory.Start(exec.Command("tmux", "attach-session", "-t", t.sanitizedName))
	if err != nil {
		return nil, fmt.Errorf("error creating GUI PTY connection: %w", err)
	}
	return ptmx, nil
}

// DisconnectPTY closes a GUI-connected PTY. The underlying tmux session
// continues running. The background monitoring PTY (t.ptmx) is unaffected.
func (t *TmuxSession) DisconnectPTY(ptmx *os.File) error {
	if ptmx == nil {
		return nil
	}
	return ptmx.Close()
}
```

- [ ] **Step 4: Run the test to verify it passes**

```bash
go test ./session/tmux/ -run TestConnectPTY -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add session/tmux/tmux.go session/tmux/tmux_gui_test.go
git commit -m "feat(tmux): add ConnectPTY/DisconnectPTY for GUI terminal widget integration"
```

---

### Task 4: Terminal Connection Helper

**Files:**
- Create: `gui/panes/terminal_conn.go`

- [ ] **Step 1: Create terminal_conn.go**

This file wraps the connection between a `fyne-io/terminal` widget and a tmux session PTY, including resize propagation.

```go
package panes

import (
	"claude-squad/log"
	"claude-squad/session"
	"os"
	"sync"

	"fyne.io/fyne/v2"
	terminal "fyne.io/terminal"
	"github.com/creack/pty"
)

// TerminalConnection manages the link between a fyne-io/terminal widget
// and a tmux session PTY.
type TerminalConnection struct {
	mu       sync.Mutex
	instance *session.Instance
	term     *terminal.Terminal
	ptmx     *os.File
	listener chan terminal.Config
	closed   bool
}

// NewTerminalConnection creates a new connection but does not start it.
func NewTerminalConnection() *TerminalConnection {
	return &TerminalConnection{
		term:     terminal.New(),
		listener: make(chan terminal.Config, 1),
	}
}

// Terminal returns the fyne terminal widget for embedding in the UI.
func (tc *TerminalConnection) Terminal() *terminal.Terminal {
	return tc.term
}

// Connect attaches the terminal widget to the given instance's tmux session.
// It disconnects any existing connection first.
func (tc *TerminalConnection) Connect(inst *session.Instance) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	// Disconnect existing connection if any
	tc.disconnectLocked()

	tmuxSession := inst.GetTmuxSession()
	if tmuxSession == nil {
		return nil
	}

	ptmx, err := tmuxSession.ConnectPTY()
	if err != nil {
		return err
	}

	tc.instance = inst
	tc.ptmx = ptmx
	tc.closed = false

	// Start resize listener
	tc.term.AddListener(tc.listener)
	go tc.resizeLoop()

	// Connect terminal widget to PTY
	go func() {
		if err := tc.term.RunWithConnection(ptmx, ptmx); err != nil {
			log.InfoLog.Printf("terminal connection ended: %v", err)
		}
	}()

	return nil
}

// Disconnect closes the PTY connection and cleans up.
func (tc *TerminalConnection) Disconnect() {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.disconnectLocked()
}

func (tc *TerminalConnection) disconnectLocked() {
	if tc.ptmx == nil || tc.closed {
		return
	}
	tc.closed = true

	if tc.instance != nil {
		tmuxSession := tc.instance.GetTmuxSession()
		if tmuxSession != nil {
			if err := tmuxSession.DisconnectPTY(tc.ptmx); err != nil {
				log.InfoLog.Printf("disconnect PTY error: %v", err)
			}
		}
	}
	tc.ptmx = nil
	tc.instance = nil
}

// Instance returns the currently connected instance, or nil.
func (tc *TerminalConnection) Instance() *session.Instance {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	return tc.instance
}

// resizeLoop propagates terminal widget size changes to the tmux PTY.
func (tc *TerminalConnection) resizeLoop() {
	var lastRows, lastCols uint
	for config := range tc.listener {
		if config.Rows == lastRows && config.Columns == lastCols {
			continue
		}
		lastRows, lastCols = config.Rows, config.Columns

		tc.mu.Lock()
		if tc.ptmx != nil && !tc.closed {
			if err := pty.Setsize(tc.ptmx, &pty.Winsize{
				Rows: uint16(config.Rows),
				Cols: uint16(config.Columns),
			}); err != nil {
				log.InfoLog.Printf("failed to resize PTY: %v", err)
			}
		}
		tc.mu.Unlock()
	}
}
```

- [ ] **Step 2: Verify it compiles**

This requires `Instance.GetTmuxSession()` which does not exist yet. Add it to `session/instance.go`:

```go
// GetTmuxSession returns the underlying tmux session, or nil if not started.
func (i *Instance) GetTmuxSession() *tmux.TmuxSession {
	if !i.started {
		return nil
	}
	return i.tmuxSession
}
```

```bash
go build ./gui/panes/
```

Expected: Compiles successfully.

- [ ] **Step 3: Commit**

```bash
git add gui/panes/terminal_conn.go session/instance.go
git commit -m "feat(gui): add terminal connection helper for PTY-to-widget bridge"
```

---

### Task 5: Single Pane Widget

**Files:**
- Create: `gui/panes/pane.go`

- [ ] **Step 1: Create pane.go**

A pane is a container with a header bar and either a terminal widget or an empty-state label.

```go
package panes

import (
	"claude-squad/session"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// Pane represents a single terminal pane with a header bar.
type Pane struct {
	container *fyne.Container
	header    *fyne.Container
	titleLabel *widget.Label
	statusIcon *canvas.Text
	branchLabel *widget.Label
	conn      *TerminalConnection
	focused   bool
	onFocus   func(*Pane)
}

// NewPane creates a new empty pane.
func NewPane(onFocus func(*Pane)) *Pane {
	p := &Pane{
		conn:        NewTerminalConnection(),
		titleLabel:  widget.NewLabel("No session"),
		statusIcon:  canvas.NewText("", colorOverlay),
		branchLabel: widget.NewLabel(""),
		onFocus:     onFocus,
	}

	p.branchLabel.TextStyle = fyne.TextStyle{Italic: true}
	p.statusIcon.TextSize = 14

	p.header = container.NewHBox(
		p.statusIcon,
		p.titleLabel,
		layout.NewSpacer(),
		p.branchLabel,
	)

	emptyLabel := widget.NewLabel("Select a session to open here")
	emptyLabel.Alignment = fyne.TextAlignCenter
	emptyContent := container.NewCenter(emptyLabel)

	p.container = container.NewBorder(p.header, nil, nil, nil, emptyContent)
	return p
}

// Widget returns the fyne canvas object for this pane.
func (p *Pane) Widget() fyne.CanvasObject {
	return p.container
}

// OpenSession connects this pane to a session's tmux terminal.
func (p *Pane) OpenSession(inst *session.Instance) error {
	if err := p.conn.Connect(inst); err != nil {
		return fmt.Errorf("failed to open session: %w", err)
	}
	p.titleLabel.SetText(inst.Title)
	p.branchLabel.SetText(inst.Branch)
	p.updateStatus(inst)

	// Replace the pane content with the terminal widget
	p.container.Objects = []fyne.CanvasObject{p.header, p.conn.Terminal()}
	p.container.Layout = layout.NewBorderLayout(p.header, nil, nil, nil)
	p.container.Refresh()
	return nil
}

// CloseSession disconnects the terminal and shows empty state.
func (p *Pane) CloseSession() {
	p.conn.Disconnect()
	p.titleLabel.SetText("No session")
	p.branchLabel.SetText("")
	p.statusIcon.Text = ""

	emptyLabel := widget.NewLabel("Select a session to open here")
	emptyLabel.Alignment = fyne.TextAlignCenter

	p.container.Objects = []fyne.CanvasObject{p.header, container.NewCenter(emptyLabel)}
	p.container.Layout = layout.NewBorderLayout(p.header, nil, nil, nil)
	p.container.Refresh()
}

// Instance returns the connected instance, or nil.
func (p *Pane) Instance() *session.Instance {
	return p.conn.Instance()
}

// SetFocused updates the visual focus state of this pane.
func (p *Pane) SetFocused(focused bool) {
	p.focused = focused
}

// IsFocused returns whether this pane is focused.
func (p *Pane) IsFocused() bool {
	return p.focused
}

// updateStatus updates the status icon based on instance state.
func (p *Pane) updateStatus(inst *session.Instance) {
	if inst == nil {
		p.statusIcon.Text = ""
		return
	}
	switch inst.Status {
	case session.Running:
		p.statusIcon.Text = "●"
		p.statusIcon.Color = colorGreen
	case session.Ready:
		p.statusIcon.Text = "▲"
		p.statusIcon.Color = colorYellow
	case session.Loading:
		p.statusIcon.Text = "◌"
		p.statusIcon.Color = colorOverlay
	case session.Paused:
		p.statusIcon.Text = "⏸"
		p.statusIcon.Color = colorOverlay
	}
	p.statusIcon.Refresh()
}

// UpdateStatus refreshes the pane header from current instance state.
func (p *Pane) UpdateStatus() {
	inst := p.conn.Instance()
	p.updateStatus(inst)
}

// Disconnect cleans up the terminal connection.
func (p *Pane) Disconnect() {
	p.conn.Disconnect()
}
```

- [ ] **Step 2: Add color variables accessible from panes package**

The pane uses theme colors. Create a small shared file or export them from theme.go. The simplest approach: capitalize the color variables in `gui/theme.go` so they're exported, or move them to a shared `gui/colors.go`. For now, reference them directly in pane.go by duplicating the needed subset:

Add at the top of `gui/panes/pane.go`:
```go
import "image/color"

var (
	colorGreen   = color.NRGBA{R: 0xa6, G: 0xe3, B: 0xa1, A: 0xff}
	colorYellow  = color.NRGBA{R: 0xf9, G: 0xe2, B: 0xaf, A: 0xff}
	colorOverlay = color.NRGBA{R: 0x6c, G: 0x70, B: 0x86, A: 0xff}
)
```

- [ ] **Step 3: Verify it compiles**

```bash
go build ./gui/panes/
```

Expected: Compiles successfully.

- [ ] **Step 4: Commit**

```bash
git add gui/panes/pane.go
git commit -m "feat(gui): add single pane widget with header and terminal embedding"
```

---

### Task 6: Pane Manager (Binary Tree Layout)

**Files:**
- Create: `gui/panes/manager.go`

- [ ] **Step 1: Create manager.go**

The pane manager maintains a binary tree of panes and splits, tracks focus, and provides split/close/navigate operations.

```go
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
	root    *node
	focused *node
	onFocus func(*Pane) // callback when focus changes
}

// NewManager creates a new pane manager with a single empty pane.
func NewManager(onFocus func(*Pane)) *Manager {
	m := &Manager{onFocus: onFocus}
	pane := NewPane(func(p *Pane) {
		m.FocusPane(p)
	})
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
	})

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

// NavigateLeft/Right/Up/Down move focus between panes.
func (m *Manager) NavigateLeft()  { m.navigate(-1, 0) }
func (m *Manager) NavigateRight() { m.navigate(1, 0) }
func (m *Manager) NavigateUp()    { m.navigate(0, -1) }
func (m *Manager) NavigateDown()  { m.navigate(0, 1) }

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
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./gui/panes/
```

Expected: Compiles successfully.

- [ ] **Step 3: Commit**

```bash
git add gui/panes/manager.go
git commit -m "feat(gui): add binary tree pane manager with split/close/navigate"
```

---

### Task 7: Sidebar Session List

**Files:**
- Create: `gui/sidebar/session_list.go`
- Create: `gui/sidebar/sidebar.go`

- [ ] **Step 1: Create session_list.go**

A grouped list widget that shows Active and Paused sessions with status icons, sorted alphabetically within each group.

```go
package sidebar

import (
	"claude-squad/session"
	"fmt"
	"image/color"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

var (
	colorGreen   = color.NRGBA{R: 0xa6, G: 0xe3, B: 0xa1, A: 0xff}
	colorYellow  = color.NRGBA{R: 0xf9, G: 0xe2, B: 0xaf, A: 0xff}
	colorOverlay = color.NRGBA{R: 0x6c, G: 0x70, B: 0x86, A: 0xff}
	colorText    = color.NRGBA{R: 0xcd, G: 0xd6, B: 0xf4, A: 0xff}
	colorSubtext = color.NRGBA{R: 0xa6, G: 0xad, B: 0xc8, A: 0xff}
)

// listEntry represents a single row in the flattened list.
type listEntry struct {
	isHeader bool
	text     string
	instance *session.Instance
}

// SessionList is a grouped, sorted session list widget.
type SessionList struct {
	widget.BaseWidget
	list        *widget.List
	entries     []listEntry
	onSelect    func(*session.Instance)
	onActivate  func(*session.Instance) // double-click
	selectedIdx int
}

// NewSessionList creates a new session list widget.
func NewSessionList(onSelect func(*session.Instance), onActivate func(*session.Instance)) *SessionList {
	sl := &SessionList{
		onSelect:    onSelect,
		onActivate:  onActivate,
		selectedIdx: -1,
	}

	sl.list = widget.NewList(
		func() int { return len(sl.entries) },
		func() fyne.CanvasObject {
			icon := canvas.NewText("●", colorGreen)
			icon.TextSize = 12
			name := widget.NewLabel("Session Name")
			name.TextStyle = fyne.TextStyle{Bold: true}
			subtitle := widget.NewLabel("Status +0/-0")
			subtitle.TextStyle = fyne.TextStyle{Italic: true}

			header := widget.NewLabelWithStyle("SECTION", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

			return container.NewStack(
				header,
				container.NewHBox(icon, container.NewVBox(name, subtitle)),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= len(sl.entries) {
				return
			}
			entry := sl.entries[id]
			stack := obj.(*fyne.Container)
			header := stack.Objects[0].(*widget.Label)
			itemBox := stack.Objects[1].(*fyne.Container)

			if entry.isHeader {
				header.SetText(entry.text)
				header.Show()
				itemBox.Hide()
				return
			}

			header.Hide()
			itemBox.Show()

			icon := itemBox.Objects[0].(*canvas.Text)
			vbox := itemBox.Objects[1].(*fyne.Container)
			name := vbox.Objects[0].(*widget.Label)
			subtitle := vbox.Objects[1].(*widget.Label)

			name.SetText(entry.instance.Title)
			sl.updateEntryStyle(entry.instance, icon, name, subtitle)
		},
	)

	sl.list.OnSelected = func(id widget.ListItemID) {
		if id >= len(sl.entries) || sl.entries[id].isHeader {
			sl.list.UnselectAll()
			return
		}
		sl.selectedIdx = id
		if sl.onSelect != nil {
			sl.onSelect(sl.entries[id].instance)
		}
	}

	sl.ExtendBaseWidget(sl)
	return sl
}

// Update rebuilds the list from the current instances.
func (sl *SessionList) Update(instances []*session.Instance) {
	var active, paused []*session.Instance
	for _, inst := range instances {
		if inst.Status == session.Paused {
			paused = append(paused, inst)
		} else {
			active = append(active, inst)
		}
	}

	sort.Slice(active, func(i, j int) bool { return active[i].Title < active[j].Title })
	sort.Slice(paused, func(i, j int) bool { return paused[i].Title < paused[j].Title })

	sl.entries = nil
	if len(active) > 0 {
		sl.entries = append(sl.entries, listEntry{isHeader: true, text: "ACTIVE"})
		for _, inst := range active {
			sl.entries = append(sl.entries, listEntry{instance: inst})
		}
	}
	if len(paused) > 0 {
		sl.entries = append(sl.entries, listEntry{isHeader: true, text: "PAUSED"})
		for _, inst := range paused {
			sl.entries = append(sl.entries, listEntry{instance: inst})
		}
	}

	sl.list.Refresh()
}

func (sl *SessionList) updateEntryStyle(inst *session.Instance, icon *canvas.Text, name *widget.Label, subtitle *widget.Label) {
	var statusText string
	switch inst.Status {
	case session.Running:
		icon.Text = "●"
		icon.Color = colorGreen
		statusText = "Running..."
	case session.Ready:
		icon.Text = "▲"
		icon.Color = colorYellow
		statusText = "Needs input"
		name.TextStyle = fyne.TextStyle{Bold: true}
	case session.Loading:
		icon.Text = "◌"
		icon.Color = colorOverlay
		statusText = "Loading..."
	case session.Paused:
		icon.Text = "⏸"
		icon.Color = colorOverlay
		statusText = "Paused"
	}
	icon.Refresh()

	diffStats := inst.GetDiffStats()
	if diffStats != nil {
		subtitle.SetText(fmt.Sprintf("%s +%d/-%d", statusText, diffStats.Added, diffStats.Removed))
	} else {
		subtitle.SetText(statusText)
	}
}

// SelectedInstance returns the currently selected instance.
func (sl *SessionList) SelectedInstance() *session.Instance {
	if sl.selectedIdx < 0 || sl.selectedIdx >= len(sl.entries) {
		return nil
	}
	return sl.entries[sl.selectedIdx].instance
}

// SelectUp moves selection up one non-header item.
func (sl *SessionList) SelectUp() {
	for i := sl.selectedIdx - 1; i >= 0; i-- {
		if !sl.entries[i].isHeader {
			sl.list.Select(i)
			return
		}
	}
}

// SelectDown moves selection down one non-header item.
func (sl *SessionList) SelectDown() {
	for i := sl.selectedIdx + 1; i < len(sl.entries); i++ {
		if !sl.entries[i].isHeader {
			sl.list.Select(i)
			return
		}
	}
}

// CreateRenderer returns the list widget as the renderer.
func (sl *SessionList) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(sl.list)
}
```

- [ ] **Step 2: Create sidebar.go**

The sidebar container wraps the session list with a title header and bottom action bar.

```go
package sidebar

import (
	"claude-squad/session"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// Sidebar is the left panel containing the session list and action buttons.
type Sidebar struct {
	container   *fyne.Container
	sessionList *SessionList
	onNew       func()
}

// NewSidebar creates a new sidebar widget.
func NewSidebar(onSelect func(*session.Instance), onActivate func(*session.Instance), onNew func()) *Sidebar {
	s := &Sidebar{
		sessionList: NewSessionList(onSelect, onActivate),
		onNew:       onNew,
	}

	title := canvas.NewText("Claude Squad", colorMauve)
	title.TextSize = 16
	title.TextStyle = fyne.TextStyle{Bold: true}
	titleContainer := container.NewPadded(title)

	newBtn := widget.NewButton("+ New", func() {
		if s.onNew != nil {
			s.onNew()
		}
	})

	bottomBar := container.NewHBox(newBtn, layout.NewSpacer())

	s.container = container.NewBorder(
		titleContainer,  // top
		bottomBar,       // bottom
		nil, nil,
		s.sessionList,   // center
	)

	return s
}

var colorMauve = color.NRGBA{R: 0xcb, G: 0xa6, B: 0xf7, A: 0xff}

// Widget returns the sidebar canvas object.
func (s *Sidebar) Widget() fyne.CanvasObject {
	return s.container
}

// Update refreshes the session list with new instance data.
func (s *Sidebar) Update(instances []*session.Instance) {
	s.sessionList.Update(instances)
}

// SelectedInstance returns the currently selected instance.
func (s *Sidebar) SelectedInstance() *session.Instance {
	return s.sessionList.SelectedInstance()
}

// SelectUp moves selection up.
func (s *Sidebar) SelectUp() { s.sessionList.SelectUp() }

// SelectDown moves selection down.
func (s *Sidebar) SelectDown() { s.sessionList.SelectDown() }
```

- [ ] **Step 3: Verify compilation**

```bash
go build ./gui/sidebar/
```

Expected: Compiles. Fix any missing imports (e.g., `"fmt"`, `"image/color"`).

- [ ] **Step 4: Commit**

```bash
git add gui/sidebar/session_list.go gui/sidebar/sidebar.go
git commit -m "feat(gui): add sidebar with grouped session list"
```

---

### Task 8: Hotkey Registration

**Files:**
- Create: `gui/hotkeys.go`

- [ ] **Step 1: Create hotkeys.go**

Registers all `Ctrl+Shift+` shortcuts at the canvas level and dispatches to handler functions.

```go
package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

const modCtrlShift = fyne.KeyModifierControl | fyne.KeyModifierShift

// Handlers is a struct of callback functions for hotkey actions.
type Handlers struct {
	NewSession      func()
	SplitVertical   func()
	SplitHorizontal func()
	ClosePane       func()
	NavigateLeft    func()
	NavigateRight   func()
	NavigateUp      func()
	NavigateDown    func()
	SidebarUp       func()
	SidebarDown     func()
	OpenInPane      func()
	KillSession     func()
	PushChanges     func()
	PauseResume     func()
	ToggleSidebar   func()
	Quit            func()
}

// RegisterHotkeys registers all Ctrl+Shift shortcuts on the given canvas.
func RegisterHotkeys(canvas fyne.Canvas, h Handlers) {
	shortcuts := []struct {
		key     fyne.KeyName
		handler func()
	}{
		{fyne.KeyN, h.NewSession},
		{fyne.KeyBackslash, h.SplitVertical},
		{fyne.KeyMinus, h.SplitHorizontal},
		{fyne.KeyW, h.ClosePane},
		{fyne.KeyLeft, h.NavigateLeft},
		{fyne.KeyRight, h.NavigateRight},
		{fyne.KeyUp, h.NavigateUp},
		{fyne.KeyDown, h.NavigateDown},
		{fyne.KeyJ, h.SidebarDown},
		{fyne.KeyK, h.SidebarUp},
		{fyne.KeyReturn, h.OpenInPane},
		{fyne.KeyD, h.KillSession},
		{fyne.KeyP, h.PushChanges},
		{fyne.KeyR, h.PauseResume},
		{fyne.KeyB, h.ToggleSidebar},
		{fyne.KeyQ, h.Quit},
	}

	for _, s := range shortcuts {
		handler := s.handler // capture for closure
		shortcut := &desktop.CustomShortcut{
			KeyName:  s.key,
			Modifier: modCtrlShift,
		}
		canvas.AddShortcut(shortcut, func(_ fyne.Shortcut) {
			if handler != nil {
				handler()
			}
		})
	}
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./gui/
```

Expected: Compiles.

- [ ] **Step 3: Commit**

```bash
git add gui/hotkeys.go
git commit -m "feat(gui): add Ctrl+Shift hotkey registration"
```

---

### Task 9: Confirmation and New Session Dialogs

**Files:**
- Create: `gui/dialogs/confirm.go`
- Create: `gui/dialogs/new_session.go`

- [ ] **Step 1: Create confirm.go**

```go
package dialogs

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
)

// ShowConfirm shows a confirmation dialog and calls onConfirm if the user accepts.
func ShowConfirm(title, message string, onConfirm func(), parent fyne.Window) {
	dialog.ShowConfirm(title, message, func(confirmed bool) {
		if confirmed && onConfirm != nil {
			onConfirm()
		}
	}, parent)
}
```

- [ ] **Step 2: Create new_session.go**

```go
package dialogs

import (
	"claude-squad/config"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// SessionOptions holds the result of the new session dialog.
type SessionOptions struct {
	Name    string
	Prompt  string
	Program string
}

// ShowNewSession shows a dialog for creating a new session.
func ShowNewSession(profiles []config.Profile, parent fyne.Window, onSubmit func(SessionOptions)) {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Session name")

	promptEntry := widget.NewMultiLineEntry()
	promptEntry.SetPlaceHolder("Initial prompt (optional)")
	promptEntry.SetMinRowsVisible(3)

	// Program/profile selector
	profileNames := make([]string, len(profiles))
	for i, p := range profiles {
		profileNames[i] = p.Name
	}
	programSelect := widget.NewSelect(profileNames, nil)
	if len(profileNames) > 0 {
		programSelect.SetSelected(profileNames[0])
	}

	items := []*widget.FormItem{
		widget.NewFormItem("Name", nameEntry),
		widget.NewFormItem("Prompt", promptEntry),
	}
	if len(profiles) > 1 {
		items = append(items, widget.NewFormItem("Program", programSelect))
	}

	d := dialog.NewForm("New Session", "Create", "Cancel", items, func(confirmed bool) {
		if !confirmed {
			return
		}
		opts := SessionOptions{
			Name:   nameEntry.Text,
			Prompt: promptEntry.Text,
		}
		// Resolve program from profile
		selected := programSelect.Selected
		for _, p := range profiles {
			if p.Name == selected {
				opts.Program = p.Program
				break
			}
		}
		if onSubmit != nil {
			onSubmit(opts)
		}
	}, parent)
	d.Resize(fyne.NewSize(500, 350))
	d.Show()
}
```

- [ ] **Step 3: Verify compilation**

```bash
go build ./gui/dialogs/
```

- [ ] **Step 4: Commit**

```bash
git add gui/dialogs/confirm.go gui/dialogs/new_session.go
git commit -m "feat(gui): add confirmation and new session dialogs"
```

---

### Task 10: Wire Everything Together in gui/app.go

**Files:**
- Modify: `gui/app.go` (complete rewrite of placeholder)

- [ ] **Step 1: Rewrite gui/app.go with full application wiring**

This is the main orchestrator — it creates the sidebar, pane manager, hotkeys, and runs the status polling loop. Instance state is held on an `appState` struct so that helper functions can mutate the slice.

```go
package gui

import (
	"claude-squad/config"
	"claude-squad/gui/dialogs"
	"claude-squad/gui/panes"
	"claude-squad/gui/sidebar"
	"claude-squad/log"
	"claude-squad/session"
	"fmt"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
)

// guiState holds mutable application state shared across callbacks.
type guiState struct {
	mu        sync.Mutex
	instances []*session.Instance
	storage   *session.Storage
}

func (s *guiState) addInstance(inst *session.Instance) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.instances = append(s.instances, inst)
}

func (s *guiState) removeInstance(title string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, inst := range s.instances {
		if inst.Title == title {
			s.instances = append(s.instances[:i], s.instances[i+1:]...)
			return
		}
	}
}

func (s *guiState) getInstances() []*session.Instance {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]*session.Instance, len(s.instances))
	copy(cp, s.instances)
	return cp
}

// Run starts the GUI application.
func Run(program string, autoYes bool) error {
	appConfig := config.LoadConfig()
	appStateConfig := config.LoadState()
	storage, err := session.NewStorage(appStateConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Load saved instances
	instances, err := storage.LoadInstances()
	if err != nil {
		return fmt.Errorf("failed to load instances: %w", err)
	}

	state := &guiState{
		instances: instances,
		storage:   storage,
	}

	a := app.New()
	a.Settings().SetTheme(&squadTheme{})
	w := a.NewWindow("Claude Squad")

	var sidebarWidget *sidebar.Sidebar
	var paneManager *panes.Manager
	var sidebarVisible bool = true

	// Pane manager
	paneManager = panes.NewManager(func(p *panes.Pane) {
		// Focus callback — could update sidebar selection
	})

	// Sidebar
	sidebarWidget = sidebar.NewSidebar(
		func(inst *session.Instance) {
			// On select — just highlights in sidebar
		},
		func(inst *session.Instance) {
			// On activate (double-click) — open in focused pane
			openSessionInFocusedPane(paneManager, inst)
		},
		func() {
			// On new
			showNewSessionDialog(w, appConfig, program, state, sidebarWidget, paneManager, autoYes)
		},
	)

	// Layout: sidebar | panes
	sidebarObj := sidebarWidget.Widget()
	rootSplit := container.NewHSplit(sidebarObj, paneManager.Widget())
	rootSplit.SetOffset(0.2)
	rootContainer := container.NewStack(rootSplit)

	// Register hotkeys
	RegisterHotkeys(w.Canvas(), Handlers{
		NewSession: func() {
			showNewSessionDialog(w, appConfig, program, state, sidebarWidget, paneManager, autoYes)
		},
		SplitVertical: func() {
			paneManager.SplitVertical()
			rootSplit.Trailing = paneManager.Widget()
			rootSplit.Refresh()
		},
		SplitHorizontal: func() {
			paneManager.SplitHorizontal()
			rootSplit.Trailing = paneManager.Widget()
			rootSplit.Refresh()
		},
		ClosePane: func() {
			paneManager.CloseFocused()
			rootSplit.Trailing = paneManager.Widget()
			rootSplit.Refresh()
		},
		NavigateLeft:  paneManager.NavigateLeft,
		NavigateRight: paneManager.NavigateRight,
		NavigateUp:    paneManager.NavigateUp,
		NavigateDown:  paneManager.NavigateDown,
		SidebarUp:     sidebarWidget.SelectUp,
		SidebarDown:   sidebarWidget.SelectDown,
		OpenInPane: func() {
			inst := sidebarWidget.SelectedInstance()
			if inst != nil {
				openSessionInFocusedPane(paneManager, inst)
			}
		},
		KillSession: func() {
			inst := sidebarWidget.SelectedInstance()
			if inst == nil {
				return
			}
			dialogs.ShowConfirm("Kill Session",
				fmt.Sprintf("Kill session '%s'?", inst.Title),
				func() {
					killSession(inst, state, sidebarWidget, paneManager)
				}, w)
		},
		PushChanges: func() {
			inst := sidebarWidget.SelectedInstance()
			if inst == nil {
				return
			}
			dialogs.ShowConfirm("Push Changes",
				fmt.Sprintf("Push changes from '%s'?", inst.Title),
				func() {
					pushSession(inst)
				}, w)
		},
		PauseResume: func() {
			inst := sidebarWidget.SelectedInstance()
			if inst == nil {
				return
			}
			togglePauseResume(inst, state, sidebarWidget)
		},
		ToggleSidebar: func() {
			sidebarVisible = !sidebarVisible
			if sidebarVisible {
				rootSplit.Leading = sidebarObj
				rootSplit.SetOffset(0.2)
			} else {
				rootSplit.Leading = container.NewStack() // empty
				rootSplit.SetOffset(0.0)
			}
			rootSplit.Refresh()
		},
		Quit: func() {
			paneManager.DisconnectAll()
			if err := state.storage.SaveInstances(state.getInstances()); err != nil {
				log.ErrorLog.Printf("failed to save instances on quit: %v", err)
			}
			a.Quit()
		},
	})

	// Initial sidebar update
	sidebarWidget.Update(state.getInstances())

	// Status polling goroutine
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			for _, inst := range state.getInstances() {
				if !inst.Started() || inst.Paused() {
					continue
				}
				inst.CheckAndHandleTrustPrompt()
				updated, prompt := inst.HasUpdated()
				if updated {
					inst.SetStatus(session.Running)
				} else if prompt {
					if autoYes {
						inst.TapEnter()
					} else {
						inst.SetStatus(session.Ready)
					}
				} else {
					inst.SetStatus(session.Ready)
				}
				if err := inst.UpdateDiffStats(); err != nil {
					log.WarningLog.Printf("failed to update diff stats: %v", err)
				}
			}
			sidebarWidget.Update(state.getInstances())
			for _, p := range paneManager.AllPanes() {
				p.UpdateStatus()
			}
		}
	}()

	w.SetContent(rootContainer)
	w.Resize(fyne.NewSize(1200, 800))
	w.SetOnClosed(func() {
		paneManager.DisconnectAll()
		state.storage.SaveInstances(state.getInstances())
	})
	w.ShowAndRun()
	return nil
}

func openSessionInFocusedPane(pm *panes.Manager, inst *session.Instance) {
	pane := pm.FocusedPane()
	if pane == nil || inst == nil {
		return
	}
	// Disconnect from any other pane showing this session
	for _, p := range pm.AllPanes() {
		if p.Instance() != nil && p.Instance().Title == inst.Title && p != pane {
			p.CloseSession()
		}
	}
	if err := pane.OpenSession(inst); err != nil {
		log.ErrorLog.Printf("failed to open session in pane: %v", err)
	}
}

func showNewSessionDialog(w fyne.Window, cfg *config.Config, defaultProgram string, state *guiState, sb *sidebar.Sidebar, pm *panes.Manager, autoYes bool) {
	dialogs.ShowNewSession(cfg.GetProfiles(), w, func(opts dialogs.SessionOptions) {
		if opts.Name == "" {
			return
		}
		prog := opts.Program
		if prog == "" {
			prog = defaultProgram
		}
		inst, err := session.NewInstance(session.InstanceOptions{
			Title:   opts.Name,
			Path:    ".",
			Program: prog,
		})
		if err != nil {
			log.ErrorLog.Printf("failed to create instance: %v", err)
			return
		}
		inst.AutoYes = autoYes
		inst.Prompt = opts.Prompt
		inst.SetStatus(session.Loading)
		state.addInstance(inst)
		sb.Update(state.getInstances())

		go func() {
			if err := inst.Start(true); err != nil {
				log.ErrorLog.Printf("failed to start instance: %v", err)
				return
			}
			if opts.Prompt != "" {
				if err := inst.SendPrompt(opts.Prompt); err != nil {
					log.ErrorLog.Printf("failed to send prompt: %v", err)
				}
				inst.Prompt = ""
			}
			sb.Update(state.getInstances())
			if err := state.storage.SaveInstances(state.getInstances()); err != nil {
				log.ErrorLog.Printf("failed to save instances: %v", err)
			}
		}()
	})
}

func killSession(inst *session.Instance, state *guiState, sb *sidebar.Sidebar, pm *panes.Manager) {
	// Close any pane showing this session
	for _, p := range pm.AllPanes() {
		if p.Instance() != nil && p.Instance().Title == inst.Title {
			p.CloseSession()
		}
	}

	if err := state.storage.DeleteInstance(inst.Title); err != nil {
		log.ErrorLog.Printf("failed to delete instance: %v", err)
	}

	state.removeInstance(inst.Title)
	sb.Update(state.getInstances())
}

func pushSession(inst *session.Instance) {
	worktree, err := inst.GetGitWorktree()
	if err != nil {
		log.ErrorLog.Printf("failed to get worktree: %v", err)
		return
	}
	commitMsg := fmt.Sprintf("[claudesquad] update from '%s' on %s", inst.Title, time.Now().Format(time.RFC822))
	if err := worktree.PushChanges(commitMsg, true); err != nil {
		log.ErrorLog.Printf("failed to push changes: %v", err)
	}
}

func togglePauseResume(inst *session.Instance, state *guiState, sb *sidebar.Sidebar) {
	if inst.Status == session.Paused {
		if err := inst.Resume(); err != nil {
			log.ErrorLog.Printf("failed to resume: %v", err)
		}
	} else {
		if err := inst.Pause(); err != nil {
			log.ErrorLog.Printf("failed to pause: %v", err)
		}
	}
	sb.Update(state.getInstances())
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build -o cs .
```

Expected: Compiles. Fix any import issues.

- [ ] **Step 3: Manual integration test**

```bash
./cs gui
```

Expected: Window opens with dark theme, empty sidebar, single empty pane. `Ctrl+Shift+N` opens new session dialog.

- [ ] **Step 4: Commit**

```bash
git add gui/app.go
git commit -m "feat(gui): wire together sidebar, pane manager, hotkeys, and status polling"
```

---

### Task 11: End-to-End Testing and Polish

**Files:**
- Various files for bug fixes discovered during testing

- [ ] **Step 1: Test new session creation flow**

1. Run `./cs gui` from a git repository
2. Press `Ctrl+Shift+N` — new session dialog should appear
3. Enter a name, click Create
4. Session should appear in sidebar as Loading, then Active
5. Click or double-click the session in the sidebar
6. Terminal pane should show Claude Code (or configured program) running

Document any issues found.

- [ ] **Step 2: Test pane splitting**

1. With a session open in a pane, press `Ctrl+Shift+\` — should split vertically
2. New pane shows empty state
3. Open a different session in the new pane
4. `Ctrl+Shift+Arrow` to navigate between panes
5. `Ctrl+Shift+W` to close a pane

Document any issues found.

- [ ] **Step 3: Test sidebar functionality**

1. Create 3+ sessions
2. Verify Active group shows alphabetically sorted
3. Pause a session — verify it moves to Paused group
4. Resume — verify it moves back to Active
5. Alarm state (Ready) — verify yellow icon appears when Claude asks a question

Document any issues found.

- [ ] **Step 4: Test session persistence**

1. Create sessions, close the GUI (`Ctrl+Shift+Q`)
2. Reopen with `./cs gui`
3. Sessions should appear in sidebar with correct statuses
4. Open the existing TUI with `./cs` — sessions should be visible there too

Document any issues found.

- [ ] **Step 5: Fix all issues found in steps 1-4**

Apply fixes as needed. Each fix gets its own commit.

- [ ] **Step 6: Final commit**

```bash
git add -A
git commit -m "fix(gui): polish and bug fixes from integration testing"
```
