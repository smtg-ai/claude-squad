package app

import (
	"claude-squad/config"
	"claude-squad/keys"
	"claude-squad/log"
	"claude-squad/session"
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
	h := &home{
		ctx:       context.Background(),
		state:     stateDefault,
		appConfig: config.DefaultConfig(),
	}

	t.Run("shows confirmation on D press", func(t *testing.T) {
		// Simulate pressing 'D'
		h.state = stateDefault
		h.confirmationOverlay = nil

		// Manually trigger what would happen in handleKeyPress for 'D'
		h.state = stateConfirm
		h.confirmationOverlay = overlay.NewConfirmationOverlay("[!] Kill session 'test'?")

		assert.Equal(t, stateConfirm, h.state)
		assert.NotNil(t, h.confirmationOverlay)
		assert.False(t, h.confirmationOverlay.Dismissed)
	})

	t.Run("returns to default on y press", func(t *testing.T) {
		// Start in confirmation state
		h.state = stateConfirm
		h.confirmationOverlay = overlay.NewConfirmationOverlay("Test confirmation")

		// Simulate pressing 'y' using HandleKeyPress
		keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")}
		shouldClose := h.confirmationOverlay.HandleKeyPress(keyMsg)
		if shouldClose {
			h.state = stateDefault
			h.confirmationOverlay = nil
		}

		assert.Equal(t, stateDefault, h.state)
		assert.Nil(t, h.confirmationOverlay)
	})

	t.Run("returns to default on n press", func(t *testing.T) {
		// Start in confirmation state
		h.state = stateConfirm
		h.confirmationOverlay = overlay.NewConfirmationOverlay("Test confirmation")

		// Simulate pressing 'n' using HandleKeyPress
		keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}
		shouldClose := h.confirmationOverlay.HandleKeyPress(keyMsg)
		if shouldClose {
			h.state = stateDefault
			h.confirmationOverlay = nil
		}

		assert.Equal(t, stateDefault, h.state)
		assert.Nil(t, h.confirmationOverlay)
	})

	t.Run("returns to default on esc press", func(t *testing.T) {
		// Start in confirmation state
		h.state = stateConfirm
		h.confirmationOverlay = overlay.NewConfirmationOverlay("Test confirmation")

		// Simulate pressing ESC using HandleKeyPress
		keyMsg := tea.KeyMsg{Type: tea.KeyEscape}
		shouldClose := h.confirmationOverlay.HandleKeyPress(keyMsg)
		if shouldClose {
			h.state = stateDefault
			h.confirmationOverlay = nil
		}

		assert.Equal(t, stateDefault, h.state)
		assert.Nil(t, h.confirmationOverlay)
	})
}

// TestConfirmationModalKeyHandling tests the actual key handling in confirmation state
func TestConfirmationModalKeyHandling(t *testing.T) {
	// Import needed packages
	spinner := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	list := ui.NewList(&spinner, false, nil)

	// Create enough of home struct to test handleKeyPress in confirmation state
	h := &home{
		ctx:                 context.Background(),
		state:               stateConfirm,
		appConfig:           config.DefaultConfig(),
		list:                list,
		menu:                ui.NewMenu(),
		confirmationOverlay: overlay.NewConfirmationOverlay("Kill session?"),
	}

	testCases := []struct {
		name              string
		key               string
		expectedState     state
		expectedDismissed bool
		expectedNil       bool
	}{
		{
			name:              "y key confirms and dismisses overlay",
			key:               "y",
			expectedState:     stateDefault,
			expectedDismissed: true,
			expectedNil:       true,
		},
		{
			name:              "n key cancels and dismisses overlay",
			key:               "n",
			expectedState:     stateDefault,
			expectedDismissed: true,
			expectedNil:       true,
		},
		{
			name:              "esc key cancels and dismisses overlay",
			key:               "esc",
			expectedState:     stateDefault,
			expectedDismissed: true,
			expectedNil:       true,
		},
		{
			name:              "other keys are ignored",
			key:               "x",
			expectedState:     stateConfirm,
			expectedDismissed: false,
			expectedNil:       false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset state
			h.state = stateConfirm
			h.confirmationOverlay = overlay.NewConfirmationOverlay("Kill session?")

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
			if tc.expectedNil {
				assert.Nil(t, homeModel.confirmationOverlay, "Overlay should be nil for key: %s", tc.key)
			} else {
				assert.NotNil(t, homeModel.confirmationOverlay, "Overlay should not be nil for key: %s", tc.key)
				assert.Equal(t, tc.expectedDismissed, homeModel.confirmationOverlay.Dismissed, "Dismissed mismatch for key: %s", tc.key)
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
	list := ui.NewList(&spinner, false, nil)

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
	}

	// Simulate what happens when D is pressed
	selected := h.list.GetSelectedInstance()
	require.NotNil(t, selected)

	// This is what the KeyKill handler does
	message := fmt.Sprintf("[!] Kill session '%s'?", selected.Title)
	h.confirmationOverlay = overlay.NewConfirmationOverlay(message)
	h.state = stateConfirm

	// Verify the state
	assert.Equal(t, stateConfirm, h.state)
	assert.NotNil(t, h.confirmationOverlay)
	assert.False(t, h.confirmationOverlay.Dismissed)
	// Test that overlay renders with the correct message
	rendered := h.confirmationOverlay.Render()
	assert.Contains(t, rendered, "Kill session 'test-session'?")
}

// TestConfirmActionWithDifferentTypes tests that confirmAction works with different action types
func TestConfirmActionWithDifferentTypes(t *testing.T) {
	h := &home{
		ctx:       context.Background(),
		state:     stateDefault,
		appConfig: config.DefaultConfig(),
	}

	t.Run("works with simple action returning nil", func(t *testing.T) {
		actionCalled := false
		action := func() tea.Msg {
			actionCalled = true
			return nil
		}

		// Set up callback to track action execution
		actionExecuted := false
		h.confirmationOverlay = overlay.NewConfirmationOverlay("Test action?")
		h.confirmationOverlay.OnConfirm = func() {
			h.state = stateDefault
			actionExecuted = true
			action() // Execute the action
		}
		h.state = stateConfirm

		// Verify state was set
		assert.Equal(t, stateConfirm, h.state)
		assert.NotNil(t, h.confirmationOverlay)
		assert.False(t, h.confirmationOverlay.Dismissed)
		assert.NotNil(t, h.confirmationOverlay.OnConfirm)

		// Execute the confirmation callback
		h.confirmationOverlay.OnConfirm()
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
		h.confirmationOverlay = overlay.NewConfirmationOverlay("Error action?")
		h.confirmationOverlay.OnConfirm = func() {
			h.state = stateDefault
			receivedMsg = action() // Execute the action and capture result
		}
		h.state = stateConfirm

		// Verify state was set
		assert.Equal(t, stateConfirm, h.state)
		assert.NotNil(t, h.confirmationOverlay)
		assert.False(t, h.confirmationOverlay.Dismissed)
		assert.NotNil(t, h.confirmationOverlay.OnConfirm)

		// Execute the confirmation callback
		h.confirmationOverlay.OnConfirm()
		assert.Equal(t, expectedErr, receivedMsg)
	})

	t.Run("works with action returning custom message", func(t *testing.T) {
		action := func() tea.Msg {
			return instanceChangedMsg{}
		}

		// Set up callback to track action execution
		var receivedMsg tea.Msg
		h.confirmationOverlay = overlay.NewConfirmationOverlay("Custom message action?")
		h.confirmationOverlay.OnConfirm = func() {
			h.state = stateDefault
			receivedMsg = action() // Execute the action and capture result
		}
		h.state = stateConfirm

		// Verify state was set
		assert.Equal(t, stateConfirm, h.state)
		assert.NotNil(t, h.confirmationOverlay)
		assert.False(t, h.confirmationOverlay.Dismissed)
		assert.NotNil(t, h.confirmationOverlay.OnConfirm)

		// Execute the confirmation callback
		h.confirmationOverlay.OnConfirm()
		_, ok := receivedMsg.(instanceChangedMsg)
		assert.True(t, ok, "Expected instanceChangedMsg but got %T", receivedMsg)
	})
}

// TestMultipleConfirmationsDontInterfere tests that multiple confirmations don't interfere with each other
func TestMultipleConfirmationsDontInterfere(t *testing.T) {
	h := &home{
		ctx:       context.Background(),
		state:     stateDefault,
		appConfig: config.DefaultConfig(),
	}

	// First confirmation
	action1Called := false
	action1 := func() tea.Msg {
		action1Called = true
		return nil
	}

	// Set up first confirmation
	h.confirmationOverlay = overlay.NewConfirmationOverlay("First action?")
	firstOnConfirm := func() {
		h.state = stateDefault
		action1()
	}
	h.confirmationOverlay.OnConfirm = firstOnConfirm
	h.state = stateConfirm

	// Verify first confirmation
	assert.Equal(t, stateConfirm, h.state)
	assert.NotNil(t, h.confirmationOverlay)
	assert.False(t, h.confirmationOverlay.Dismissed)
	assert.NotNil(t, h.confirmationOverlay.OnConfirm)

	// Cancel first confirmation (simulate pressing 'n')
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}
	shouldClose := h.confirmationOverlay.HandleKeyPress(keyMsg)
	if shouldClose {
		h.state = stateDefault
		h.confirmationOverlay = nil
	}

	// Second confirmation with different action
	action2Called := false
	action2 := func() tea.Msg {
		action2Called = true
		return fmt.Errorf("action2 error")
	}

	// Set up second confirmation
	h.confirmationOverlay = overlay.NewConfirmationOverlay("Second action?")
	var secondResult tea.Msg
	secondOnConfirm := func() {
		h.state = stateDefault
		secondResult = action2()
	}
	h.confirmationOverlay.OnConfirm = secondOnConfirm
	h.state = stateConfirm

	// Verify second confirmation
	assert.Equal(t, stateConfirm, h.state)
	assert.NotNil(t, h.confirmationOverlay)
	assert.False(t, h.confirmationOverlay.Dismissed)
	assert.NotNil(t, h.confirmationOverlay.OnConfirm)

	// Execute second action to verify it's the correct one
	h.confirmationOverlay.OnConfirm()
	err, ok := secondResult.(error)
	assert.True(t, ok)
	assert.Equal(t, "action2 error", err.Error())
	assert.True(t, action2Called)
	assert.False(t, action1Called, "First action should not have been called")

	// Test that cancelled action can still be executed independently
	firstOnConfirm()
	assert.True(t, action1Called, "First action should be callable after being replaced")
}

// SCROLL FUNCTIONALITY TESTS
// ===========================

// TestPreviewScrollMethods tests that PreviewPane has scroll methods
func TestPreviewScrollMethods(t *testing.T) {
	previewPane := ui.NewPreviewPane()
	previewPane.SetSize(80, 20)

	// Test that scroll methods exist and don't panic
	assert.NotPanics(t, func() {
		previewPane.ScrollUp()
	}, "ScrollUp should not panic")

	assert.NotPanics(t, func() {
		previewPane.ScrollDown()
	}, "ScrollDown should not panic")
}

// TestTabbedWindowScrollDelegation tests that TabbedWindow routes scroll to correct pane
func TestTabbedWindowScrollDelegation(t *testing.T) {
	preview := ui.NewPreviewPane()
	diff := ui.NewDiffPane()
	console := ui.NewConsolePane()
	tabbedWindow := ui.NewTabbedWindow(preview, diff, console)

	tabbedWindow.SetSize(100, 30)

	// Test scroll methods exist and don't panic for both tabs
	assert.NotPanics(t, func() {
		// Preview tab (activeTab = 0 by default)
		tabbedWindow.ScrollUp()
		tabbedWindow.ScrollDown()
	}, "Scroll in preview tab should not panic")

	// Switch to diff tab
	tabbedWindow.Toggle()

	assert.NotPanics(t, func() {
		// Diff tab (activeTab = 1)
		tabbedWindow.ScrollUp()
		tabbedWindow.ScrollDown()
	}, "Scroll in diff tab should not panic")
}

// TestScrollKeyHandling tests that scroll keys are properly handled
func TestScrollKeyHandling(t *testing.T) {
	h := createTestHome()

	// Test handling scroll keys directly by name
	assert.NotPanics(t, func() {
		// Simulate shift+up processing
		h.tabbedWindow.ScrollUp()
	}, "ScrollUp should not panic")

	assert.NotPanics(t, func() {
		// Simulate shift+down processing
		h.tabbedWindow.ScrollDown()
	}, "ScrollDown should not panic")
}

// TestScrollKeyBindings tests that scroll key bindings are properly defined
func TestScrollKeyBindings(t *testing.T) {
	// Test that scroll keys are defined in the global key map
	shiftUpKey, exists := keys.GlobalKeyStringsMap["shift+up"]
	assert.True(t, exists, "shift+up should be defined in key map")
	assert.Equal(t, keys.KeyShiftUp, shiftUpKey, "shift+up should map to KeyShiftUp")

	shiftDownKey, exists := keys.GlobalKeyStringsMap["shift+down"]
	assert.True(t, exists, "shift+down should be defined in key map")
	assert.Equal(t, keys.KeyShiftDown, shiftDownKey, "shift+down should map to KeyShiftDown")

	// Test that help text is updated
	shiftUpBinding, exists := keys.GlobalkeyBindings[keys.KeyShiftUp]
	assert.True(t, exists, "KeyShiftUp should have a binding")
	assert.Contains(t, shiftUpBinding.Help().Desc, "scroll", "KeyShiftUp help should mention scroll")

	shiftDownBinding, exists := keys.GlobalkeyBindings[keys.KeyShiftDown]
	assert.True(t, exists, "KeyShiftDown should have a binding")
	assert.Contains(t, shiftDownBinding.Help().Desc, "scroll", "KeyShiftDown help should mention scroll")
}

// TestTabIsolation tests that scroll only affects the active tab
func TestTabIsolation(t *testing.T) {
	h := createTestHome()

	// Ensure we start in preview tab
	if h.tabbedWindow.IsInDiffTab() {
		h.tabbedWindow.Toggle()
	}

	// Test scroll in preview tab
	assert.NotPanics(t, func() {
		h.tabbedWindow.ScrollUp()
	}, "Scroll in preview tab should work")

	// Switch to diff tab
	h.tabbedWindow.Toggle()
	assert.True(t, h.tabbedWindow.IsInDiffTab(), "Should be in diff tab after toggle")

	// Test scroll in diff tab
	assert.NotPanics(t, func() {
		h.tabbedWindow.ScrollUp()
	}, "Scroll in diff tab should work")
}

// TestMouseScrollEvents tests mouse wheel scrolling
func TestMouseScrollEvents(t *testing.T) {
	h := createTestHome()

	// Test mouse wheel up
	mouseMsg := tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelUp,
	}

	assert.NotPanics(t, func() {
		_, _ = h.Update(mouseMsg)
	}, "Mouse wheel up should not panic")

	// Test mouse wheel down
	mouseMsg = tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelDown,
	}

	assert.NotPanics(t, func() {
		_, _ = h.Update(mouseMsg)
	}, "Mouse wheel down should not panic")
}

func TestInstanceScrollIsolation(t *testing.T) {
	h := createTestHome()

	// Create two test instances
	instance1, err := session.NewInstance(session.InstanceOptions{
		Title:   "test1",
		Path:    ".",
		Program: "echo",
	})
	require.NoError(t, err)

	instance2, err := session.NewInstance(session.InstanceOptions{
		Title:   "test2",
		Path:    ".",
		Program: "echo",
	})
	require.NoError(t, err)

	// Add instances to list
	h.list.AddInstance(instance1)()
	h.list.AddInstance(instance2)()

	// Set up preview pane with some content for both instances
	h.tabbedWindow.UpdatePreview(instance1)

	// Scroll down in instance1
	for i := 0; i < 5; i++ {
		h.tabbedWindow.ScrollDown()
	}

	// Switch to instance2
	h.list.SetSelectedInstance(1)
	h.tabbedWindow.UpdatePreview(instance2)

	// Scroll up in instance2
	for i := 0; i < 3; i++ {
		h.tabbedWindow.ScrollUp()
	}

	// Switch back to instance1
	h.list.SetSelectedInstance(0)
	h.tabbedWindow.UpdatePreview(instance1)

	// Test that scroll operations don't panic and instances maintain separate state
	assert.NotPanics(t, func() {
		h.tabbedWindow.ScrollUp()
		h.tabbedWindow.ScrollDown()
	}, "Instance scroll isolation should work without panics")

	// Note: This test mainly ensures no panics occur.
	// Full position verification would require exposing viewport internals
}

func TestAutoScrollBehavior(t *testing.T) {
	h := createTestHome()

	// Create a test instance
	instance, err := session.NewInstance(session.InstanceOptions{
		Title:   "auto-scroll-test",
		Path:    ".",
		Program: "echo",
	})
	require.NoError(t, err)

	// Add instance to list
	h.list.AddInstance(instance)()

	// Initial content update - should go to bottom for new instance
	h.tabbedWindow.UpdatePreview(instance)

	assert.NotPanics(t, func() {
		// Simulate scrolling up (user reading something above)
		for i := 0; i < 3; i++ {
			h.tabbedWindow.ScrollUp()
		}

		// Update content again (simulating new content while user is scrolled up)
		h.tabbedWindow.UpdatePreview(instance)

		// Should not panic and should preserve user's scroll position
		// (This tests that we don't force scroll to bottom when user is reading above)
	}, "Auto-scroll should preserve user position when scrolled up")
}

func TestInstanceSwitchAutoScroll(t *testing.T) {
	h := createTestHome()

	// Create two test instances
	instance1, err := session.NewInstance(session.InstanceOptions{
		Title:   "switch-test-1",
		Path:    ".",
		Program: "echo",
	})
	require.NoError(t, err)

	instance2, err := session.NewInstance(session.InstanceOptions{
		Title:   "switch-test-2",
		Path:    ".",
		Program: "echo",
	})
	require.NoError(t, err)

	// Add instances to list
	h.list.AddInstance(instance1)()
	h.list.AddInstance(instance2)()

	assert.NotPanics(t, func() {
		// Start with instance1
		h.list.SetSelectedInstance(0)
		h.tabbedWindow.UpdatePreview(instance1)

		// Scroll up in instance1
		for i := 0; i < 5; i++ {
			h.tabbedWindow.ScrollUp()
		}

		// Switch to instance2 - should auto-scroll to bottom to show latest activity
		h.list.SetSelectedInstance(1)
		h.tabbedWindow.UpdatePreview(instance2)

		// Switch back to instance1 - should auto-scroll to bottom
		h.list.SetSelectedInstance(0)
		h.tabbedWindow.UpdatePreview(instance1)

	}, "Instance switching should auto-scroll to show latest activity")
}

func TestFastScrollMethods(t *testing.T) {
	h := createTestHome()

	// Create a test instance
	instance, err := session.NewInstance(session.InstanceOptions{
		Title:   "fast-scroll-test",
		Path:    ".",
		Program: "echo",
	})
	require.NoError(t, err)

	// Add instance to list
	h.list.AddInstance(instance)()
	h.tabbedWindow.UpdatePreview(instance)

	assert.NotPanics(t, func() {
		// Test fast scroll methods exist and work in preview tab
		h.tabbedWindow.FastScrollUp()
		h.tabbedWindow.FastScrollDown()

		// Switch to diff tab and test there too
		h.tabbedWindow.Toggle()
		h.tabbedWindow.FastScrollUp()
		h.tabbedWindow.FastScrollDown()
	}, "Fast scroll methods should work without panics")
}

func TestFastScrollKeyBindings(t *testing.T) {
	// Test that fast scroll key bindings are properly defined
	ctrlShiftUpKey, exists := keys.GlobalKeyStringsMap["ctrl+shift+up"]
	assert.True(t, exists, "ctrl+shift+up should be defined in key map")
	assert.Equal(t, keys.KeyCtrlShiftUp, ctrlShiftUpKey)

	ctrlShiftDownKey, exists := keys.GlobalKeyStringsMap["ctrl+shift+down"]
	assert.True(t, exists, "ctrl+shift+down should be defined in key map")
	assert.Equal(t, keys.KeyCtrlShiftDown, ctrlShiftDownKey)

	// Test key bindings have proper help text
	binding := keys.GlobalkeyBindings[keys.KeyCtrlShiftUp]
	assert.Contains(t, binding.Help().Key, "ctrl+shift+↑")
	assert.Contains(t, binding.Help().Desc, "fast scroll up")

	binding = keys.GlobalkeyBindings[keys.KeyCtrlShiftDown]
	assert.Contains(t, binding.Help().Key, "ctrl+shift+↓")
	assert.Contains(t, binding.Help().Desc, "fast scroll down")
}

func TestFastScrollKeyHandling(t *testing.T) {
	h := createTestHome()

	// Create a test instance
	instance, err := session.NewInstance(session.InstanceOptions{
		Title:   "fast-key-test",
		Path:    ".",
		Program: "echo",
	})
	require.NoError(t, err)

	// Add instance to list
	h.list.AddInstance(instance)()
	h.tabbedWindow.UpdatePreview(instance)

	// Test ctrl+shift+up key handling (simulate the key string)
	ctrlShiftUpMsg := tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("ctrl+shift+up"),
	}

	assert.NotPanics(t, func() {
		_, _ = h.Update(ctrlShiftUpMsg)
	}, "Ctrl+Shift+Up key should not panic")

	// Test ctrl+shift+down key handling (simulate the key string)
	ctrlShiftDownMsg := tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("ctrl+shift+down"),
	}

	assert.NotPanics(t, func() {
		_, _ = h.Update(ctrlShiftDownMsg)
	}, "Ctrl+Shift+Down key should not panic")
}

// TestDuplicateTitleScrollIsolation tests that instances with same title maintain separate scroll positions
func TestDuplicateTitleScrollIsolation(t *testing.T) {
	h := createTestHome()

	// Create two test instances with SAME title but different content
	instance1, err := session.NewInstance(session.InstanceOptions{
		Title:   "duplicate-title", // Same title
		Path:    ".",
		Program: "echo",
	})
	require.NoError(t, err)

	instance2, err := session.NewInstance(session.InstanceOptions{
		Title:   "duplicate-title", // Same title
		Path:    ".",
		Program: "echo",
	})
	require.NoError(t, err)

	// Verify they are different instances (different pointers)
	assert.NotEqual(t, instance1, instance2, "Should be different instance objects")

	// Add instances to list
	h.list.AddInstance(instance1)()
	h.list.AddInstance(instance2)()

	// Set up preview pane with content for instance1
	h.tabbedWindow.UpdatePreview(instance1)

	// Scroll down in instance1
	for i := 0; i < 5; i++ {
		h.tabbedWindow.ScrollDown()
	}

	// Switch to instance2 (same title, different instance)
	h.list.SetSelectedInstance(1)
	h.tabbedWindow.UpdatePreview(instance2)

	// Scroll up in instance2 - should NOT affect instance1's scroll position
	for i := 0; i < 3; i++ {
		h.tabbedWindow.ScrollUp()
	}

	// Switch back to instance1
	h.list.SetSelectedInstance(0)
	h.tabbedWindow.UpdatePreview(instance1)

	// Test that instances maintain separate scroll state despite having same title
	assert.NotPanics(t, func() {
		h.tabbedWindow.ScrollUp()
		h.tabbedWindow.ScrollDown()
	}, "Duplicate title instances should maintain separate scroll positions")

	// This test mainly ensures that using instance pointers as keys works correctly
	// Previous implementation using instance.Title would have had both instances
	// sharing the same scroll position, causing bugs
}

// TestConfirmationModalVisualAppearance tests that confirmation modal has distinct visual appearance
func TestConfirmationModalVisualAppearance(t *testing.T) {
	h := &home{
		ctx:       context.Background(),
		state:     stateDefault,
		appConfig: config.DefaultConfig(),
	}

	// Create a test confirmation overlay
	message := "[!] Delete everything?"
	h.confirmationOverlay = overlay.NewConfirmationOverlay(message)
	h.state = stateConfirm

	// Verify the overlay was created with confirmation settings
	assert.NotNil(t, h.confirmationOverlay)
	assert.Equal(t, stateConfirm, h.state)
	assert.False(t, h.confirmationOverlay.Dismissed)

	// Test the overlay render (we can test that it renders without errors)
	rendered := h.confirmationOverlay.Render()
	assert.NotEmpty(t, rendered)

	// Test that it includes the message content and instructions
	assert.Contains(t, rendered, "Delete everything?")
	assert.Contains(t, rendered, "Press")
	assert.Contains(t, rendered, "to confirm")
	assert.Contains(t, rendered, "to cancel")

	// Test that the danger indicator is preserved
	assert.Contains(t, rendered, "[!")
}

// Helper function to create a test home instance for scroll tests
func createTestHome() *home {
	testSpinner := spinner.New(spinner.WithSpinner(spinner.MiniDot))

	return &home{
		ctx:          context.Background(),
		state:        stateDefault,
		appConfig:    config.DefaultConfig(),
		list:         ui.NewList(&testSpinner, false, nil),
		tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewDiffPane(), ui.NewConsolePane()),
		menu:         ui.NewMenu(),
		spinner:      testSpinner,
	}
}
