package panes

import (
	"claude-squad/session"
	"fmt"
	"image/color"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

var (
	colorGreen      = color.NRGBA{R: 0xa6, G: 0xe3, B: 0xa1, A: 0xff}
	colorYellow     = color.NRGBA{R: 0xf9, G: 0xe2, B: 0xaf, A: 0xff}
	colorOverlay    = color.NRGBA{R: 0x6c, G: 0x70, B: 0x86, A: 0xff}
	colorFocusBorder = color.NRGBA{R: 0x89, G: 0xb4, B: 0xfa, A: 0xff} // blue accent
	colorDimBorder   = color.NRGBA{R: 0x45, G: 0x47, B: 0x5a, A: 0xff}
)

// ShortcutRegistrar registers hotkey shortcuts on a target that supports AddShortcut.
type ShortcutRegistrar func(target ShortcutAdder)

// ShortcutAdder is anything with an AddShortcut method (Canvas, ShortcutHandler, etc).
type ShortcutAdder interface {
	AddShortcut(shortcut fyne.Shortcut, handler func(fyne.Shortcut))
}

// Pane represents a single terminal pane with a header bar.
type Pane struct {
	container    *fyne.Container // outer: stack(focusBorder, inner, overlay)
	inner        *fyne.Container // border layout with header + content
	header       *fyne.Container
	titleLabel   *widget.Label
	statusIcon   *canvas.Text
	branchLabel  *widget.Label
	hintLabel    *widget.Label
	focusBorder  *canvas.Rectangle
	overlay      *tapOverlay
	conn         *TerminalConnection
	canvas       fyne.Canvas
	focused      bool
	onFocus      func(*Pane)
	registerKeys ShortcutRegistrar
}

// tapOverlay is an invisible full-pane overlay that intercepts clicks.
// It fires the onTap callback and then forwards keyboard focus to an
// optional focusable widget (the terminal) so typing still works.
type tapOverlay struct {
	widget.BaseWidget
	onTap     func()
	focusable fyne.Focusable // set when a terminal is connected
	canvas    fyne.Canvas
}

func newTapOverlay(c fyne.Canvas, onTap func()) *tapOverlay {
	t := &tapOverlay{canvas: c, onTap: onTap}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tapOverlay) Tapped(_ *fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap()
	}
	if t.focusable != nil && t.canvas != nil {
		t.canvas.Focus(t.focusable)
	}
}

func (t *tapOverlay) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

// NewPane creates a new empty pane.
func NewPane(onFocus func(*Pane), registerKeys ShortcutRegistrar, c fyne.Canvas) *Pane {
	p := &Pane{
		conn:         NewTerminalConnection(),
		registerKeys: registerKeys,
		canvas:       c,
		titleLabel:  widget.NewLabel("No session"),
		statusIcon:  canvas.NewText("", colorOverlay),
		branchLabel: widget.NewLabel(""),
		onFocus:     onFocus,
	}

	p.branchLabel.TextStyle = fyne.TextStyle{Italic: true}
	p.statusIcon.TextSize = 14
	p.hintLabel = widget.NewLabel(hintText())
	p.hintLabel.TextStyle = fyne.TextStyle{Italic: true}

	p.header = container.NewHBox(
		p.statusIcon,
		p.titleLabel,
		layout.NewSpacer(),
		p.hintLabel,
		p.branchLabel,
	)

	p.focusBorder = canvas.NewRectangle(colorDimBorder)
	p.focusBorder.StrokeWidth = 2
	p.focusBorder.StrokeColor = colorDimBorder
	p.focusBorder.FillColor = color.Transparent

	// Transparent overlay on top of everything — catches clicks to focus this pane
	p.overlay = newTapOverlay(c, func() {
		if p.onFocus != nil {
			p.onFocus(p)
		}
	})

	emptyLabel := widget.NewLabel("Select a session to open here")
	emptyLabel.Alignment = fyne.TextAlignCenter
	emptyContent := container.NewCenter(emptyLabel)

	p.inner = container.NewBorder(p.header, nil, nil, nil, emptyContent)
	// Stack order: border (back), content (middle), overlay (front — intercepts taps)
	p.container = container.NewStack(p.focusBorder, p.inner, p.overlay)
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

	// Register hotkey shortcuts on the terminal widget so they work when it has focus
	if p.registerKeys != nil {
		p.registerKeys(p.conn.Terminal())
	}

	// Tell the overlay to forward keyboard focus to this terminal after tap
	p.overlay.focusable = p.conn.Terminal()

	// Replace the pane content with the terminal widget
	p.inner.Objects = []fyne.CanvasObject{p.header, p.conn.Terminal()}
	p.inner.Layout = layout.NewBorderLayout(p.header, nil, nil, nil)
	p.inner.Refresh()
	return nil
}

// CloseSession disconnects the terminal and shows empty state.
func (p *Pane) CloseSession() {
	p.conn.Disconnect()
	p.overlay.focusable = nil
	p.titleLabel.SetText("No session")
	p.branchLabel.SetText("")
	p.statusIcon.Text = ""

	emptyLabel := widget.NewLabel("Select a session to open here")
	emptyLabel.Alignment = fyne.TextAlignCenter

	p.inner.Objects = []fyne.CanvasObject{p.header, container.NewCenter(emptyLabel)}
	p.inner.Layout = layout.NewBorderLayout(p.header, nil, nil, nil)
	p.inner.Refresh()
}

// Instance returns the connected instance, or nil.
func (p *Pane) Instance() *session.Instance {
	return p.conn.Instance()
}

// SetFocused updates the visual focus state of this pane.
func (p *Pane) SetFocused(focused bool) {
	p.focused = focused
	if focused {
		p.focusBorder.StrokeColor = colorFocusBorder
	} else {
		p.focusBorder.StrokeColor = colorDimBorder
	}
	p.focusBorder.Refresh()
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

func hintText() string {
	mod := "Ctrl+Shift"
	if runtime.GOOS == "darwin" {
		mod = "⌘⇧"
	}
	return fmt.Sprintf("Split: %s+\\  %s+-  |  Close: %s+W  |  Nav: %s+←→↑↓", mod, mod, mod, mod)
}
