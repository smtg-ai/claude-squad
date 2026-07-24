package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

type ErrBox struct {
	height, width int
	err           error
	// info is a neutral status message shown when there is no error.
	info string
}

var errStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
	Light: "#FF0000",
	Dark:  "#FF0000",
})

var infoStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
	Light: "#808080",
	Dark:  "#808080",
})

func NewErrBox() *ErrBox {
	return &ErrBox{}
}

func (e *ErrBox) SetError(err error) {
	e.err = err
	e.info = ""
}

// SetInfo shows a neutral status message (e.g. "copied selection").
func (e *ErrBox) SetInfo(info string) {
	e.info = info
	e.err = nil
}

func (e *ErrBox) Clear() {
	e.err = nil
	e.info = ""
}

func (e *ErrBox) SetSize(width, height int) {
	e.width = width
	e.height = height
}

func (e *ErrBox) String() string {
	if e.err == nil && e.info != "" {
		info := e.info
		if runewidth.StringWidth(info) > e.width-3 && e.width-3 >= 0 {
			info = runewidth.Truncate(info, e.width-3, "...")
		}
		return lipgloss.Place(e.width, e.height, lipgloss.Center, lipgloss.Center, infoStyle.Render(info))
	}
	var err string
	if e.err != nil {
		err = e.err.Error()
		lines := strings.Split(err, "\n")
		err = strings.Join(lines, "//")
		if runewidth.StringWidth(err) > e.width-3 && e.width-3 >= 0 {
			err = runewidth.Truncate(err, e.width-3, "...")
		}
	}
	return lipgloss.Place(e.width, e.height, lipgloss.Center, lipgloss.Center, errStyle.Render(err))
}
