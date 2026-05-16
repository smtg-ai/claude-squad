package overlay

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDescriptionInput(t *testing.T) {
	t.Run("GetDescription returns empty string by default", func(t *testing.T) {
		o := NewTextInputOverlayWithBranchPicker("Enter prompt", "", nil)
		assert.Equal(t, "", o.GetDescription())
	})

	t.Run("isDescriptionInput returns true when focused", func(t *testing.T) {
		o := NewTextInputOverlayWithBranchPicker("Enter prompt", "", nil)
		o.setFocusIndex(1)
		assert.True(t, o.isDescriptionInput())
	})

	t.Run("isDescriptionInput returns false when not focused", func(t *testing.T) {
		o := NewTextInputOverlayWithBranchPicker("Enter prompt", "", nil)
		o.setFocusIndex(0)
		assert.False(t, o.isDescriptionInput())
	})

	t.Run("Tab cycles through stops including description", func(t *testing.T) {
		o := NewTextInputOverlayWithBranchPicker("Enter prompt", "", nil)
		// 无 profile 时: textarea(0) -> description(1) -> branch(2) -> enter(3)
		assert.True(t, o.isTextarea())
		o.setFocusIndex(1)
		assert.True(t, o.isDescriptionInput())
		o.setFocusIndex(2)
		assert.True(t, o.isBranchPicker())
		o.setFocusIndex(3)
		assert.True(t, o.isEnterButton())
	})
}
