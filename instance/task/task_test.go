package task

import (
	"claude-squad/config"
	"claude-squad/log"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

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

// mockConfigDir temporarily overrides the config directory for testing
func mockConfigDir(t *testing.T, tempDir string) func() {
	// Store original home directory
	originalHome := os.Getenv("HOME")

	// Create a fake home directory in tempdir
	fakeHome := filepath.Join(tempDir, "home")
	err := os.MkdirAll(fakeHome, 0755)
	require.NoError(t, err)

	// Set HOME to our fake home directory
	err = os.Setenv("HOME", fakeHome)
	require.NoError(t, err)

	// Return cleanup function
	return func() {
		os.Setenv("HOME", originalHome)
	}
}

// TestInstanceCreateDeleteRecreate tests creating an instance with name "asdf",
// deleting it, and recreating it with the same name should produce no error.
func TestInstanceCreateDeleteRecreate(t *testing.T) {
	// Use tempdir for test isolation
	tempDir := t.TempDir()

	// Mock the config directory to use tempdir for worktrees as well
	cleanup := mockConfigDir(t, tempDir)
	defer cleanup()

	// Verify the config directory is using our tempdir
	configDir, err := config.GetConfigDir()
	require.NoError(t, err)
	assert.Contains(t, configDir, tempDir, "Config directory should be within tempdir")

	// Verify worktrees directory would be created in tempdir too
	expectedWorktreesDir := filepath.Join(configDir, "worktrees")
	assert.Contains(t, expectedWorktreesDir, tempDir, "Worktrees directory should be within tempdir")

	// Create first instance with name "asdf"
	instance1, err := NewTask(TaskOptions{
		Title:   "asdf",
		Path:    tempDir,
		Program: "claude",
		AutoYes: false,
	})
	require.NoError(t, err)
	assert.Equal(t, "asdf", instance1.Title)
	assert.Equal(t, tempDir, instance1.Path)
	assert.Equal(t, "claude", instance1.Program)
	assert.False(t, instance1.AutoYes)
	assert.Equal(t, Loading, instance1.Status)

	// Delete the instance (simulate cleanup)
	err = instance1.Kill()
	require.NoError(t, err)

	// Recreate instance with same name "asdf" - should not error
	instance2, err := NewTask(TaskOptions{
		Title:   "asdf",
		Path:    tempDir,
		Program: "claude",
		AutoYes: false,
	})
	require.NoError(t, err)
	assert.Equal(t, "asdf", instance2.Title)
	assert.Equal(t, tempDir, instance2.Path)
	assert.Equal(t, "claude", instance2.Program)
	assert.False(t, instance2.AutoYes)
	assert.Equal(t, Loading, instance2.Status)

	// Verify instances are separate objects
	assert.NotSame(t, instance1, instance2)

	// Clean up second instance
	err = instance2.Kill()
	require.NoError(t, err)
}

// TestRapidInstanceCreationDeletion tests rapid creation and deletion of instances
// to reproduce real-world race conditions that might not show up in basic tests
func TestRapidInstanceCreationDeletion(t *testing.T) {
	// Use tempdir for test isolation
	tempDir := t.TempDir()

	// Mock the config directory to use tempdir
	cleanup := mockConfigDir(t, tempDir)
	defer cleanup()

	// Test creating and destroying the same instance name multiple times rapidly
	for i := 0; i < 5; i++ {
		t.Run(fmt.Sprintf("Iteration_%d", i), func(t *testing.T) {
			// Create instance
			instance, err := NewTask(TaskOptions{
				Title:   "rapid-test",
				Path:    tempDir,
				Program: "claude",
				AutoYes: false,
			})
			require.NoError(t, err, "Failed to create instance on iteration %d", i)
			assert.Equal(t, "rapid-test", instance.Title)

			// Immediately delete it
			err = instance.Kill()
			require.NoError(t, err, "Failed to kill instance on iteration %d", i)

			// Brief pause to simulate real timing
			time.Sleep(10 * time.Millisecond)
		})
	}
}
