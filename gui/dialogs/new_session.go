package dialogs

import (
	"claude-squad/config"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// SessionOptions holds the result of the new session dialog.
type SessionOptions struct {
	Name    string
	Prompt  string
	Program string
	Branch  string // empty = new branch from default
	InPlace bool
}

// ShowNewSession shows a dialog for creating a new session.
func ShowNewSession(profiles []config.Profile, defaultBranch string, branches []string, parent fyne.Window, onBranchSearch func(filter string) []string, onSubmit func(SessionOptions)) {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Session name")

	promptEntry := widget.NewMultiLineEntry()
	promptEntry.SetPlaceHolder("Initial prompt (optional)")
	promptEntry.SetMinRowsVisible(3)

	// In-place toggle
	inPlaceCheck := widget.NewCheck("Run in-place (no git isolation)", nil)

	// Branch picker
	newBranchLabel := fmt.Sprintf("New branch (from %s)", defaultBranch)
	branchOptions := append([]string{newBranchLabel}, branches...)
	branchSelect := widget.NewSelect(branchOptions, nil)
	branchSelect.SetSelected(newBranchLabel)

	// Search entry for filtering branches
	branchSearch := widget.NewEntry()
	branchSearch.SetPlaceHolder("Search branches...")
	branchSearch.OnChanged = func(filter string) {
		if onBranchSearch == nil {
			return
		}
		filtered := onBranchSearch(filter)
		newOptions := append([]string{newBranchLabel}, filtered...)
		branchSelect.Options = newOptions
		branchSelect.Refresh()
	}

	branchContainer := container.NewVBox(branchSearch, branchSelect)
	branchFormItem := widget.NewFormItem("Branch", branchContainer)

	// Toggle branch picker visibility based on in-place checkbox
	inPlaceCheck.OnChanged = func(checked bool) {
		if checked {
			branchFormItem.Widget = widget.NewLabel("(disabled for in-place sessions)")
		} else {
			branchFormItem.Widget = branchContainer
		}
		branchFormItem.Widget.Refresh()
	}

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
		widget.NewFormItem("In-place", inPlaceCheck),
		branchFormItem,
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
			Name:    nameEntry.Text,
			Prompt:  promptEntry.Text,
			InPlace: inPlaceCheck.Checked,
		}

		// Resolve branch selection
		if !inPlaceCheck.Checked && branchSelect.Selected != newBranchLabel {
			opts.Branch = branchSelect.Selected
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
	d.Resize(fyne.NewSize(500, 500))
	d.Show()
}
