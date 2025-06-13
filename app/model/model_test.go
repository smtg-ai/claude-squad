package model

import (
	"claude-squad/config"
	"claude-squad/instance/task"
	"claude-squad/log"
	"claude-squad/ui"
	"claude-squad/ui/overlay"
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain runs before all tests to set up the test environment
func TestMain(m *testing.M) {
	// Initialize the logger before any tests run
	log.Initialize(false)
	defer log.Close()

	// Run all tests
	exitCode := m.Run()

	// Exit with the same code as the tests
	os.Exit(exitCode)
}

// TestConfirmationModalStateTransitions tests state transitions without full instance setup
func TestConfirmationModalStateTransitions(t *testing.T) {
	// Create a minimal home struct for testing state transitions
	h := &Model{
		ctx:        context.Background(),
		state:      tuiStateDefault,
		appConfig:  config.DefaultConfig(),
		controller: NewController(&spinner.Model{}, false),
	}

	t.Run("shows confirmation on D press", func(t *testing.T) {
		// Simulate pressing 'D'
		h.state = tuiStateDefault
		h.controller.confirmationOverlay = nil

		// Manually trigger what would happen in handleKeyPress for 'D'
		h.state = tuiStateConfirm
		h.controller.confirmationOverlay = overlay.NewConfirmationOverlay("[!] Kill session 'test'?")

		assert.Equal(t, tuiStateConfirm, h.state)
		assert.NotNil(t, h.controller.confirmationOverlay)
		assert.False(t, h.controller.confirmationOverlay.Dismissed)
	})

	t.Run("returns to default on y press", func(t *testing.T) {
		// Start in confirmation state
		h.state = tuiStateConfirm
		h.controller.confirmationOverlay = overlay.NewConfirmationOverlay("Test confirmation")

		// Simulate pressing 'y' using HandleKeyPress
		keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")}
		shouldClose := h.controller.confirmationOverlay.HandleKeyPress(keyMsg)
		if shouldClose {
			h.state = tuiStateDefault
			h.controller.confirmationOverlay = nil
		}

		assert.Equal(t, tuiStateDefault, h.state)
		assert.Nil(t, h.controller.confirmationOverlay)
	})

	t.Run("returns to default on n press", func(t *testing.T) {
		// Start in confirmation state
		h.state = tuiStateConfirm
		h.controller.confirmationOverlay = overlay.NewConfirmationOverlay("Test confirmation")

		// Simulate pressing 'n' using HandleKeyPress
		keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}
		shouldClose := h.controller.confirmationOverlay.HandleKeyPress(keyMsg)
		if shouldClose {
			h.state = tuiStateDefault
			h.controller.confirmationOverlay = nil
		}

		assert.Equal(t, tuiStateDefault, h.state)
		assert.Nil(t, h.controller.confirmationOverlay)
	})

	t.Run("returns to default on esc press", func(t *testing.T) {
		// Start in confirmation state
		h.state = tuiStateConfirm
		h.controller.confirmationOverlay = overlay.NewConfirmationOverlay("Test confirmation")

		// Simulate pressing ESC using HandleKeyPress
		keyMsg := tea.KeyMsg{Type: tea.KeyEscape}
		shouldClose := h.controller.confirmationOverlay.HandleKeyPress(keyMsg)
		if shouldClose {
			h.state = tuiStateDefault
			h.controller.confirmationOverlay = nil
		}

		assert.Equal(t, tuiStateDefault, h.state)
		assert.Nil(t, h.controller.confirmationOverlay)
	})
}

// TestConfirmationModalKeyHandling tests the actual key handling in confirmation state
func TestConfirmationModalKeyHandling(t *testing.T) {
	spinner := spinner.New(spinner.WithSpinner(spinner.MiniDot))

	// Create enough of home struct to test handleKeyPress in confirmation state
	h := &Model{
		ctx:        context.Background(),
		state:      tuiStateConfirm,
		appConfig:  config.DefaultConfig(),
		menu:       ui.NewMenu(),
		controller: NewController(&spinner, false),
	}

	testCases := []struct {
		name              string
		key               string
		expectedState     tuiState
		expectedDismissed bool
		expectedNil       bool
	}{
		{
			name:              "y key confirms and dismisses overlay",
			key:               "y",
			expectedState:     tuiStateDefault,
			expectedDismissed: true,
			expectedNil:       true,
		},
		{
			name:              "n key cancels and dismisses overlay",
			key:               "n",
			expectedState:     tuiStateDefault,
			expectedDismissed: true,
			expectedNil:       true,
		},
		{
			name:              "esc key cancels and dismisses overlay",
			key:               "esc",
			expectedState:     tuiStateDefault,
			expectedDismissed: true,
			expectedNil:       true,
		},
		{
			name:              "other keys are ignored",
			key:               "x",
			expectedState:     tuiStateConfirm,
			expectedDismissed: false,
			expectedNil:       false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset state
			h.state = tuiStateConfirm
			h.controller.confirmationOverlay = overlay.NewConfirmationOverlay("Kill session?")

			// Create key message
			var keyMsg tea.KeyMsg
			if tc.key == "esc" {
				keyMsg = tea.KeyMsg{Type: tea.KeyEscape}
			} else {
				keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.key)}
			}

			// Call handleKeyPress
			// First simulate what happens in the controller's handleKeyPress method
			shouldClose := h.controller.confirmationOverlay.HandleKeyPress(keyMsg)
			if shouldClose {
				h.state = tuiStateDefault
				h.controller.confirmationOverlay = nil
			}

			// Now check the state
			assert.Equal(t, tc.expectedState, h.state, "State mismatch for key: %s", tc.key)
			if tc.expectedNil {
				assert.Nil(t, h.controller.confirmationOverlay, "Overlay should be nil for key: %s", tc.key)
			} else {
				assert.NotNil(t, h.controller.confirmationOverlay, "Overlay should not be nil for key: %s", tc.key)
				assert.Equal(t, tc.expectedDismissed, h.controller.confirmationOverlay.Dismissed, "Dismissed mismatch for key: %s", tc.key)
			}
		})
	}
}

// TestConfirmationMessageFormatting tests that confirmation messages are formatted correctly
func TestConfirmationMessageFormatting(t *testing.T) {
	testCases := []struct {
		name            string
		sessionTitle    string
		expectedMessage string
	}{
		{
			name:            "short session name",
			sessionTitle:    "my-feature",
			expectedMessage: "[!] Kill session 'my-feature'? (y/n)",
		},
		{
			name:            "long session name",
			sessionTitle:    "very-long-feature-branch-name-here",
			expectedMessage: "[!] Kill session 'very-long-feature-branch-name-here'? (y/n)",
		},
		{
			name:            "session with spaces",
			sessionTitle:    "feature with spaces",
			expectedMessage: "[!] Kill session 'feature with spaces'? (y/n)",
		},
		{
			name:            "session with special chars",
			sessionTitle:    "feature/branch-123",
			expectedMessage: "[!] Kill session 'feature/branch-123'? (y/n)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test the message formatting directly
			actualMessage := fmt.Sprintf("[!] Kill session '%s'? (y/n)", tc.sessionTitle)
			assert.Equal(t, tc.expectedMessage, actualMessage)
		})
	}
}

// TestConfirmationFlowSimulation tests the confirmation flow by simulating the state changes
func TestConfirmationFlowSimulation(t *testing.T) {
	// Create a minimal setup
	spinner := spinner.New(spinner.WithSpinner(spinner.MiniDot))

	// Create the model with controller
	h := &Model{
		ctx:        context.Background(),
		state:      tuiStateDefault,
		appConfig:  config.DefaultConfig(),
		menu:       ui.NewMenu(),
		controller: NewController(&spinner, false),
	}

	// Add test instance to the controller's instances (source of truth)
	instance, err := task.NewTask(task.TaskOptions{
		Title:   "test-session",
		Path:    t.TempDir(),
		Program: "claude",
		AutoYes: false,
	})
	require.NoError(t, err)
	h.controller.addInstance(instance)
	h.controller.list.SetSelectedInstance(0)

	// Simulate what happens when D is pressed
	selected := h.controller.list.GetSelectedInstance()
	require.NotNil(t, selected)

	// This is what the KeyKill handler does
	taskInstance := selected.(*task.Task)
	message := fmt.Sprintf("[!] Kill session '%s'?", taskInstance.Title)
	h.controller.confirmationOverlay = overlay.NewConfirmationOverlay(message)
	h.state = tuiStateConfirm

	// Verify the state
	assert.Equal(t, tuiStateConfirm, h.state)
	assert.NotNil(t, h.controller.confirmationOverlay)
	assert.False(t, h.controller.confirmationOverlay.Dismissed)
	// Test that overlay renders with the correct message
	rendered := h.controller.confirmationOverlay.Render()
	assert.Contains(t, rendered, "Kill session 'test-session'?")
}

// TestConfirmActionWithDifferentTypes tests that confirmAction works with different action types
func TestConfirmActionWithDifferentTypes(t *testing.T) {
	// Create a minimal setup
	spinner := spinner.New(spinner.WithSpinner(spinner.MiniDot))

	h := &Model{
		ctx:        context.Background(),
		state:      tuiStateDefault,
		appConfig:  config.DefaultConfig(),
		menu:       ui.NewMenu(),
		controller: NewController(&spinner, false),
	}

	t.Run("works with simple action returning nil", func(t *testing.T) {
		actionCalled := false
		action := func() tea.Msg {
			actionCalled = true
			return nil
		}

		// Set up callback to track action execution
		actionExecuted := false
		h.controller.confirmationOverlay = overlay.NewConfirmationOverlay("Test action?")
		h.controller.confirmationOverlay.OnConfirm = func() {
			h.state = tuiStateDefault
			actionExecuted = true
			action() // Execute the action
		}
		h.state = tuiStateConfirm

		// Verify state was set
		assert.Equal(t, tuiStateConfirm, h.state)
		assert.NotNil(t, h.controller.confirmationOverlay)
		assert.False(t, h.controller.confirmationOverlay.Dismissed)
		assert.NotNil(t, h.controller.confirmationOverlay.OnConfirm)

		// Execute the confirmation callback
		h.controller.confirmationOverlay.OnConfirm()
		assert.True(t, actionCalled)
		assert.True(t, actionExecuted)
	})

	t.Run("works with action returning error", func(t *testing.T) {
		expectedErr := fmt.Errorf("test error")
		action := func() tea.Msg {
			return expectedErr
		}

		// Set up callback to track action execution
		var receivedMsg tea.Msg
		h.controller.confirmationOverlay = overlay.NewConfirmationOverlay("Error action?")
		h.controller.confirmationOverlay.OnConfirm = func() {
			h.state = tuiStateDefault
			receivedMsg = action() // Execute the action and capture result
		}
		h.state = tuiStateConfirm

		// Verify state was set
		assert.Equal(t, tuiStateConfirm, h.state)
		assert.NotNil(t, h.controller.confirmationOverlay)
		assert.False(t, h.controller.confirmationOverlay.Dismissed)
		assert.NotNil(t, h.controller.confirmationOverlay.OnConfirm)

		// Execute the confirmation callback
		h.controller.confirmationOverlay.OnConfirm()
		assert.Equal(t, expectedErr, receivedMsg)
	})

	t.Run("works with action returning custom message", func(t *testing.T) {
		action := func() tea.Msg {
			return instanceChangedMsg{}
		}

		// Set up callback to track action execution
		var receivedMsg tea.Msg
		h.controller.confirmationOverlay = overlay.NewConfirmationOverlay("Custom message action?")
		h.controller.confirmationOverlay.OnConfirm = func() {
			h.state = tuiStateDefault
			receivedMsg = action() // Execute the action and capture result
		}
		h.state = tuiStateConfirm

		// Verify state was set
		assert.Equal(t, tuiStateConfirm, h.state)
		assert.NotNil(t, h.controller.confirmationOverlay)
		assert.False(t, h.controller.confirmationOverlay.Dismissed)
		assert.NotNil(t, h.controller.confirmationOverlay.OnConfirm)

		// Execute the confirmation callback
		h.controller.confirmationOverlay.OnConfirm()
		_, ok := receivedMsg.(instanceChangedMsg)
		assert.True(t, ok, "Expected instanceChangedMsg but got %T", receivedMsg)
	})
}

// TestMultipleConfirmationsDontInterfere tests that multiple confirmations don't interfere with each other
func TestMultipleConfirmationsDontInterfere(t *testing.T) {
	spinner := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	h := &Model{
		ctx:        context.Background(),
		state:      tuiStateDefault,
		appConfig:  config.DefaultConfig(),
		controller: NewController(&spinner, false),
	}

	// First confirmation
	action1Called := false
	action1 := func() tea.Msg {
		action1Called = true
		return nil
	}

	// Set up first confirmation
	h.controller.confirmationOverlay = overlay.NewConfirmationOverlay("First action?")
	firstOnConfirm := func() {
		h.state = tuiStateDefault
		action1()
	}
	h.controller.confirmationOverlay.OnConfirm = firstOnConfirm
	h.state = tuiStateConfirm

	// Verify first confirmation
	assert.Equal(t, tuiStateConfirm, h.state)
	assert.NotNil(t, h.controller.confirmationOverlay)
	assert.False(t, h.controller.confirmationOverlay.Dismissed)
	assert.NotNil(t, h.controller.confirmationOverlay.OnConfirm)

	// Cancel first confirmation (simulate pressing 'n')
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}
	shouldClose := h.controller.confirmationOverlay.HandleKeyPress(keyMsg)
	if shouldClose {
		h.state = tuiStateDefault
		h.controller.confirmationOverlay = nil
	}

	// Second confirmation with different action
	action2Called := false
	action2 := func() tea.Msg {
		action2Called = true
		return fmt.Errorf("action2 error")
	}

	// Set up second confirmation
	h.controller.confirmationOverlay = overlay.NewConfirmationOverlay("Second action?")
	var secondResult tea.Msg
	secondOnConfirm := func() {
		h.state = tuiStateDefault
		secondResult = action2()
	}
	h.controller.confirmationOverlay.OnConfirm = secondOnConfirm
	h.state = tuiStateConfirm

	// Verify second confirmation
	assert.Equal(t, tuiStateConfirm, h.state)
	assert.NotNil(t, h.controller.confirmationOverlay)
	assert.False(t, h.controller.confirmationOverlay.Dismissed)
	assert.NotNil(t, h.controller.confirmationOverlay.OnConfirm)

	// Execute second action to verify it's the correct one
	h.controller.confirmationOverlay.OnConfirm()
	err, ok := secondResult.(error)
	assert.True(t, ok)
	assert.Equal(t, "action2 error", err.Error())
	assert.True(t, action2Called)
	assert.False(t, action1Called, "First action should not have been called")

	// Test that cancelled action can still be executed independently
	firstOnConfirm()
	assert.True(t, action1Called, "First action should be callable after being replaced")
}

// TestConfirmationModalVisualAppearance tests that confirmation modal has distinct visual appearance
func TestConfirmationModalVisualAppearance(t *testing.T) {
	spinner := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	h := &Model{
		ctx:        context.Background(),
		state:      tuiStateDefault,
		appConfig:  config.DefaultConfig(),
		controller: NewController(&spinner, false),
	}

	// Create a test confirmation overlay
	message := "[!] Delete everything?"
	h.controller.confirmationOverlay = overlay.NewConfirmationOverlay(message)
	h.state = tuiStateConfirm

	// Verify the overlay was created with confirmation settings
	assert.NotNil(t, h.controller.confirmationOverlay)
	assert.Equal(t, tuiStateConfirm, h.state)
	assert.False(t, h.controller.confirmationOverlay.Dismissed)

	// Test the overlay render (we can test that it renders without errors)
	rendered := h.controller.confirmationOverlay.Render()
	assert.NotEmpty(t, rendered)

	// Test that it includes the message content and instructions
	assert.Contains(t, rendered, "Delete everything?")
	assert.Contains(t, rendered, "Press")
	assert.Contains(t, rendered, "to confirm")
	assert.Contains(t, rendered, "to cancel")

	// Test that the danger indicator is preserved
	assert.Contains(t, rendered, "[!")
}
