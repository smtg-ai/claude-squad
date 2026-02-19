package session

import (
	"os/exec"
	"runtime"
)

// NotificationsEnabled controls whether desktop notifications are sent.
// Set from config at startup.
var NotificationsEnabled bool = true

// SendNotification sends a desktop notification. It is fire-and-forget:
// the command is started but we do not wait for it to finish.
func SendNotification(title, body string) {
	if !NotificationsEnabled {
		return
	}

	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("osascript", "-e",
			`display notification "`+body+`" with title "`+title+`"`)
		_ = cmd.Start()
	case "linux":
		if path, err := exec.LookPath("notify-send"); err == nil {
			cmd := exec.Command(path, title, body)
			_ = cmd.Start()
		}
	}
}
