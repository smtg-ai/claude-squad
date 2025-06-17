package project

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProject(t *testing.T) {
	t.Run("creates valid project with absolute path", func(t *testing.T) {
		path := "/tmp/test-project"
		name := "Test Project"

		project, err := NewProject(path, name)

		require.NoError(t, err)
		assert.NotNil(t, project)
		assert.NotEmpty(t, project.ID)
		assert.Equal(t, name, project.Name)
		assert.Equal(t, path, project.Path)
		assert.False(t, project.IsActive)
		assert.NotNil(t, project.Instances)
		assert.Equal(t, 0, len(project.Instances))
		assert.WithinDuration(t, time.Now(), project.CreatedAt, time.Second)
		assert.WithinDuration(t, time.Now(), project.LastAccessed, time.Second)
	})

	t.Run("generates name from path when name is empty", func(t *testing.T) {
		path := "/home/user/my-awesome-project"

		project, err := NewProject(path, "")

		require.NoError(t, err)
		assert.Equal(t, "my-awesome-project", project.Name)
	})

	t.Run("fails with empty path", func(t *testing.T) {
		_, err := NewProject("", "Test Project")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "project path cannot be empty")
	})

	t.Run("fails with relative path", func(t *testing.T) {
		_, err := NewProject("./relative/path", "Test Project")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "project path must be absolute")
	})

	t.Run("fails when cannot determine name from path", func(t *testing.T) {
		_, err := NewProject("/", "")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "could not determine project name from path")
	})

	t.Run("fails when cannot determine name from dot path", func(t *testing.T) {
		_, err := NewProject("/.", "")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "could not determine project name from path")
	})

	t.Run("cleans path correctly", func(t *testing.T) {
		path := "/home/user/../user/project//subdir"

		project, err := NewProject(path, "Test")

		require.NoError(t, err)
		assert.Equal(t, filepath.Clean(path), project.Path)
	})
}

func TestGenerateProjectID(t *testing.T) {
	t.Run("generates ID from path", func(t *testing.T) {
		testCases := []struct {
			path     string
			expected string
		}{
			{"/home/user/project", "home_user_project"},
			{"/tmp/test", "tmp_test"},
			{"/", ""},
			{"/single", "single"},
			{"/home/user/my-project/nested", "home_user_my-project_nested"},
		}

		for _, tc := range testCases {
			t.Run(tc.path, func(t *testing.T) {
				id := generateProjectID(tc.path)
				assert.Equal(t, tc.expected, id)
			})
		}
	})
}

func TestProjectInstanceManagement(t *testing.T) {
	project := createTestProject(t)

	t.Run("adds instance", func(t *testing.T) {
		initialCount := project.InstanceCount()
		initialTime := project.LastAccessed
		time.Sleep(time.Millisecond) // Ensure time difference

		project.AddInstance("instance-1")

		assert.Equal(t, initialCount+1, project.InstanceCount())
		assert.True(t, project.HasInstance("instance-1"))
		assert.True(t, project.LastAccessed.After(initialTime))
	})

	t.Run("does not add empty instance ID", func(t *testing.T) {
		initialCount := project.InstanceCount()

		project.AddInstance("")

		assert.Equal(t, initialCount, project.InstanceCount())
	})

	t.Run("does not add duplicate instance", func(t *testing.T) {
		project.AddInstance("instance-2")
		initialCount := project.InstanceCount()

		project.AddInstance("instance-2")

		assert.Equal(t, initialCount, project.InstanceCount())
	})

	t.Run("removes instance", func(t *testing.T) {
		project.AddInstance("instance-to-remove")
		initialCount := project.InstanceCount()
		initialTime := project.LastAccessed
		time.Sleep(time.Millisecond) // Ensure time difference

		removed := project.RemoveInstance("instance-to-remove")

		assert.True(t, removed)
		assert.Equal(t, initialCount-1, project.InstanceCount())
		assert.False(t, project.HasInstance("instance-to-remove"))
		assert.True(t, project.LastAccessed.After(initialTime))
	})

	t.Run("returns false when removing non-existent instance", func(t *testing.T) {
		removed := project.RemoveInstance("non-existent")

		assert.False(t, removed)
	})

	t.Run("checks instance existence correctly", func(t *testing.T) {
		project.AddInstance("existing-instance")

		assert.True(t, project.HasInstance("existing-instance"))
		assert.False(t, project.HasInstance("non-existent"))
	})
}

func TestProjectActiveState(t *testing.T) {
	project := createTestProject(t)

	t.Run("starts inactive", func(t *testing.T) {
		assert.False(t, project.IsActive)
	})

	t.Run("sets active", func(t *testing.T) {
		initialTime := project.LastAccessed
		time.Sleep(time.Millisecond) // Ensure time difference

		project.SetActive()

		assert.True(t, project.IsActive)
		assert.True(t, project.LastAccessed.After(initialTime))
	})

	t.Run("sets inactive", func(t *testing.T) {
		project.SetActive()

		project.SetInactive()

		assert.False(t, project.IsActive)
	})
}

func TestProjectValidation(t *testing.T) {
	t.Run("validates complete project", func(t *testing.T) {
		project := createTestProject(t)

		err := project.Validate()

		assert.NoError(t, err)
	})

	t.Run("fails validation with empty ID", func(t *testing.T) {
		project := createTestProject(t)
		project.ID = ""

		err := project.Validate()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "project ID cannot be empty")
	})

	t.Run("fails validation with empty name", func(t *testing.T) {
		project := createTestProject(t)
		project.Name = ""

		err := project.Validate()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "project name cannot be empty")
	})

	t.Run("fails validation with empty path", func(t *testing.T) {
		project := createTestProject(t)
		project.Path = ""

		err := project.Validate()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "project path cannot be empty")
	})

	t.Run("fails validation with relative path", func(t *testing.T) {
		project := createTestProject(t)
		project.Path = "relative/path"

		err := project.Validate()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "project path must be absolute")
	})
}

func TestProjectInstanceSliceOperations(t *testing.T) {
	project := createTestProject(t)

	t.Run("removes instance from middle of slice", func(t *testing.T) {
		// Add multiple instances
		instances := []string{"instance-1", "instance-2", "instance-3", "instance-4"}
		for _, id := range instances {
			project.AddInstance(id)
		}

		// Remove from middle
		removed := project.RemoveInstance("instance-2")

		assert.True(t, removed)
		assert.Equal(t, 3, project.InstanceCount())
		assert.False(t, project.HasInstance("instance-2"))
		assert.True(t, project.HasInstance("instance-1"))
		assert.True(t, project.HasInstance("instance-3"))
		assert.True(t, project.HasInstance("instance-4"))
	})

	t.Run("removes instance from beginning of slice", func(t *testing.T) {
		project := createTestProject(t)
		instances := []string{"first", "second", "third"}
		for _, id := range instances {
			project.AddInstance(id)
		}

		removed := project.RemoveInstance("first")

		assert.True(t, removed)
		assert.Equal(t, 2, project.InstanceCount())
		assert.False(t, project.HasInstance("first"))
		assert.True(t, project.HasInstance("second"))
		assert.True(t, project.HasInstance("third"))
	})

	t.Run("removes instance from end of slice", func(t *testing.T) {
		project := createTestProject(t)
		instances := []string{"first", "second", "last"}
		for _, id := range instances {
			project.AddInstance(id)
		}

		removed := project.RemoveInstance("last")

		assert.True(t, removed)
		assert.Equal(t, 2, project.InstanceCount())
		assert.True(t, project.HasInstance("first"))
		assert.True(t, project.HasInstance("second"))
		assert.False(t, project.HasInstance("last"))
	})
}

func TestProjectEdgeCases(t *testing.T) {
	t.Run("handles special characters in path", func(t *testing.T) {
		path := "/home/user/project with spaces & special-chars_123"

		project, err := NewProject(path, "Special Project")

		require.NoError(t, err)
		assert.Equal(t, path, project.Path)
		assert.NotEmpty(t, project.ID)
	})

	t.Run("handles unicode in name", func(t *testing.T) {
		name := "プロジェクト名"

		project, err := NewProject("/tmp/unicode", name)

		require.NoError(t, err)
		assert.Equal(t, name, project.Name)
	})

	t.Run("generates unique IDs for different paths", func(t *testing.T) {
		project1, err1 := NewProject("/home/user/project1", "Project 1")
		project2, err2 := NewProject("/home/user/project2", "Project 2")

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, project1.ID, project2.ID)
	})

	t.Run("instance count works with empty project", func(t *testing.T) {
		project := createTestProject(t)

		assert.Equal(t, 0, project.InstanceCount())
	})
}

// Helper function to create a test project
func createTestProject(t *testing.T) *Project {
	project, err := NewProject("/tmp/test-project", "Test Project")
	require.NoError(t, err)
	return project
}
