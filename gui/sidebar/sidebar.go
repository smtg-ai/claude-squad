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
		titleContainer, // top
		bottomBar,      // bottom
		nil, nil,
		s.sessionList, // center
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
