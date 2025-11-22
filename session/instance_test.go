package session

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewInstance(t *testing.T) {
	t.Run("creates instance with HEAD base ref", func(t *testing.T) {
		instance, err := NewInstance(InstanceOptions{
			Title:   "test-instance",
			Path:    ".",
			Program: "claude",
			BaseRef: "HEAD",
		})

		assert.NoError(t, err)
		assert.Equal(t, "test-instance", instance.Title)
		assert.Equal(t, "HEAD", instance.BaseRef)
		assert.Equal(t, "claude", instance.Program)
	})

	t.Run("creates instance with main base ref", func(t *testing.T) {
		instance, err := NewInstance(InstanceOptions{
			Title:   "test-instance-main",
			Path:    ".",
			Program: "claude",
			BaseRef: "main",
		})

		assert.NoError(t, err)
		assert.Equal(t, "test-instance-main", instance.Title)
		assert.Equal(t, "main", instance.BaseRef)
		assert.Equal(t, "claude", instance.Program)
	})

	t.Run("creates instance with empty base ref", func(t *testing.T) {
		instance, err := NewInstance(InstanceOptions{
			Title:   "test-instance-empty",
			Path:    ".",
			Program: "claude",
			BaseRef: "",
		})

		assert.NoError(t, err)
		assert.Equal(t, "test-instance-empty", instance.Title)
		assert.Equal(t, "", instance.BaseRef)
		assert.Equal(t, "claude", instance.Program)
	})
}

func TestInstanceDataSerialization(t *testing.T) {
	t.Run("serializes BaseRef field", func(t *testing.T) {
		instance, err := NewInstance(InstanceOptions{
			Title:   "test-serialize",
			Path:    ".",
			Program: "claude",
			BaseRef: "main",
		})
		assert.NoError(t, err)

		data := instance.ToInstanceData()
		assert.Equal(t, "main", data.BaseRef)
		assert.Equal(t, "test-serialize", data.Title)
	})

	t.Run("deserializes BaseRef field", func(t *testing.T) {
		data := InstanceData{
			Title:   "test-deserialize",
			Path:    ".",
			Branch:  "test-branch",
			Status:  Ready,
			BaseRef: "HEAD",
			Program: "claude",
			Worktree: GitWorktreeData{
				RepoPath:     "/tmp/repo",
				WorktreePath: "/tmp/worktree",
				SessionName:  "test-deserialize",
				BranchName:   "test-branch",
			},
		}

		instance, err := FromInstanceData(data)
		assert.NoError(t, err)
		assert.Equal(t, "HEAD", instance.BaseRef)
		assert.Equal(t, "test-deserialize", instance.Title)
	})
}
