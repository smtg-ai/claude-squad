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
	colorGreen   = color.NRGBA{R: 0xa6, G: 0xe3, B: 0xa1, A: 0xff}
	colorYellow  = color.NRGBA{R: 0xf9, G: 0xe2, B: 0xaf, A: 0xff}
	colorOverlay = color.NRGBA{R: 0x6c, G: 0x70, B: 0x86, A: 0xff}
)

// Pane represents a single terminal pane with a header bar.
type Pane struct {
	container   *fyne.Container
	header      *fyne.Container
	titleLabel  *widget.Label
	statusIcon  *canvas.Text
	branchLabel *widget.Label
	hintLabel   *widget.Label
	conn        *TerminalConnection
	focused     bool
	onFocus     func(*Pane)
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
	p.hintLabel = widget.NewLabel(hintText())
	p.hintLabel.TextStyle = fyne.TextStyle{Italic: true}

	p.header = container.NewHBox(
		p.statusIcon,
		p.titleLabel,
		layout.NewSpacer(),
		p.hintLabel,
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

func hintText() string {
	mod := "Ctrl+Shift"
	if runtime.GOOS == "darwin" {
		mod = "⌘⇧"
	}
	return fmt.Sprintf("Split: %s+\\  %s+-  |  Close: %s+W  |  Nav: %s+←→↑↓", mod, mod, mod, mod)
}
