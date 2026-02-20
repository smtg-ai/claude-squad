package app

import (
	"fmt"

	"github.com/ByteMirror/hivemind/session"
	"github.com/ByteMirror/hivemind/ui"
	"github.com/ByteMirror/hivemind/ui/overlay"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
)

// removeAgentFromBrain removes an agent's brain entry when an instance is killed or paused.
func (m *home) removeAgentFromBrain(instance *session.Instance) {
	if m.brainServer == nil || instance == nil {
		return
	}
	repoPath := instance.GetRepoPath()
	if repoPath == "" {
		return
	}
	m.brainServer.Manager().RemoveAgent(repoPath, instance.Title)
}

// executeContextAction performs the action selected from a context menu.
func (m *home) executeContextAction(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "kill_all_in_topic":
		selectedID := m.sidebar.GetSelectedID()
		if selectedID == ui.SidebarAll || ui.IsUngroupedID(selectedID) {
			return m, nil
		}
		killAction := func() tea.Msg {
			// Clean up brain entries and remove from allInstances before killing
			for i := len(m.allInstances) - 1; i >= 0; i-- {
				if m.allInstances[i].TopicName == selectedID {
					m.removeAgentFromBrain(m.allInstances[i])
					m.allInstances = append(m.allInstances[:i], m.allInstances[i+1:]...)
				}
			}
			m.list.KillInstancesByTopic(selectedID)
			m.saveAllInstances()
			m.updateSidebarItems()
			return instanceChangedMsg{}
		}
		message := fmt.Sprintf("[!] Kill all instances in topic '%s'?", selectedID)
		return m, m.confirmAction(message, killAction)

	case "delete_topic_and_instances":
		selectedID := m.sidebar.GetSelectedID()
		if selectedID == ui.SidebarAll || ui.IsUngroupedID(selectedID) {
			return m, nil
		}
		deleteAction := func() tea.Msg {
			// Clean up brain entries and remove from allInstances before killing
			for i := len(m.allInstances) - 1; i >= 0; i-- {
				if m.allInstances[i].TopicName == selectedID {
					m.removeAgentFromBrain(m.allInstances[i])
					m.allInstances = append(m.allInstances[:i], m.allInstances[i+1:]...)
				}
			}
			m.list.KillInstancesByTopic(selectedID)
			for i, t := range m.topics {
				if t.Name == selectedID {
					t.Cleanup()
					m.topics = append(m.topics[:i], m.topics[i+1:]...)
					break
				}
			}
			for i, t := range m.allTopics {
				if t.Name == selectedID {
					m.allTopics = append(m.allTopics[:i], m.allTopics[i+1:]...)
					break
				}
			}
			m.saveAllInstances()
			m.saveAllTopics()
			m.updateSidebarItems()
			return instanceChangedMsg{}
		}
		message := fmt.Sprintf("[!] Delete topic '%s' and kill all its instances?", selectedID)
		return m, m.confirmAction(message, deleteAction)

	case "delete_topic":
		selectedID := m.sidebar.GetSelectedID()
		// Remove all instances in this topic first
		for _, inst := range m.allInstances {
			if inst.TopicName == selectedID {
				inst.TopicName = ""
			}
		}
		// Remove the topic
		for i, t := range m.topics {
			if t.Name == selectedID {
				t.Cleanup()
				m.topics = append(m.topics[:i], m.topics[i+1:]...)
				break
			}
		}
		for i, t := range m.allTopics {
			if t.Name == selectedID {
				m.allTopics = append(m.allTopics[:i], m.allTopics[i+1:]...)
				break
			}
		}
		m.updateSidebarItems()
		m.saveAllInstances()
		m.saveAllTopics()
		return m, tea.WindowSize()

	case "kill_instance":
		selected := m.list.GetSelectedInstance()
		if selected != nil {
			m.removeAgentFromBrain(selected)
			title := selected.Title
			m.tabbedWindow.GetTerminalPane().KillSession(title)
			m.removeFromAllInstances(title)
			m.list.Kill()
			m.saveAllInstances()
			m.updateSidebarItems()
		}
		return m, tea.Batch(tea.WindowSize(), m.instanceChanged())

	case "zen_mode":
		selected := m.list.GetSelectedInstance()
		if selected == nil || !selected.Started() || selected.Paused() || !selected.TmuxAlive() {
			return m, nil
		}
		return m, func() tea.Msg {
			ch, err := m.list.Attach()
			if err != nil {
				return err
			}
			<-ch
			return instanceChangedMsg{}
		}

	case "pause_instance":
		selected := m.list.GetSelectedInstance()
		if selected != nil && selected.Status != session.Paused {
			// Set Loading immediately to prevent the metadata tick goroutine from
			// overwriting the status while Pause() is running.
			selected.SetStatus(session.Loading)
			selected.LoadingMessage = "Pausing..."
			m.removeAgentFromBrain(selected)
			if err := selected.Pause(); err != nil {
				return m, m.handleError(err)
			}
			m.saveAllInstances()
		}
		return m, tea.Batch(tea.WindowSize(), m.instanceChanged())

	case "resume_instance":
		selected := m.list.GetSelectedInstance()
		if selected != nil && selected.Status == session.Paused {
			selected.SetStatus(session.Loading)
			selected.LoadingMessage = "Resuming..."
			resumeCmd := func() tea.Msg {
				err := selected.Resume()
				return instanceResumedMsg{instance: selected, err: err}
			}
			return m, tea.Batch(tea.WindowSize(), resumeCmd)
		}
		return m, tea.Batch(tea.WindowSize(), m.instanceChanged())

	case "restart_instance":
		selected := m.list.GetSelectedInstance()
		if selected != nil && selected.IsTmuxDead() {
			selected.SetStatus(session.Loading)
			selected.LoadingMessage = "Restarting agent..."
			restartCmd := func() tea.Msg {
				err := selected.Restart()
				return instanceResumedMsg{instance: selected, err: err, wasDead: true}
			}
			return m, tea.Batch(tea.WindowSize(), restartCmd)
		}
		return m, tea.Batch(tea.WindowSize(), m.instanceChanged())

	case "move_instance":
		selected := m.list.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}
		m.state = stateMoveTo
		m.pickerOverlay = overlay.NewPickerOverlay("Move to topic", m.getMovableTopicNames())
		return m, nil

	case "push_instance":
		selected := m.list.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}
		// Trigger the existing push flow
		return m, func() tea.Msg {
			return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
		}

	case "create_pr_instance":
		selected := m.list.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}
		m.state = statePRTitle
		m.textInputOverlay = overlay.NewTextInputOverlay("PR title", selected.Title)
		m.textInputOverlay.SetSize(60, 3)
		return m, nil

	case "focus_instance":
		selected := m.list.GetSelectedInstance()
		if selected == nil || !selected.Started() || selected.Paused() {
			return m, nil
		}
		return m, m.enterFocusMode()

	case "copy_worktree_path":
		selected := m.list.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}
		worktree, err := selected.GetGitWorktree()
		if err != nil {
			return m, m.handleError(err)
		}
		_ = clipboard.WriteAll(worktree.GetWorktreePath())
		return m, nil

	case "copy_branch_name":
		selected := m.list.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}
		_ = clipboard.WriteAll(selected.Branch)
		return m, nil

	case "rename_instance":
		selected := m.list.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}
		m.state = stateRenameInstance
		m.textInputOverlay = overlay.NewTextInputOverlay("Rename instance", selected.Title)
		m.textInputOverlay.SetSize(60, 3)
		return m, nil

	case "rename_topic":
		selectedID := m.sidebar.GetSelectedID()
		if selectedID == ui.SidebarAll || ui.IsUngroupedID(selectedID) {
			return m, nil
		}
		m.state = stateRenameTopic
		m.textInputOverlay = overlay.NewTextInputOverlay("Rename topic", selectedID)
		m.textInputOverlay.SetSize(60, 3)
		return m, nil

	case "push_topic":
		// Push the topic's branch — find first running instance in topic to push via
		selectedID := m.sidebar.GetSelectedID()
		for _, inst := range m.list.GetInstances() {
			if inst.TopicName == selectedID && inst.Started() {
				m.list.SetSelectedInstance(0) // select it
				return m, func() tea.Msg {
					return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
				}
			}
		}
		return m, nil
	}

	return m, nil
}

// openContextMenu builds a context menu for the currently focused/selected item
// (sidebar topic or instance) and positions it next to the selected item.
func (m *home) openContextMenu() (tea.Model, tea.Cmd) {
	if m.focusedPanel == 0 {
		// Sidebar focused — build topic context menu
		selectedID := m.sidebar.GetSelectedID()
		if selectedID == ui.SidebarAll || ui.IsUngroupedID(selectedID) {
			return m, nil
		}
		var topic *session.Topic
		for _, t := range m.topics {
			if t.Name == selectedID {
				topic = t
				break
			}
		}
		if topic == nil {
			return m, nil
		}
		items := []overlay.ContextMenuItem{
			{Label: "Kill all instances", Action: "kill_all_in_topic"},
			{Label: "Delete topic + instances", Action: "delete_topic_and_instances"},
			{Label: "Delete topic (ungroup only)", Action: "delete_topic"},
			{Label: "Rename topic", Action: "rename_topic"},
		}
		if topic.SharedWorktree {
			items = append(items, overlay.ContextMenuItem{Label: "Push branch", Action: "push_topic"})
		}
		// Position next to the selected sidebar item
		x := m.sidebarWidth
		y := 1 + 4 + m.sidebar.GetSelectedIdx() // PaddingTop(1) + search/header rows + item index
		m.contextMenu = overlay.NewContextMenu(x, y, items)
		m.state = stateContextMenu
		return m, nil
	}

	// Instance list focused — build instance context menu
	selected := m.list.GetSelectedInstance()
	if selected == nil {
		return m, nil
	}
	items := []overlay.ContextMenuItem{
		{Label: "Focus", Action: "focus_instance"},
		{Label: "Zen mode", Action: "zen_mode"},
		{Label: "Kill", Action: "kill_instance"},
	}
	if selected.IsTmuxDead() {
		items = append(items, overlay.ContextMenuItem{Label: "Restart agent", Action: "restart_instance"})
	} else if selected.Status == session.Paused {
		items = append(items, overlay.ContextMenuItem{Label: "Resume", Action: "resume_instance"})
	} else {
		items = append(items, overlay.ContextMenuItem{Label: "Pause", Action: "pause_instance"})
	}
	items = append(items, overlay.ContextMenuItem{Label: "Rename", Action: "rename_instance"})
	items = append(items, overlay.ContextMenuItem{Label: "Move to topic", Action: "move_instance"})
	items = append(items, overlay.ContextMenuItem{Label: "Push branch", Action: "push_instance"})
	items = append(items, overlay.ContextMenuItem{Label: "Create PR", Action: "create_pr_instance"})
	items = append(items, overlay.ContextMenuItem{Label: "Copy worktree path", Action: "copy_worktree_path"})
	items = append(items, overlay.ContextMenuItem{Label: "Copy branch name", Action: "copy_branch_name"})
	// Position next to the selected instance
	x := m.sidebarWidth + m.listWidth
	y := 1 + 4 + m.list.GetSelectedIdx()*4 // PaddingTop(1) + header rows + item offset
	m.contextMenu = overlay.NewContextMenu(x, y, items)
	m.state = stateContextMenu
	return m, nil
}
