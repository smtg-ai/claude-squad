package dialogs

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
)

// ShowConfirm shows a confirmation dialog and calls onConfirm if the user accepts.
func ShowConfirm(title, message string, onConfirm func(), parent fyne.Window) {
	dialog.ShowConfirm(title, message, func(confirmed bool) {
		if confirmed && onConfirm != nil {
			onConfirm()
		}
	}, parent)
}

// ShowError shows an error dialog with the given title and message.
func ShowError(title, message string, parent fyne.Window) {
	dialog.ShowError(fmt.Errorf("%s: %s", title, message), parent)
}
