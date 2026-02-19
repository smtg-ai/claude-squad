package app

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/ByteMirror/hivemind/session/git"

	tea "github.com/charmbracelet/bubbletea"
)

// folderPickedMsg is sent when the OS folder picker returns a result.
type folderPickedMsg struct {
	path string
	err  error
}

// openFolderPicker launches the native OS folder picker asynchronously.
func (m *home) openFolderPicker() tea.Cmd {
	return func() tea.Msg {
		path, err := nativeFolderPicker()
		if err != nil {
			return folderPickedMsg{err: err}
		}
		if path == "" {
			return folderPickedMsg{} // cancelled
		}
		// Validate it's a git repo
		if !git.IsGitRepo(path) {
			return folderPickedMsg{err: fmt.Errorf("selected folder is not a git repository")}
		}
		return folderPickedMsg{path: path}
	}
}

func nativeFolderPicker() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("osascript", "-e",
			`POSIX path of (choose folder with prompt "Select a git repository")`).Output()
		if err != nil {
			// User cancelled the dialog
			return "", nil
		}
		path := strings.TrimSpace(string(out))
		// osascript returns paths with trailing slash
		path = strings.TrimRight(path, "/")
		return path, nil
	case "linux":
		out, err := exec.Command("zenity", "--file-selection", "--directory",
			"--title=Select a git repository").Output()
		if err != nil {
			// User cancelled
			return "", nil
		}
		return strings.TrimSpace(string(out)), nil
	default:
		return "", fmt.Errorf("folder picker not supported on %s", runtime.GOOS)
	}
}
