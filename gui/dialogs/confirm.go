package dialogs

import (
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
