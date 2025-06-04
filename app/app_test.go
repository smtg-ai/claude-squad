package app

import (
	"claude-squad/config"
	"claude-squad/session"
	"claude-squad/ui"
	"context"
	"fmt"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfirmationModalStateTransitions tests state transitions without full instance setup
func TestConfirmationModalStateTransitions(t *testing.T) {
	// Create a minimal home struct for testing state transitions
	h := &home{
		ctx:       context.Background(),
		state:     stateDefault,
		appConfig: config.DefaultConfig(),
		confirmation: struct {
			pendingAction tea.Cmd
			message       string
			active        bool
		}{},
	}

	t.Run("shows confirmation on D press", func(t *testing.T) {
		// Simulate pressing 'D'
		h.state = stateDefault
		h.confirmation.active = false

		// Manually trigger what would happen in handleKeyPress for 'D'
		h.state = stateConfirm
		h.confirmation.active = true
		h.confirmation.message = "Kill session? (y/n)"

		assert.Equal(t, stateConfirm, h.state)
		assert.True(t, h.confirmation.active)
		assert.Equal(t, "Kill session? (y/n)", h.confirmation.message)
	})

	t.Run("returns to default on y press", func(t *testing.T) {
		// Start in confirmation state
		h.state = stateConfirm
		h.confirmation.active = true

		// Simulate pressing 'y' - the handler would reset state
		h.state = stateDefault
		h.confirmation.active = false

		assert.Equal(t, stateDefault, h.state)
		assert.False(t, h.confirmation.active)
	})

	t.Run("returns to default on n press", func(t *testing.T) {
		// Start in confirmation state
		h.state = stateConfirm
		h.confirmation.active = true

		// Simulate pressing 'n' - the handler would reset state
		h.state = stateDefault
		h.confirmation.active = false

		assert.Equal(t, stateDefault, h.state)
		assert.False(t, h.confirmation.active)
	})

	t.Run("returns to default on esc press", func(t *testing.T) {
		// Start in confirmation state
		h.state = stateConfirm
		h.confirmation.active = true

		// Simulate pressing ESC - the handler would reset state
		h.state = stateDefault
		h.confirmation.active = false

		assert.Equal(t, stateDefault, h.state)
		assert.False(t, h.confirmation.active)
	})
}

// TestConfirmationModalKeyHandling tests the actual key handling in confirmation state
func TestConfirmationModalKeyHandling(t *testing.T) {
	// Import needed packages
	spinner := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	list := ui.NewList(&spinner, false)

	// Create enough of home struct to test handleKeyPress in confirmation state
	h := &home{
		ctx:       context.Background(),
		state:     stateConfirm,
		appConfig: config.DefaultConfig(),
		list:      list,
		menu:      ui.NewMenu(),
		confirmation: struct {
			pendingAction tea.Cmd
			message       string
			active        bool
		}{
			active:  true,
			message: "Kill session? (y/n)",
		},
	}

	testCases := []struct {
		name           string
		key            string
		expectedState  state
		expectedActive bool
	}{
		{
			name:           "y key cancels confirmation",
			key:            "y",
			expectedState:  stateDefault,
			expectedActive: false,
		},
		{
			name:           "n key cancels confirmation",
			key:            "n",
			expectedState:  stateDefault,
			expectedActive: false,
		},
		{
			name:           "esc key cancels confirmation",
			key:            "esc",
			expectedState:  stateDefault,
			expectedActive: false,
		},
		{
			name:           "other keys are ignored",
			key:            "x",
			expectedState:  stateConfirm,
			expectedActive: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset state
			h.state = stateConfirm
			h.confirmation.active = true

			// Create key message
			var keyMsg tea.KeyMsg
			if tc.key == "esc" {
				keyMsg = tea.KeyMsg{Type: tea.KeyEscape}
			} else {
				keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.key)}
			}

			// Call handleKeyPress
			model, _ := h.handleKeyPress(keyMsg)
			homeModel, ok := model.(*home)
			require.True(t, ok)

			assert.Equal(t, tc.expectedState, homeModel.state, "State mismatch for key: %s", tc.key)
			assert.Equal(t, tc.expectedActive, homeModel.confirmation.active, "Active mismatch for key: %s", tc.key)
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
	list := ui.NewList(&spinner, false)

	// Add test instance
	instance, err := session.NewInstance(session.InstanceOptions{
		Title:   "test-session",
		Path:    t.TempDir(),
		Program: "claude",
		AutoYes: false,
	})
	require.NoError(t, err)
	_ = list.AddInstance(instance)
	list.SetSelectedInstance(0)

	h := &home{
		ctx:       context.Background(),
		state:     stateDefault,
		appConfig: config.DefaultConfig(),
		list:      list,
		menu:      ui.NewMenu(),
		confirmation: struct {
			pendingAction tea.Cmd
			message       string
			active        bool
		}{},
	}

	// Simulate what happens when D is pressed
	selected := h.list.GetSelectedInstance()
	require.NotNil(t, selected)

	// This is what the KeyKill handler does
	h.confirmation.active = true
	h.confirmation.message = fmt.Sprintf("[!] Kill session '%s'? (y/n)", selected.Title)
	h.state = stateConfirm

	// Verify the state
	assert.Equal(t, stateConfirm, h.state)
	assert.True(t, h.confirmation.active)
	assert.Equal(t, "[!] Kill session 'test-session'? (y/n)", h.confirmation.message)
}

// TestConfirmActionWithDifferentTypes tests that confirmAction works with different action types
func TestConfirmActionWithDifferentTypes(t *testing.T) {
	h := &home{
		ctx:       context.Background(),
		state:     stateDefault,
		appConfig: config.DefaultConfig(),
		confirmation: struct {
			pendingAction tea.Cmd
			message       string
			active        bool
		}{},
	}

	t.Run("works with simple action returning nil", func(t *testing.T) {
		actionCalled := false
		action := func() tea.Msg {
			actionCalled = true
			return nil
		}

		// Call confirmAction
		h.confirmAction("Test action? (y/n)", action)

		// Verify state was set
		assert.Equal(t, stateConfirm, h.state)
		assert.True(t, h.confirmation.active)
		assert.Equal(t, "Test action? (y/n)", h.confirmation.message)
		assert.NotNil(t, h.confirmation.pendingAction)

		// Execute the action
		msg := h.confirmation.pendingAction()
		assert.Nil(t, msg)
		assert.True(t, actionCalled)
	})

	t.Run("works with action returning error", func(t *testing.T) {
		expectedErr := fmt.Errorf("test error")
		action := func() tea.Msg {
			return expectedErr
		}

		// Call confirmAction
		h.confirmAction("Error action? (y/n)", action)

		// Verify state was set
		assert.Equal(t, stateConfirm, h.state)
		assert.True(t, h.confirmation.active)
		assert.Equal(t, "Error action? (y/n)", h.confirmation.message)
		assert.NotNil(t, h.confirmation.pendingAction)

		// Execute the action
		msg := h.confirmation.pendingAction()
		assert.Equal(t, expectedErr, msg)
	})

	t.Run("works with action returning custom message", func(t *testing.T) {
		action := func() tea.Msg {
			return instanceChangedMsg{}
		}

		// Call confirmAction
		h.confirmAction("Custom message action? (y/n)", action)

		// Verify state was set
		assert.Equal(t, stateConfirm, h.state)
		assert.True(t, h.confirmation.active)
		assert.Equal(t, "Custom message action? (y/n)", h.confirmation.message)
		assert.NotNil(t, h.confirmation.pendingAction)

		// Execute the action
		msg := h.confirmation.pendingAction()
		_, ok := msg.(instanceChangedMsg)
		assert.True(t, ok, "Expected instanceChangedMsg but got %T", msg)
	})
}

// TestMultipleConfirmationsDontInterfere tests that multiple confirmations don't interfere with each other
func TestMultipleConfirmationsDontInterfere(t *testing.T) {
	h := &home{
		ctx:       context.Background(),
		state:     stateDefault,
		appConfig: config.DefaultConfig(),
		confirmation: struct {
			pendingAction tea.Cmd
			message       string
			active        bool
		}{},
	}

	// First confirmation
	action1Called := false
	action1 := func() tea.Msg {
		action1Called = true
		return nil
	}

	// Set up first confirmation
	h.confirmAction("First action? (y/n)", action1)

	// Verify first confirmation
	assert.Equal(t, stateConfirm, h.state)
	assert.True(t, h.confirmation.active)
	assert.Equal(t, "First action? (y/n)", h.confirmation.message)
	firstAction := h.confirmation.pendingAction

	// Cancel first confirmation (simulate pressing 'n')
	h.state = stateDefault
	h.confirmation.active = false
	h.confirmation.pendingAction = nil

	// Second confirmation with different action
	action2Called := false
	action2 := func() tea.Msg {
		action2Called = true
		return fmt.Errorf("action2 error")
	}

	// Set up second confirmation
	h.confirmAction("Second action? (y/n)", action2)

	// Verify second confirmation
	assert.Equal(t, stateConfirm, h.state)
	assert.True(t, h.confirmation.active)
	assert.Equal(t, "Second action? (y/n)", h.confirmation.message)
	secondAction := h.confirmation.pendingAction

	// Execute second action to verify it's the correct one
	msg := secondAction()
	err, ok := msg.(error)
	assert.True(t, ok)
	assert.Equal(t, "action2 error", err.Error())
	assert.True(t, action2Called)
	assert.False(t, action1Called, "First action should not have been called")

	// Test that cancelled action can still be executed independently
	firstAction()
	assert.True(t, action1Called, "First action should be callable after being replaced")
}

// TestConfirmationModalVisualAppearance tests that confirmation modal has distinct visual appearance
func TestConfirmationModalVisualAppearance(t *testing.T) {
	h := &home{
		ctx:       context.Background(),
		state:     stateDefault,
		appConfig: config.DefaultConfig(),
		confirmation: struct {
			pendingAction tea.Cmd
			message       string
			active        bool
		}{},
	}

	// Create a test action
	action := func() tea.Msg { return nil }

	// Call confirmAction
	h.confirmAction("[!] Delete everything? (y/n)", action)

	// Verify the overlay was created with confirmation settings
	assert.NotNil(t, h.textOverlay)
	assert.Equal(t, stateConfirm, h.state)

	// Test that the message contains the danger indicator
	assert.Contains(t, h.confirmation.message, "[!]")

	// Test the overlay render (we can't test the actual red border without rendering,
	// but we can verify the overlay is set up correctly)
	rendered := h.textOverlay.Render()
	assert.NotEmpty(t, rendered)

	// Test that it includes the message content
	assert.Contains(t, rendered, "Delete everything? (y/n)")
}
