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
