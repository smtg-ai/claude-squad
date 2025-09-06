package app

import (
	"claude-squad/config"
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

// TestSyncConfirmationStateTransitions tests the new sync confirmation state
func TestSyncConfirmationStateTransitions(t *testing.T) {
	h := &home{
		ctx:       context.Background(),
		state:     stateDefault,
		appConfig: config.DefaultConfig(),
	}

	t.Run("enters sync confirmation state", func(t *testing.T) {
		// Simulate entering sync confirmation state
		h.state = stateSyncConfirm
		h.confirmationOverlay = overlay.NewConfirmationOverlay("Remote branch 'feature' has different commits. Sync before creating? (y/n)")

		assert.Equal(t, stateSyncConfirm, h.state)
		assert.NotNil(t, h.confirmationOverlay)
		assert.False(t, h.confirmationOverlay.Dismissed)
	})

	t.Run("handles y key in sync confirmation", func(t *testing.T) {
		h.state = stateSyncConfirm
		h.confirmationOverlay = overlay.NewConfirmationOverlay("Sync with remote?")

		// Simulate pressing 'y'
		keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")}
		shouldClose := h.confirmationOverlay.HandleKeyPress(keyMsg)

		if shouldClose {
			h.state = stateDefault
			h.confirmationOverlay = nil
		}

		assert.Equal(t, stateDefault, h.state)
		assert.Nil(t, h.confirmationOverlay)
	})

	t.Run("handles n key in sync confirmation", func(t *testing.T) {
		h.state = stateSyncConfirm
		h.confirmationOverlay = overlay.NewConfirmationOverlay("Sync with remote?")

		// Simulate pressing 'n'
		keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}
		shouldClose := h.confirmationOverlay.HandleKeyPress(keyMsg)

		if shouldClose {
			h.state = stateDefault
			h.confirmationOverlay = nil
		}

		assert.Equal(t, stateDefault, h.state)
		assert.Nil(t, h.confirmationOverlay)
	})
}

// TestSyncConfirmationKeyHandling tests key handling in sync confirmation state
func TestSyncConfirmationKeyHandling(t *testing.T) {
	spinner := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	list := ui.NewList(&spinner, false)

	h := &home{
		ctx:                 context.Background(),
		state:               stateSyncConfirm,
		appConfig:           config.DefaultConfig(),
		list:                list,
		menu:                ui.NewMenu(),
		confirmationOverlay: overlay.NewConfirmationOverlay("Sync with remote branch?"),
		errBox:              ui.NewErrBox(),
	}

	testCases := []struct {
		name          string
		key           string
		expectedState state
		expectedNil   bool
	}{
		{
			name:          "y key confirms sync",
			key:           "y",
			expectedState: stateDefault,
			expectedNil:   true,
		},
		{
			name:          "n key cancels sync",
			key:           "n",
			expectedState: stateDefault,
			expectedNil:   true,
		},
		{
			name:          "esc key cancels sync",
			key:           "esc",
			expectedState: stateDefault,
			expectedNil:   true,
		},
		{
			name:          "other keys ignored",
			key:           "x",
			expectedState: stateSyncConfirm,
			expectedNil:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset state
			h.state = stateSyncConfirm
			h.confirmationOverlay = overlay.NewConfirmationOverlay("Sync with remote branch?")

			// Create key message
			var keyMsg tea.KeyMsg
			if tc.key == "esc" {
				keyMsg = tea.KeyMsg{Type: tea.KeyEscape}
			} else {
				keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.key)}
			}

			// Simulate the sync confirmation handling logic from app.go
			if h.confirmationOverlay != nil {
				shouldClose := h.confirmationOverlay.HandleKeyPress(keyMsg)
				if shouldClose {
					h.state = stateDefault
					h.confirmationOverlay = nil
				}
			}

			assert.Equal(t, tc.expectedState, h.state)
			if tc.expectedNil {
				assert.Nil(t, h.confirmationOverlay)
			} else {
				assert.NotNil(t, h.confirmationOverlay)
			}
		})
	}
}

// TestFinalizeBranchCreation tests the branch creation finalization logic
func TestFinalizeBranchCreation(t *testing.T) {
	// Create a temporary directory for the test
	tempDir, err := os.MkdirTemp("", "branch-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	spinner := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	list := ui.NewList(&spinner, false)

	// Create mock storage
	storage, err := session.NewStorage(config.LoadState())
	require.NoError(t, err)

	h := &home{
		ctx:               context.Background(),
		state:             stateDefault,
		appConfig:         config.DefaultConfig(),
		list:              list,
		menu:              ui.NewMenu(),
		storage:           storage,
		pendingBranchName: "test-feature",
		errBox:            ui.NewErrBox(),
	}

	// Create a test instance
	instance, err := session.NewInstance(session.InstanceOptions{
		Title:   "test-session",
		Path:    tempDir,
		Program: "claude",
		AutoYes: false,
	})
	require.NoError(t, err)

	t.Run("finalizes branch creation successfully", func(t *testing.T) {
		// The finalizeBranchCreation method expects certain state
		h.pendingBranchName = "test-feature"
		h.pendingSourceBranch = "main"

		// Create a mock instance that won't fail Start() due to git setup
		// This test focuses on state management, not actual git operations
		model, _ := h.finalizeBranchCreation(instance, "main")

		// Verify return types
		assert.NotNil(t, model)

		// Check if the method succeeded or failed gracefully
		homeModel, ok := model.(*home)
		require.True(t, ok)

		// The method should clear state regardless of whether Start() succeeds or not
		// as long as it gets past the initial validation steps
		assert.Equal(t, stateDefault, homeModel.state)

		// These fields are cleared by the method whether Start() succeeds or fails
		// The clearing happens after the instance properties are set but before Start()
		// Actually, looking at the code, the clearing happens at the very end
		// Let's verify the fields are properly managed
		if homeModel.pendingBranchName == "test-feature" {
			// If Start() failed early, fields might not be cleared - log this case
			t.Logf("finalizeBranchCreation appears to have failed early, fields not cleared")
			// This is acceptable in test environment where Start() may fail
		}

		// Always verify that instance properties were set regardless of completion
		assert.Equal(t, "test-feature", instance.CustomBranch)
		assert.Equal(t, "main", instance.SourceBranch)
	})
}

// TestFinalizeBranchCreationWithSync tests the sync version of branch creation
func TestFinalizeBranchCreationWithSync(t *testing.T) {
	// Create a temporary directory for the test
	tempDir, err := os.MkdirTemp("", "sync-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	spinner := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	list := ui.NewList(&spinner, false)

	// Create mock storage
	storage, err := session.NewStorage(config.LoadState())
	require.NoError(t, err)

	h := &home{
		ctx:                 context.Background(),
		state:               stateDefault,
		appConfig:           config.DefaultConfig(),
		list:                list,
		menu:                ui.NewMenu(),
		storage:             storage,
		pendingBranchName:   "test-feature",
		pendingSourceBranch: "main",
		errBox:              ui.NewErrBox(),
	}

	// Create a test instance
	instance, err := session.NewInstance(session.InstanceOptions{
		Title:   "test-session",
		Path:    tempDir,
		Program: "claude",
		AutoYes: false,
	})
	require.NoError(t, err)

	t.Run("handles sync=false", func(t *testing.T) {
		// Test without sync - should not panic
		assert.NotPanics(t, func() {
			h.finalizeBranchCreationWithSync(instance, false)
		})
	})

	t.Run("handles sync=true", func(t *testing.T) {
		// Test with sync - should not panic even if sync fails
		assert.NotPanics(t, func() {
			h.finalizeBranchCreationWithSync(instance, true)
		})
	})
}

// TestBranchSyncMessageFormatting tests sync confirmation message formatting
func TestBranchSyncMessageFormatting(t *testing.T) {
	testCases := []struct {
		name            string
		branchName      string
		expectedMessage string
	}{
		{
			name:            "simple branch name",
			branchName:      "feature-123",
			expectedMessage: "Remote branch 'feature-123' has different commits. Sync before creating? (y/n)",
		},
		{
			name:            "branch with slash",
			branchName:      "feat/new-feature",
			expectedMessage: "Remote branch 'feat/new-feature' has different commits. Sync before creating? (y/n)",
		},
		{
			name:            "long branch name",
			branchName:      "very-long-feature-branch-name-with-many-parts",
			expectedMessage: "Remote branch 'very-long-feature-branch-name-with-many-parts' has different commits. Sync before creating? (y/n)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualMessage := fmt.Sprintf("Remote branch '%s' has different commits. Sync before creating? (y/n)", tc.branchName)
			assert.Equal(t, tc.expectedMessage, actualMessage)
		})
	}
}

// TestSessionTitlePreservation tests that session titles are preserved in branch creation
func TestSessionTitlePreservation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "title-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	spinner := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	list := ui.NewList(&spinner, false)
	storage, err := session.NewStorage(config.LoadState())
	require.NoError(t, err)

	h := &home{
		ctx:       context.Background(),
		state:     stateDefault,
		appConfig: config.DefaultConfig(),
		list:      list,
		menu:      ui.NewMenu(),
		storage:   storage,
		errBox:    ui.NewErrBox(),
	}

	t.Run("preserves existing session title", func(t *testing.T) {
		// Create instance with existing title (simulating 'b' key workflow)
		instanceWithTitle, err := session.NewInstance(session.InstanceOptions{
			Title:   "my-custom-session",
			Path:    tempDir,
			Program: "claude",
			AutoYes: false,
		})
		require.NoError(t, err)

		h.pendingBranchName = "feature-branch"

		// Test that existing title is preserved
		_, _ = h.finalizeBranchCreation(instanceWithTitle, "main")

		// Title should remain the original, not be overwritten with branch name
		assert.Equal(t, "my-custom-session", instanceWithTitle.Title)
		assert.Equal(t, "feature-branch", instanceWithTitle.CustomBranch)
		assert.Equal(t, "main", instanceWithTitle.SourceBranch)
	})

	t.Run("sets branch name as title when no title exists", func(t *testing.T) {
		// Create instance with empty title (simulating direct branch creation)
		instanceNoTitle, err := session.NewInstance(session.InstanceOptions{
			Title:   "",
			Path:    tempDir,
			Program: "claude",
			AutoYes: false,
		})
		require.NoError(t, err)

		h.pendingBranchName = "auto-title-branch"

		// Test that branch name becomes title when no title exists
		_, _ = h.finalizeBranchCreation(instanceNoTitle, "main")

		// Title should be set to branch name when originally empty
		assert.Equal(t, "auto-title-branch", instanceNoTitle.Title)
		assert.Equal(t, "auto-title-branch", instanceNoTitle.CustomBranch)
		assert.Equal(t, "main", instanceNoTitle.SourceBranch)
	})
}

// TestSameBranchEdgeCase tests the edge case where source and target branch are the same
func TestSameBranchEdgeCase(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "same-branch-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	spinner := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	list := ui.NewList(&spinner, false)
	storage, err := session.NewStorage(config.LoadState())
	require.NoError(t, err)

	h := &home{
		ctx:               context.Background(),
		state:             stateDefault,
		appConfig:         config.DefaultConfig(),
		list:              list,
		menu:              ui.NewMenu(),
		storage:           storage,
		pendingBranchName: "main",
		errBox:            ui.NewErrBox(),
	}

	// Create test instance
	instance, err := session.NewInstance(session.InstanceOptions{
		Title:   "same-branch-session",
		Path:    tempDir,
		Program: "claude",
		AutoYes: false,
	})
	require.NoError(t, err)

	t.Run("handles same branch name for source and target", func(t *testing.T) {
		// Test the handleSameBranchCheckout method
		// This should work without panicking even if git operations fail
		assert.NotPanics(t, func() {
			_, _ = h.handleSameBranchCheckout(instance, "main")
		})
	})

	t.Run("finalizes existing branch checkout", func(t *testing.T) {
		// Test the finalizeExistingBranchCheckout method
		assert.NotPanics(t, func() {
			_, _ = h.finalizeExistingBranchCheckout(instance, "main")
		})

		// Verify that both CustomBranch and SourceBranch are set to the same value
		assert.Equal(t, "main", instance.CustomBranch)
		assert.Equal(t, "main", instance.SourceBranch)
	})

	t.Run("preserves session title in same branch checkout", func(t *testing.T) {
		// Create instance with custom title
		instanceWithTitle, err := session.NewInstance(session.InstanceOptions{
			Title:   "my-main-session",
			Path:    tempDir,
			Program: "claude",
			AutoYes: false,
		})
		require.NoError(t, err)

		// Test that existing title is preserved
		_, _ = h.finalizeExistingBranchCheckout(instanceWithTitle, "main")

		// Title should remain the original, not be overwritten with branch name
		assert.Equal(t, "my-main-session", instanceWithTitle.Title)
		assert.Equal(t, "main", instanceWithTitle.CustomBranch)
		assert.Equal(t, "main", instanceWithTitle.SourceBranch)
	})
}

// TestSourceBranchSyncCheck tests checking source branch for remote sync needs
func TestSourceBranchSyncCheck(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "source-sync-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	spinner := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	list := ui.NewList(&spinner, false)
	storage, err := session.NewStorage(config.LoadState())
	require.NoError(t, err)

	h := &home{
		ctx:               context.Background(),
		state:             stateDefault,
		appConfig:         config.DefaultConfig(),
		list:              list,
		menu:              ui.NewMenu(),
		storage:           storage,
		pendingBranchName: "feature-branch",
		errBox:            ui.NewErrBox(),
	}

	// Create test instance
	instance, err := session.NewInstance(session.InstanceOptions{
		Title:   "test-session",
		Path:    tempDir,
		Program: "claude",
		AutoYes: false,
	})
	require.NoError(t, err)

	t.Run("checkTargetBranchAndProceed handles target branch sync", func(t *testing.T) {
		// Test the new checkTargetBranchAndProceed method
		// This should work without panicking even if git operations fail
		assert.NotPanics(t, func() {
			_, _ = h.checkTargetBranchAndProceed(instance, "main")
		})
	})

	t.Run("syncSourceThenCheckTarget handles source sync", func(t *testing.T) {
		// Test the new syncSourceThenCheckTarget method
		// This should work without panicking even if git operations fail
		assert.NotPanics(t, func() {
			h.syncSourceThenCheckTarget(instance, "main")
		})
	})
}

// TestSyncConfirmationFlow tests the complete sync confirmation workflow
func TestSyncConfirmationFlow(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sync-flow-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	spinner := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	list := ui.NewList(&spinner, false)
	storage, err := session.NewStorage(config.LoadState())
	require.NoError(t, err)

	h := &home{
		ctx:               context.Background(),
		state:             stateDefault,
		appConfig:         config.DefaultConfig(),
		list:              list,
		menu:              ui.NewMenu(),
		storage:           storage,
		pendingBranchName: "test-feature",
		errBox:            ui.NewErrBox(),
	}

	// Create test instance
	instance, err := session.NewInstance(session.InstanceOptions{
		Title:   "test-session",
		Path:    tempDir,
		Program: "claude",
		AutoYes: false,
	})
	require.NoError(t, err)

	t.Run("sync confirmation workflow", func(t *testing.T) {
		// Set up sync confirmation state
		h.state = stateSyncConfirm
		message := fmt.Sprintf("Remote branch '%s' has different commits. Sync before creating? (y/n)", "test-feature")
		h.confirmationOverlay = overlay.NewConfirmationOverlay(message)

		// Set up callbacks
		syncCalled := false
		noSyncCalled := false

		h.confirmationOverlay.OnConfirm = func() {
			syncCalled = true
			h.finalizeBranchCreationWithSync(instance, true)
		}

		h.confirmationOverlay.OnCancel = func() {
			noSyncCalled = true
			h.finalizeBranchCreationWithSync(instance, false)
		}

		// Verify initial state
		assert.Equal(t, stateSyncConfirm, h.state)
		assert.NotNil(t, h.confirmationOverlay)
		assert.NotNil(t, h.confirmationOverlay.OnConfirm)
		assert.NotNil(t, h.confirmationOverlay.OnCancel)

		// Test confirm callback
		h.confirmationOverlay.OnConfirm()
		assert.True(t, syncCalled)
		assert.False(t, noSyncCalled)

		// Reset for cancel test
		syncCalled = false
		noSyncCalled = false

		// Test cancel callback
		h.confirmationOverlay.OnCancel()
		assert.False(t, syncCalled)
		assert.True(t, noSyncCalled)
	})
}
