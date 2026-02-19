package session

import (
	"os/exec"
	"runtime"
	"strings"
)

// NotificationsEnabled controls whether desktop notifications are sent.
// Set from config at startup.
var NotificationsEnabled = true

// escapeAppleScript escapes backslashes and double quotes for AppleScript strings.
func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

// SendNotification sends a desktop notification. It is fire-and-forget:
// the command is started but we do not wait for it to finish.
func SendNotification(title, body string) {
	if !NotificationsEnabled {
		return
	}

	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("osascript", "-e",
			`display notification "`+escapeAppleScript(body)+`" with title "`+escapeAppleScript(title)+`"`)
		_ = cmd.Start()
	case "linux":
		if path, err := exec.LookPath("notify-send"); err == nil {
			cmd := exec.Command(path, title, body)
			_ = cmd.Start()
		}
	}
}
