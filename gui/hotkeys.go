package gui

import (
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

// modPrefix returns the display string for the modifier combo.
func modPrefix() string {
	if runtime.GOOS == "darwin" {
		return "⌘⇧"
	}
	return "Ctrl+Shift+"
}

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

// RegisterHotkeys registers shortcuts on the given canvas.
// On macOS, registers Cmd+Shift; on other platforms, Ctrl+Shift.
// Both modifier combos are registered so either works everywhere.
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

	// Register both Ctrl+Shift and Super(Cmd)+Shift so hotkeys work on all platforms
	modifiers := []fyne.KeyModifier{
		fyne.KeyModifierControl | fyne.KeyModifierShift,
		fyne.KeyModifierSuper | fyne.KeyModifierShift,
	}

	for _, s := range shortcuts {
		handler := s.handler // capture for closure
		for _, mod := range modifiers {
			shortcut := &desktop.CustomShortcut{
				KeyName:  s.key,
				Modifier: mod,
			}
			canvas.AddShortcut(shortcut, func(_ fyne.Shortcut) {
				if handler != nil {
					handler()
				}
			})
		}
	}
}
