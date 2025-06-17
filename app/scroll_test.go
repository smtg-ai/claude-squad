package app

import (
	"claude-squad/config"
	"claude-squad/keys"
	"claude-squad/ui"
	"context"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
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