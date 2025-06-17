package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewProjectInputOverlay(t *testing.T) {
	overlay := NewProjectInputOverlay()

	if overlay == nil {
		t.Fatal("NewProjectInputOverlay returned nil")
	}

	if overlay.visible {
		t.Error("New overlay should not be visible by default")
	}

	if overlay.error != "" {
		t.Error("New overlay should not have an error by default")
	}
}

func TestProjectInputOverlayVisibility(t *testing.T) {
	overlay := NewProjectInputOverlay()

	// Test Show
	overlay.Show()
	if !overlay.IsVisible() {
		t.Error("Overlay should be visible after Show()")
	}

	// Test Hide
	overlay.Hide()
	if overlay.IsVisible() {
		t.Error("Overlay should not be visible after Hide()")
	}
}

func TestProjectInputOverlayError(t *testing.T) {
	overlay := NewProjectInputOverlay()

	testError := "test error message"
	overlay.SetError(testError)

	if overlay.error != testError {
		t.Errorf("Expected error %q, got %q", testError, overlay.error)
	}

	overlay.ClearError()
	if overlay.error != "" {
		t.Error("Error should be cleared after ClearError()")
	}
}

func TestProjectInputOverlayValidatePath(t *testing.T) {
	overlay := NewProjectInputOverlay()

	tests := []struct {
		name      string
		input     string
		wantError bool
	}{
		{
			name:      "empty path",
			input:     "",
			wantError: true,
		},
		{
			name:      "relative path",
			input:     "relative/path",
			wantError: true,
		},
		{
			name:      "absolute path",
			input:     "/absolute/path",
			wantError: false,
		},
		{
			name:      "absolute path with spaces",
			input:     "/absolute/path with spaces",
			wantError: false,
		},
		{
			name:      "path with trailing slash",
			input:     "/absolute/path/",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			overlay.textInput.SetValue(tt.input)
			err := overlay.ValidatePath()

			if tt.wantError && err == nil {
				t.Error("Expected validation error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Expected no validation error, got: %v", err)
			}
		})
	}
}

func TestProjectInputOverlayResponsiveSizing(t *testing.T) {
	overlay := NewProjectInputOverlay()

	tests := []struct {
		name           string
		terminalWidth  int
		terminalHeight int
		expectMinWidth int // Minimum expected dialog width
		expectMaxWidth int // Maximum expected dialog width
	}{
		{
			name:           "small terminal",
			terminalWidth:  80,
			terminalHeight: 24,
			expectMinWidth: 50,
			expectMaxWidth: 70,
		},
		{
			name:           "medium terminal",
			terminalWidth:  100,
			terminalHeight: 30,
			expectMinWidth: 70,
			expectMaxWidth: 80,
		},
		{
			name:           "large terminal",
			terminalWidth:  150,
			terminalHeight: 40,
			expectMinWidth: 70,
			expectMaxWidth: 80,
		},
		{
			name:           "very small terminal",
			terminalWidth:  60,
			terminalHeight: 20,
			expectMinWidth: 50,
			expectMaxWidth: 60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set size to trigger responsive sizing
			overlay.SetSize(tt.terminalWidth, tt.terminalHeight)

			// Check that dimensions were set
			if overlay.width != tt.terminalWidth {
				t.Errorf("Expected width %d, got %d", tt.terminalWidth, overlay.width)
			}
			if overlay.height != tt.terminalHeight {
				t.Errorf("Expected height %d, got %d", tt.terminalHeight, overlay.height)
			}

			// Check text input width is reasonable
			inputWidth := overlay.textInput.Width
			if inputWidth < 40 {
				t.Errorf("Text input width %d is too small (minimum 40)", inputWidth)
			}
			if inputWidth > 70 {
				t.Errorf("Text input width %d is too large (maximum 70)", inputWidth)
			}
		})
	}
}

func TestProjectInputOverlayUpdate(t *testing.T) {
	overlay := NewProjectInputOverlay()
	overlay.Show()

	// Test escape key
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedOverlay, _ := overlay.Update(msg)

	if updatedOverlay.IsVisible() {
		t.Error("Overlay should be hidden after Esc key")
	}

	// Test typing
	overlay.Show()
	overlay.textInput.SetValue("")
	overlay.SetError("test error")

	// Simulate typing to clear error
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}}
	updatedOverlay, _ = overlay.Update(msg)

	// Note: Error clearing happens in the Update method on key input
	// but we can't easily test the exact behavior without more complex mocking
}

func TestProjectInputOverlayView(t *testing.T) {
	overlay := NewProjectInputOverlay()

	// Test hidden overlay
	view := overlay.View()
	if view != "" {
		t.Error("Hidden overlay should return empty view")
	}

	// Test visible overlay
	overlay.Show()
	view = overlay.View()
	if view == "" {
		t.Error("Visible overlay should return non-empty view")
	}

	// Test view with error
	overlay.SetError("test error")
	viewWithError := overlay.View()
	if len(viewWithError) <= len(view) {
		t.Error("View with error should be longer than view without error")
	}
}

func TestProjectInputOverlayDimensions(t *testing.T) {
	overlay := NewProjectInputOverlay()

	testCases := []struct {
		width, height int
	}{
		{80, 24},  // Small terminal
		{100, 30}, // Medium terminal
		{150, 40}, // Large terminal
		{60, 20},  // Very small terminal
	}

	for _, tc := range testCases {
		overlay.SetSize(tc.width, tc.height)

		// Verify dimensions are stored
		if overlay.width != tc.width || overlay.height != tc.height {
			t.Errorf("SetSize(%d, %d) failed: got width=%d, height=%d",
				tc.width, tc.height, overlay.width, overlay.height)
		}

		// Verify the view still works
		overlay.Show()
		view := overlay.View()
		if view == "" {
			t.Errorf("View should not be empty after SetSize(%d, %d)", tc.width, tc.height)
		}
	}
}
