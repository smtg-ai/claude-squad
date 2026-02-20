package session

import (
	"os/exec"
	"runtime"
	"strings"
)

// NotificationsEnabled controls whether desktop notifications are sent.
// Set from config at startup.
var NotificationsEnabled = true

// sanitizeNotification strips control characters and escapes special chars for AppleScript strings.
func sanitizeNotification(s string) string {
	// Strip control characters (newlines, tabs, etc.) that could break AppleScript
	var b strings.Builder
	for _, r := range s {
		if r >= 0x20 && r != 0x7F {
			b.WriteRune(r)
		}
	}
	s = b.String()
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
			`display notification "`+sanitizeNotification(body)+`" with title "`+sanitizeNotification(title)+`"`)
		_ = cmd.Start()
	case "linux":
		if path, err := exec.LookPath("notify-send"); err == nil {
			cmd := exec.Command(path, title, body)
			_ = cmd.Start()
		}
	}
}
