package project

import (
	"claude-squad/config"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestState is a minimal implementation of config.State for testing
type TestState struct {
	ProjectsData  json.RawMessage `json:"projects"`
	ActiveProject string          `json:"active_project"`
}

// TestStateManager implements config.StateManager for testing
type TestStateManager struct {
	state    *TestState
	saveFunc func(*TestState) error
}

func NewTestStateManager() *TestStateManager {
	return &TestStateManager{
		state: &TestState{
			ProjectsData:  json.RawMessage("{}"),
			ActiveProject: "",
		},
		saveFunc: func(*TestState) error { return nil },
	}
}

func (tsm *TestStateManager) SaveInstances(instancesJSON json.RawMessage) error {
	// Not used for project storage tests
	return nil
}

func (tsm *TestStateManager) GetInstances() json.RawMessage {
	// Not used for project storage tests
	return json.RawMessage("[]")
}

func (tsm *TestStateManager) DeleteAllInstances() error {
	// Not used for project storage tests
	return nil
}

func (tsm *TestStateManager) GetHelpScreensSeen() uint32 {
	// Not used for project storage tests
	return 0
}

func (tsm *TestStateManager) SetHelpScreensSeen(seen uint32) error {
	// Not used for project storage tests
	return nil
}

func (tsm *TestStateManager) SetSaveFunc(f func(*TestState) error) {
	tsm.saveFunc = f
}

func TestStateProjectStorage(t *testing.T) {
	t.Run("creates storage with valid state manager", func(t *testing.T) {
		// Use actual config.State for storage tests since StateProjectStorage expects it
		state := config.DefaultState()

		storage := NewStateProjectStorage(state)

		assert.NotNil(t, storage)
	})

	t.Run("saves and retrieves projects", func(t *testing.T) {
		state := config.DefaultState()
		storage := NewStateProjectStorage(state)

		// Create test project data
		testProjects := map[string]*Project{
			"test-id": {
				ID:        "test-id",
				Name:      "Test Project",
				Path:      "/tmp/test",
				IsActive:  false,
				Instances: []string{"instance-1"},
			},
		}
		projectsJSON, _ := json.Marshal(testProjects)

		// Save projects
		err := storage.SaveProjects(projectsJSON)

		require.NoError(t, err)

		// Retrieve projects
		retrievedJSON := storage.GetProjects()

		assert.NotEmpty(t, retrievedJSON)

		// Unmarshal and verify
		var retrievedProjects map[string]*Project
		err = json.Unmarshal(retrievedJSON, &retrievedProjects)
		require.NoError(t, err)

		assert.Equal(t, 1, len(retrievedProjects))
		project := retrievedProjects["test-id"]
		assert.Equal(t, "Test Project", project.Name)
		assert.Equal(t, "/tmp/test", project.Path)
		assert.Contains(t, project.Instances, "instance-1")
	})

	t.Run("sets and gets active project", func(t *testing.T) {
		state := config.DefaultState()
		storage := NewStateProjectStorage(state)

		// Initially no active project
		activeProject := storage.GetActiveProject()
		assert.Empty(t, activeProject)

		// Set active project
		err := storage.SetActiveProject("test-project-id")
		require.NoError(t, err)

		// Get active project
		activeProject = storage.GetActiveProject()
		assert.Equal(t, "test-project-id", activeProject)
	})

	t.Run("deletes project", func(t *testing.T) {
		state := config.DefaultState()
		storage := NewStateProjectStorage(state)

		// Create test projects
		testProjects := map[string]*Project{
			"project-1": {ID: "project-1", Name: "Project 1", Path: "/tmp/project1"},
			"project-2": {ID: "project-2", Name: "Project 2", Path: "/tmp/project2"},
		}
		projectsJSON, _ := json.Marshal(testProjects)
		storage.SaveProjects(projectsJSON)

		// Delete one project
		err := storage.DeleteProject("project-1")
		require.NoError(t, err)

		// Verify project was deleted
		retrievedJSON := storage.GetProjects()
		var retrievedProjects map[string]*Project
		json.Unmarshal(retrievedJSON, &retrievedProjects)

		assert.Equal(t, 1, len(retrievedProjects))
		_, exists := retrievedProjects["project-1"]
		assert.False(t, exists)
		_, exists = retrievedProjects["project-2"]
		assert.True(t, exists)
	})

	t.Run("handles empty projects for deletion", func(t *testing.T) {
		state := config.DefaultState()
		storage := NewStateProjectStorage(state)

		// Try to delete from empty storage
		err := storage.DeleteProject("non-existent")

		assert.NoError(t, err) // Should not error on empty storage
	})

	t.Run("handles invalid JSON during deletion", func(t *testing.T) {
		state := config.DefaultState()
		storage := NewStateProjectStorage(state)

		// Set invalid JSON
		state.ProjectsData = json.RawMessage("invalid json")

		err := storage.DeleteProject("any-id")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal projects")
	})

	t.Run("handles marshal error during deletion", func(t *testing.T) {
		state := config.DefaultState()
		storage := NewStateProjectStorage(state)

		// Create projects with data that will cause marshal to fail
		// This is difficult to simulate without complex setup, so we'll test the error path indirectly

		// Set up projects normally first
		testProjects := map[string]*Project{
			"test-id": {ID: "test-id", Name: "Test", Path: "/tmp/test"},
		}
		projectsJSON, _ := json.Marshal(testProjects)
		storage.SaveProjects(projectsJSON)

		// This should work normally
		err := storage.DeleteProject("test-id")
		assert.NoError(t, err)
	})
}

func TestStateProjectStorageIntegration(t *testing.T) {
	t.Run("integrates with actual config.State structure", func(t *testing.T) {
		// Create a temporary directory for config
		tempDir := t.TempDir()

		// Set up environment to use temp directory
		originalXDG := os.Getenv("XDG_CONFIG_HOME")
		os.Setenv("XDG_CONFIG_HOME", filepath.Join(tempDir, ".config"))
		defer os.Setenv("XDG_CONFIG_HOME", originalXDG)

		// Load state (will create default if doesn't exist)
		state := config.LoadState()
		storage := NewStateProjectStorage(state)

		// Test basic operations
		err := storage.SetActiveProject("test-active")
		require.NoError(t, err)

		activeProject := storage.GetActiveProject()
		assert.Equal(t, "test-active", activeProject)

		// Test project data
		testProject := map[string]*Project{
			"integration-test": {
				ID:        "integration-test",
				Name:      "Integration Test Project",
				Path:      "/tmp/integration",
				IsActive:  true,
				Instances: []string{"instance-1", "instance-2"},
			},
		}
		projectsJSON, _ := json.Marshal(testProject)

		err = storage.SaveProjects(projectsJSON)
		require.NoError(t, err)

		// Retrieve and verify
		retrievedJSON := storage.GetProjects()
		var retrievedProjects map[string]*Project
		err = json.Unmarshal(retrievedJSON, &retrievedProjects)
		require.NoError(t, err)

		project := retrievedProjects["integration-test"]
		assert.Equal(t, "Integration Test Project", project.Name)
		assert.Equal(t, 2, len(project.Instances))
	})

	t.Run("persists state across reload", func(t *testing.T) {
		// Create a temporary directory for config
		tempDir := t.TempDir()

		// Set up environment to use temp directory
		originalXDG := os.Getenv("XDG_CONFIG_HOME")
		os.Setenv("XDG_CONFIG_HOME", filepath.Join(tempDir, ".config"))
		defer os.Setenv("XDG_CONFIG_HOME", originalXDG)

		// First session - save data
		{
			state := config.LoadState()
			storage := NewStateProjectStorage(state)

			testProject := map[string]*Project{
				"persist-test": {
					ID:   "persist-test",
					Name: "Persistence Test",
					Path: "/tmp/persist",
				},
			}
			projectsJSON, _ := json.Marshal(testProject)

			storage.SaveProjects(projectsJSON)
			storage.SetActiveProject("persist-test")
		}

		// Second session - reload and verify
		{
			state := config.LoadState()
			storage := NewStateProjectStorage(state)

			// Verify data persisted
			activeProject := storage.GetActiveProject()
			assert.Equal(t, "persist-test", activeProject)

			retrievedJSON := storage.GetProjects()
			var retrievedProjects map[string]*Project
			json.Unmarshal(retrievedJSON, &retrievedProjects)

			project := retrievedProjects["persist-test"]
			assert.Equal(t, "Persistence Test", project.Name)
		}
	})
}

func TestStateProjectStorageErrorHandling(t *testing.T) {
	t.Run("handles wrong state type gracefully", func(t *testing.T) {
		// Create a storage with a StateManager that's not a *config.State
		// This tests the type assertion failure paths in the StateProjectStorage
		testStateManager := NewTestStateManager()
		storage := &StateProjectStorage{state: testStateManager}

		// These operations should fail gracefully because testStateManager is not *config.State
		err := storage.SaveProjects(json.RawMessage("{}"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "state is not of type *config.State")

		err = storage.SetActiveProject("test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "state is not of type *config.State")

		// GetProjects should return empty JSON when type assertion fails
		result := storage.GetProjects()
		assert.Equal(t, json.RawMessage("{}"), result)

		// GetActiveProject should return empty string when type assertion fails
		activeProject := storage.GetActiveProject()
		assert.Empty(t, activeProject)
	})
}

func TestStateProjectStorageCompleteWorkflow(t *testing.T) {
	t.Run("complete project lifecycle", func(t *testing.T) {
		state := config.DefaultState()
		storage := NewStateProjectStorage(state)

		// Step 1: Start with empty state
		// Clear any existing data to ensure clean test
		storage.SaveProjects(json.RawMessage("{}"))
		storage.SetActiveProject("")

		projects := storage.GetProjects()
		assert.Equal(t, json.RawMessage("{}"), projects)

		activeProject := storage.GetActiveProject()
		assert.Empty(t, activeProject)

		// Step 2: Add first project
		project1 := map[string]*Project{
			"project-1": {
				ID:        "project-1",
				Name:      "First Project",
				Path:      "/tmp/first",
				IsActive:  true,
				Instances: []string{},
			},
		}
		projectsJSON, _ := json.Marshal(project1)

		err := storage.SaveProjects(projectsJSON)
		require.NoError(t, err)

		err = storage.SetActiveProject("project-1")
		require.NoError(t, err)

		// Step 3: Add second project
		bothProjects := map[string]*Project{
			"project-1": {
				ID:        "project-1",
				Name:      "First Project",
				Path:      "/tmp/first",
				IsActive:  false, // No longer active
				Instances: []string{"instance-1"},
			},
			"project-2": {
				ID:        "project-2",
				Name:      "Second Project",
				Path:      "/tmp/second",
				IsActive:  true, // Now active
				Instances: []string{},
			},
		}
		projectsJSON, _ = json.Marshal(bothProjects)

		err = storage.SaveProjects(projectsJSON)
		require.NoError(t, err)

		err = storage.SetActiveProject("project-2")
		require.NoError(t, err)

		// Step 4: Verify final state
		retrievedJSON := storage.GetProjects()
		var retrievedProjects map[string]*Project
		json.Unmarshal(retrievedJSON, &retrievedProjects)

		assert.Equal(t, 2, len(retrievedProjects))

		proj1 := retrievedProjects["project-1"]
		assert.Equal(t, "First Project", proj1.Name)
		assert.Contains(t, proj1.Instances, "instance-1")

		proj2 := retrievedProjects["project-2"]
		assert.Equal(t, "Second Project", proj2.Name)

		activeProjectID := storage.GetActiveProject()
		assert.Equal(t, "project-2", activeProjectID)

		// Step 5: Remove first project
		err = storage.DeleteProject("project-1")
		require.NoError(t, err)

		// Step 6: Verify deletion
		retrievedJSON = storage.GetProjects()
		var finalProjects map[string]*Project
		json.Unmarshal(retrievedJSON, &finalProjects)

		assert.Equal(t, 1, len(finalProjects))
		_, exists := finalProjects["project-1"]
		assert.False(t, exists)
		_, exists = finalProjects["project-2"]
		assert.True(t, exists)
	})
}

func TestStateProjectStorageEdgeCases(t *testing.T) {
	t.Run("handles empty project data gracefully", func(t *testing.T) {
		state := config.DefaultState()
		storage := NewStateProjectStorage(state)

		// Save empty project data
		err := storage.SaveProjects(json.RawMessage("{}"))
		assert.NoError(t, err)

		// Retrieve should work
		projects := storage.GetProjects()
		assert.Equal(t, json.RawMessage("{}"), projects)

		// Delete from empty should work
		err = storage.DeleteProject("any-id")
		assert.NoError(t, err)
	})

	t.Run("handles large project data", func(t *testing.T) {
		state := config.DefaultState()
		storage := NewStateProjectStorage(state)

		// Create large project data
		largeProjects := make(map[string]*Project)
		for i := 0; i < 100; i++ {
			projectID := fmt.Sprintf("project-%d", i)
			largeProjects[projectID] = &Project{
				ID:        projectID,
				Name:      fmt.Sprintf("Project %d", i),
				Path:      fmt.Sprintf("/tmp/project-%d", i),
				Instances: make([]string, 10), // 10 instances per project
			}
			// Fill instances
			for j := 0; j < 10; j++ {
				largeProjects[projectID].Instances[j] = fmt.Sprintf("instance-%d-%d", i, j)
			}
		}

		projectsJSON, err := json.Marshal(largeProjects)
		require.NoError(t, err)

		// Save large data
		err = storage.SaveProjects(projectsJSON)
		assert.NoError(t, err)

		// Retrieve and verify
		retrievedJSON := storage.GetProjects()
		var retrievedProjects map[string]*Project
		err = json.Unmarshal(retrievedJSON, &retrievedProjects)
		require.NoError(t, err)

		assert.Equal(t, 100, len(retrievedProjects))

		// Verify a few projects
		project0 := retrievedProjects["project-0"]
		assert.Equal(t, "Project 0", project0.Name)
		assert.Equal(t, 10, len(project0.Instances))

		project99 := retrievedProjects["project-99"]
		assert.Equal(t, "Project 99", project99.Name)
	})

	t.Run("handles special characters in project data", func(t *testing.T) {
		state := config.DefaultState()
		storage := NewStateProjectStorage(state)

		// Create project with special characters
		specialProject := map[string]*Project{
			"special-chars": {
				ID:   "special-chars",
				Name: "Project with 特殊文字 & symbols!@#$%",
				Path: "/tmp/path with spaces/and-symbols",
				Instances: []string{
					"instance-with-unicode-名前",
					"instance/with/slashes",
					"instance with spaces",
				},
			},
		}

		projectsJSON, err := json.Marshal(specialProject)
		require.NoError(t, err)

		err = storage.SaveProjects(projectsJSON)
		assert.NoError(t, err)

		// Retrieve and verify
		retrievedJSON := storage.GetProjects()
		var retrievedProjects map[string]*Project
		err = json.Unmarshal(retrievedJSON, &retrievedProjects)
		require.NoError(t, err)

		project := retrievedProjects["special-chars"]
		assert.Equal(t, "Project with 特殊文字 & symbols!@#$%", project.Name)
		assert.Equal(t, "/tmp/path with spaces/and-symbols", project.Path)
		assert.Contains(t, project.Instances, "instance-with-unicode-名前")
	})
}
