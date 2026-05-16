package ui

import (
	"claude-squad/session"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstanceRendererDescription(t *testing.T) {
	sp := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	r := &InstanceRenderer{spinner: &sp}
	r.setWidth(60)

	t.Run("renders description line when Description is set", func(t *testing.T) {
		inst, err := session.NewInstance(session.InstanceOptions{
			Title:       "test",
			Path:        ".",
			Program:     "claude",
			Description: "这是一个描述",
		})
		require.NoError(t, err)
		inst.SetStatus(session.Ready)
		result := r.Render(inst, 1, false, false)
		assert.Contains(t, result, "☰")
		assert.Contains(t, result, "这是一个描述")
	})

	t.Run("does not render description line when Description is empty", func(t *testing.T) {
		inst, err := session.NewInstance(session.InstanceOptions{
			Title:   "test",
			Path:    ".",
			Program: "claude",
		})
		require.NoError(t, err)
		inst.SetStatus(session.Ready)
		result := r.Render(inst, 1, false, false)
		assert.NotContains(t, result, "☰")
	})

	t.Run("truncates long description with ellipsis", func(t *testing.T) {
		longDesc := "这是一个非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常长的描述"
		inst, err := session.NewInstance(session.InstanceOptions{
			Title:       "test",
			Path:        ".",
			Program:     "claude",
			Description: longDesc,
		})
		require.NoError(t, err)
		inst.SetStatus(session.Ready)
		result := r.Render(inst, 1, false, false)
		assert.Contains(t, result, "☰")
		assert.Contains(t, result, "…")
	})
}
