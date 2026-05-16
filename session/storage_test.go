package session

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstanceDataDescriptionJSON(t *testing.T) {
	t.Run("Description serializes to JSON", func(t *testing.T) {
		data := InstanceData{
			Title:       "test",
			Description: "中文描述",
		}
		jsonData, err := json.Marshal(data)
		require.NoError(t, err)
		assert.Contains(t, string(jsonData), `"description":"中文描述"`)
	})

	t.Run("Description deserializes from JSON", func(t *testing.T) {
		jsonStr := `{"title":"test","description":"反序列化描述"}`
		var data InstanceData
		err := json.Unmarshal([]byte(jsonStr), &data)
		require.NoError(t, err)
		assert.Equal(t, "反序列化描述", data.Description)
	})

	t.Run("Missing description field defaults to empty string", func(t *testing.T) {
		// 向前兼容：老版本 state.json 中无 description 字段
		jsonStr := `{"title":"test"}`
		var data InstanceData
		err := json.Unmarshal([]byte(jsonStr), &data)
		require.NoError(t, err)
		assert.Equal(t, "", data.Description)
	})

	t.Run("Unknown fields are ignored", func(t *testing.T) {
		// 向后兼容：老版本忽略新字段
		jsonStr := `{"title":"test","description":"desc","future_field":"value"}`
		var data InstanceData
		err := json.Unmarshal([]byte(jsonStr), &data)
		require.NoError(t, err)
		assert.Equal(t, "desc", data.Description)
	})
}
