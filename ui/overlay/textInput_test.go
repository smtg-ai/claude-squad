package overlay

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func keyRunes(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestPathInputTabCompletesPath(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, root, "projects")

	o := NewPathInputOverlay("Directory", filepath.Join(root, "proj"))
	shouldClose, _ := o.HandleKeyPress(tea.KeyMsg{Type: tea.KeyTab})

	if shouldClose {
		t.Error("Tab should not close the overlay")
	}
	want := filepath.Join(root, "projects") + string(filepath.Separator)
	if got := o.GetValue(); got != want {
		t.Errorf("GetValue() = %q, want %q", got, want)
	}
}

func TestPathInputTabShowsCandidatesWhenAmbiguous(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, root, "backend", "backfill")

	o := NewPathInputOverlay("Directory", filepath.Join(root, "ba"))
	o.SetSize(80, 10)
	o.HandleKeyPress(tea.KeyMsg{Type: tea.KeyTab})

	rendered := o.Render()
	if !strings.Contains(rendered, "backend") || !strings.Contains(rendered, "backfill") {
		t.Errorf("Render() should list candidates, got:\n%s", rendered)
	}
}

func TestPathInputEnterSubmitsDirectly(t *testing.T) {
	o := NewPathInputOverlay("Directory", "/some/path")
	shouldClose, _ := o.HandleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})

	if !shouldClose {
		t.Error("Enter should close the overlay")
	}
	if !o.IsSubmitted() {
		t.Error("Enter should submit the overlay")
	}
}

func TestPathInputEscCancels(t *testing.T) {
	o := NewPathInputOverlay("Directory", "")
	shouldClose, _ := o.HandleKeyPress(tea.KeyMsg{Type: tea.KeyEsc})

	if !shouldClose || !o.IsCanceled() {
		t.Error("Esc should close and cancel the overlay")
	}
}

func TestPathInputErrorShownAndClearedOnTyping(t *testing.T) {
	o := NewPathInputOverlay("Directory", "/nope")
	o.SetSize(80, 10)

	o.SetError("not a git repository: /nope")
	if !strings.Contains(o.Render(), "not a git repository") {
		t.Error("Render() should show the error message")
	}

	o.HandleKeyPress(keyRunes("x"))
	if strings.Contains(o.Render(), "not a git repository") {
		t.Error("typing should clear the error message")
	}
}

func TestPathInputResetSubmittedAllowsRetry(t *testing.T) {
	o := NewPathInputOverlay("Directory", "/nope")
	o.HandleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	if !o.IsSubmitted() {
		t.Fatal("precondition: overlay should be submitted")
	}

	o.ResetSubmitted()
	if o.IsSubmitted() {
		t.Error("ResetSubmitted() should clear the submitted flag")
	}

	// The overlay must still accept input and resubmit after a reset.
	o.HandleKeyPress(keyRunes("2"))
	shouldClose, _ := o.HandleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	if !shouldClose || !o.IsSubmitted() {
		t.Error("overlay should submit again after ResetSubmitted()")
	}
}
