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
		assert.Equal(t, 3, l.numFilteredInstances())
	})

	t.Run("按标题模糊搜索", func(t *testing.T) {
		l.SetSearchQuery("login")
		assert.Equal(t, 1, l.numFilteredInstances())
		assert.Equal(t, "feat-login", l.getFilteredInstance(0).Title)
	})

	t.Run("按描述模糊搜索", func(t *testing.T) {
		l.SetSearchQuery("白屏")
		assert.Equal(t, 1, l.numFilteredInstances())
		assert.Equal(t, "fix-bug", l.getFilteredInstance(0).Title)
	})

	t.Run("按分支模糊搜索", func(t *testing.T) {
		l.SetSearchQuery("api")
		assert.Equal(t, 1, l.numFilteredInstances())
		assert.Equal(t, "refactor-api", l.getFilteredInstance(0).Title)
	})

	t.Run("大小写不敏感搜索", func(t *testing.T) {
		l.SetSearchQuery("LOGIN")
		assert.Equal(t, 1, l.numFilteredInstances())
	})

	t.Run("无匹配时返回空", func(t *testing.T) {
		l.SetSearchQuery("不存在的关键词")
		assert.Equal(t, 0, l.numFilteredInstances())
	})

	t.Run("过滤后选中项索引应更新为过滤结果中的位置", func(t *testing.T) {
		// 清空搜索，选中第 3 项（selectedIdx = 2）
		l.SetSearchQuery("")
		l.SetSelectedInstance(2)
		assert.Equal(t, "refactor-api", l.GetSelectedInstance().Title)

		// 搜索 "login" 只匹配第 1 项，当前选中项不在结果中，应自动选中第 1 项
		l.SetSearchQuery("login")
		assert.Equal(t, 0, l.selectedIdx)
		assert.Equal(t, "feat-login", l.GetSelectedInstance().Title)

		// 清空搜索，选中第 3 项
		l.SetSearchQuery("")
		l.SetSelectedInstance(2)

		// 搜索 "feat" 匹配第 1 项，当前选中项不在结果中
		l.SetSearchQuery("feat")
		assert.Equal(t, 0, l.selectedIdx)
		assert.Equal(t, "feat-login", l.GetSelectedInstance().Title)
	})

	t.Run("选中项在过滤结果中时索引应更新", func(t *testing.T) {
		// 清空搜索，选中第 2 项 fix-bug
		l.SetSearchQuery("")
		l.SetSelectedInstance(1)
		assert.Equal(t, "fix-bug", l.GetSelectedInstance().Title)

		// 搜索 "白屏" 只匹配 fix-bug（l.items 索引 1）
		// 过滤后 filteredItems = [fix-bug]，selectedIdx 应更新为 0
		l.SetSearchQuery("白屏")
		assert.Equal(t, 0, l.selectedIdx)
		assert.Equal(t, "fix-bug", l.GetSelectedInstance().Title)
	})
}

func TestHighlightMatch(t *testing.T) {
	t.Run("高亮匹配子串", func(t *testing.T) {
		result := HighlightMatch("feat-login", "login", "\x1b[39m")
		assert.Contains(t, result, "login")
	})

	t.Run("空搜索词不改变原文", func(t *testing.T) {
		result := HighlightMatch("feat-login", "", "\x1b[39m")
		assert.Equal(t, "feat-login", result)
	})

	t.Run("大小写不敏感高亮", func(t *testing.T) {
		result := HighlightMatch("feat-LOGIN", "login", "\x1b[39m")
		assert.Contains(t, result, "LOGIN")
	})

	t.Run("无匹配时不改变原文", func(t *testing.T) {
		result := HighlightMatch("feat-login", "不存在", "\x1b[39m")
		assert.Equal(t, "feat-login", result)
	})

	t.Run("选中态恢复前景色", func(t *testing.T) {
		restoreFg := "\x1b[38;2;26;26;26m"
		result := HighlightMatch("feat-login", "login", restoreFg)
		// 匹配后应有重置前缀+恢复前景色序列
		assert.Contains(t, result, matchHighlightResetPrefix)
		assert.Contains(t, result, restoreFg)
	})

	t.Run("非选中态恢复默认前景色", func(t *testing.T) {
		result := HighlightMatch("feat-login", "login", "\x1b[39m")
		assert.Contains(t, result, "\x1b[39m")
	})
}