package session

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstanceDescription(t *testing.T) {
	t.Run("NewInstance sets Description from options", func(t *testing.T) {
		inst, err := NewInstance(InstanceOptions{
			Title:       "test",
			Path:        ".",
			Program:     "claude",
			Description: "这是一个测试描述",
		})
		require.NoError(t, err)
		assert.Equal(t, "这是一个测试描述", inst.Description)
	})

	t.Run("SetDescription sets the description", func(t *testing.T) {
		inst, err := NewInstance(InstanceOptions{
			Title:   "test",
			Path:    ".",
			Program: "claude",
		})
		require.NoError(t, err)
		inst.SetDescription("新描述")
		assert.Equal(t, "新描述", inst.Description)
	})

	t.Run("ToInstanceData includes Description", func(t *testing.T) {
		inst, err := NewInstance(InstanceOptions{
			Title:       "test",
			Path:        ".",
			Program:     "claude",
			Description: "描述内容",
		})
		require.NoError(t, err)
		data := inst.ToInstanceData()
		assert.Equal(t, "描述内容", data.Description)
	})

	t.Run("FromInstanceData includes Description", func(t *testing.T) {
		data := InstanceData{
			Title:       "test",
			Path:        ".",
			Program:     "claude",
			Description: "反序列化描述",
		}
		inst, err := FromInstanceData(data)
		require.NoError(t, err)
		assert.Equal(t, "反序列化描述", inst.Description)
	})

	t.Run("Description defaults to empty string", func(t *testing.T) {
		inst, err := NewInstance(InstanceOptions{
			Title:   "test",
			Path:    ".",
			Program: "claude",
		})
		require.NoError(t, err)
		assert.Equal(t, "", inst.Description)
	})
}
