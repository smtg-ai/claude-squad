package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/widget"
)

// Run starts the GUI application.
func Run(program string, autoYes bool) error {
	a := app.New()
	a.Settings().SetTheme(&squadTheme{})
	w := a.NewWindow("Claude Squad")
	w.SetContent(widget.NewLabel("Claude Squad GUI - Coming Soon"))
	w.Resize(fyne.NewSize(1200, 800))
	w.ShowAndRun()
	return nil
}
