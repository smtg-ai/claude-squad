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

	t.Run("renders description line when descInputActive even if Description is empty", func(t *testing.T) {
		inst, err := session.NewInstance(session.InstanceOptions{
			Title:   "test",
			Path:    ".",
			Program: "claude",
		})
		require.NoError(t, err)
		inst.SetStatus(session.Ready)
		r.descInputActive = true
		result := r.Render(inst, 1, false, false)
		assert.Contains(t, result, "☰")
		r.descInputActive = false
	})

	t.Run("does not render description line when not active and Description is empty", func(t *testing.T) {
		inst, err := session.NewInstance(session.InstanceOptions{
			Title:   "test",
			Path:    ".",
			Program: "claude",
		})
		require.NoError(t, err)
		inst.SetStatus(session.Ready)
		r.descInputActive = false
		result := r.Render(inst, 1, false, false)
		assert.NotContains(t, result, "☰")
	})
}

func TestListSearchFilter(t *testing.T) {
	sp := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	l := NewList(&sp, false)
	l.SetSize(60, 20)

	inst1, _ := session.NewInstance(session.InstanceOptions{Title: "feat-login", Path: ".", Program: "claude", Description: "实现用户登录"})
	inst1.Branch = "feat/login"
	inst2, _ := session.NewInstance(session.InstanceOptions{Title: "fix-bug", Path: ".", Program: "claude", Description: "修复首页白屏"})
	inst2.Branch = "fix/white-screen"
	inst3, _ := session.NewInstance(session.InstanceOptions{Title: "refactor-api", Path: ".", Program: "aider"})
	inst3.Branch = "refactor/api"

	l.AddInstance(inst1)
	l.AddInstance(inst2)
	l.AddInstance(inst3)

	t.Run("空搜索词返回全部实例", func(t *testing.T) {
		l.SetSearchQuery("")
		assert.Equal(t, 3, l.NumFilteredInstances())
	})

	t.Run("按标题模糊搜索", func(t *testing.T) {
		l.SetSearchQuery("login")
		assert.Equal(t, 1, l.NumFilteredInstances())
		assert.Equal(t, "feat-login", l.GetFilteredInstance(0).Title)
	})

	t.Run("按描述模糊搜索", func(t *testing.T) {
		l.SetSearchQuery("白屏")
		assert.Equal(t, 1, l.NumFilteredInstances())
		assert.Equal(t, "fix-bug", l.GetFilteredInstance(0).Title)
	})

	t.Run("按分支模糊搜索", func(t *testing.T) {
		l.SetSearchQuery("api")
		assert.Equal(t, 1, l.NumFilteredInstances())
		assert.Equal(t, "refactor-api", l.GetFilteredInstance(0).Title)
	})

	t.Run("大小写不敏感搜索", func(t *testing.T) {
		l.SetSearchQuery("LOGIN")
		assert.Equal(t, 1, l.NumFilteredInstances())
	})

	t.Run("无匹配时返回空", func(t *testing.T) {
		l.SetSearchQuery("不存在的关键词")
		assert.Equal(t, 0, l.NumFilteredInstances())
	})
}