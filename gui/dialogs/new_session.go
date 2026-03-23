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
