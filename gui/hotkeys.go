package gui

import (
	"claude-squad/log"
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

// shortcutDef pairs a key with its handler for reuse across registration targets.
type shortcutDef struct {
	key     fyne.KeyName
	handler func()
}

// modifiers we register for — both Ctrl+Shift and Cmd+Shift
var shortcutModifiers = []fyne.KeyModifier{
	fyne.KeyModifierControl | fyne.KeyModifierShift,
	fyne.KeyModifierSuper | fyne.KeyModifierShift,
}

func buildShortcuts(h Handlers) []shortcutDef {
	return []shortcutDef{
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
}

// registerShortcutsOn adds all hotkey shortcuts to the given target.
// The target can be a Canvas, ShortcutHandler, or anything with AddShortcut.
func registerShortcutsOn(target interface {
	AddShortcut(fyne.Shortcut, func(fyne.Shortcut))
}, defs []shortcutDef) {
	for _, s := range defs {
		handler := s.handler
		key := s.key
		for _, mod := range shortcutModifiers {
			shortcut := &desktop.CustomShortcut{
				KeyName:  key,
				Modifier: mod,
			}
			target.AddShortcut(shortcut, func(_ fyne.Shortcut) {
				log.InfoLog.Printf("hotkey fired: key=%s mod=%d", key, mod)
				if handler != nil {
					handler()
				}
			})
		}
	}
}

// RegisterHotkeys registers all shortcuts on the canvas (works when no terminal has focus).
// Returns the shortcut definitions so they can also be registered on terminal widgets.
func RegisterHotkeys(canvas fyne.Canvas, h Handlers) []shortcutDef {
	defs := buildShortcuts(h)
	registerShortcutsOn(canvas, defs)

	// Debug: log key events that reach the canvas
	canvas.SetOnTypedKey(func(ev *fyne.KeyEvent) {
		log.InfoLog.Printf("canvas key event: name=%q", ev.Name)
	})

	return defs
}

// RegisterTerminalShortcuts registers the hotkey shortcuts on a terminal widget
// so they fire even when the terminal has keyboard focus.
func RegisterTerminalShortcuts(target interface {
	AddShortcut(fyne.Shortcut, func(fyne.Shortcut))
}, defs []shortcutDef) {
	registerShortcutsOn(target, defs)
}
