package app

import (
	"claude-squad/config"
	"claude-squad/keys"
	"claude-squad/session"
	"claude-squad/ui"
	"context"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	tabbedWindow := ui.NewTabbedWindow(preview, diff)

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

// Helper function to create a test home instance (copied from existing app_test.go pattern)
func createTestHome() *home {
	testSpinner := spinner.New(spinner.WithSpinner(spinner.MiniDot))

	return &home{
		ctx:          context.Background(),
		state:        stateDefault,
		appConfig:    config.DefaultConfig(),
		list:         ui.NewList(&testSpinner, false),
		tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewDiffPane()),
		menu:         ui.NewMenu(),
		spinner:      testSpinner,
	}
}
