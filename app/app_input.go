package app

import (
	"fmt"
	"time"

	"github.com/ByteMirror/hivemind/keys"
	"github.com/ByteMirror/hivemind/log"
	"github.com/ByteMirror/hivemind/session"
	"github.com/ByteMirror/hivemind/ui"
	"github.com/ByteMirror/hivemind/ui/overlay"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

func (m *home) handleMenuHighlighting(msg tea.KeyMsg) (cmd tea.Cmd, returnEarly bool) {
	// Handle menu highlighting when you press a button. We intercept it here and immediately return to
	// update the ui while re-sending the keypress. Then, on the next call to this, we actually handle the keypress.
	if m.keySent {
		m.keySent = false
		return nil, false
	}
	if m.state == statePrompt || m.state == stateHelp || m.state == stateConfirm || m.state == stateNewTopic || m.state == stateNewTopicConfirm || m.state == stateSearch || m.state == stateMoveTo || m.state == stateContextMenu || m.state == statePRTitle || m.state == statePRBody || m.state == stateRenameInstance || m.state == stateRenameTopic || m.state == stateSendPrompt || m.state == stateFocusAgent || m.state == stateRepoSwitch || m.state == stateNewTopicRepo {
		return nil, false
	}
	// If it's in the global keymap, we should try to highlight it.
	name, ok := keys.GlobalKeyStringsMap[msg.String()]
	if !ok {
		return nil, false
	}

	if m.list.GetSelectedInstance() != nil && m.list.GetSelectedInstance().Paused() && (name == keys.KeyEnter || name == keys.KeyZenMode) {
		return nil, false
	}
	if name == keys.KeyShiftDown || name == keys.KeyShiftUp || name == keys.KeyShiftLeft || name == keys.KeyShiftRight {
		return nil, false
	}

	// Skip the menu highlighting if the key is not in the map or we are using the shift up and down keys.
	// TODO: cleanup: when you press enter on stateNew, we use keys.KeySubmitName. We should unify the keymap.
	if name == keys.KeyEnter && m.state == stateNew {
		name = keys.KeySubmitName
	}
	m.keySent = true
	return tea.Batch(
		func() tea.Msg { return msg },
		m.keydownCallback(name)), true
}

// handleMouse processes mouse events for click and scroll interactions.
// Focus mode is the default for pane interactions — clicking a tab or pane content
// enters focus mode, clicking outside the pane area exits it.
func (m *home) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	isPaneArea := msg.X >= m.sidebarWidth+m.listWidth

	// In focus mode: pane-area events go to PTY (with tab-click intercept),
	// clicks outside pane area exit focus mode and fall through to normal handling.
	if m.state == stateFocusAgent {
		if isPaneArea {
			// Left-click on tab bar → switch tabs within focus mode
			if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
				localX := msg.X - m.sidebarWidth - m.listWidth
				contentY := msg.Y - 1
				prevTab := m.tabbedWindow.GetActiveTab()
				if m.tabbedWindow.HandleTabClick(localX, contentY) && m.tabbedWindow.GetActiveTab() != prevTab {
					m.exitFocusMode()
					m.menu.SetInDiffTab(m.tabbedWindow.IsInDiffTab())
					return m.enterFocusModeForActiveTab()
				}
			}
			// All other pane-area events → forward to PTY
			return m.handleFocusModeMouseEvent(msg)
		}
		// Click outside pane area → exit focus mode and process normally below
		if msg.Action == tea.MouseActionPress {
			m.exitFocusMode()
			// Fall through to normal handling
		} else {
			return m, nil
		}
	}

	// Track hover state for the repo button using precise bounds
	sidebarScreenTop := 1 // PaddingTop(1) from colStyle
	repoHovered := m.sidebar.IsRepoBtnHit(msg.X, msg.Y, sidebarScreenTop)
	m.sidebar.SetRepoHovered(repoHovered)

	if msg.Action != tea.MouseActionPress {
		return m, nil
	}

	// Handle scroll wheel — always scrolls content (never navigates files)
	if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
		selected := m.list.GetSelectedInstance()
		if selected != nil && selected.Status != session.Paused {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				m.tabbedWindow.ContentScrollUp()
			case tea.MouseButtonWheelDown:
				m.tabbedWindow.ContentScrollDown()
			}
		}
		return m, nil
	}

	// Dismiss overlays on click-outside
	if m.state == stateContextMenu && msg.Button == tea.MouseButtonLeft {
		m.contextMenu = nil
		m.state = stateDefault
		return m, nil
	}
	if m.state == stateRepoSwitch && msg.Button == tea.MouseButtonLeft {
		m.pickerOverlay = nil
		m.state = stateDefault
		return m, nil
	}
	if m.state == stateSearch && msg.Button == tea.MouseButtonLeft {
		// Only keep search active if clicking inside the search bar area
		clickContentY := msg.Y - 1
		if msg.X < m.sidebarWidth && clickContentY >= 0 && clickContentY <= 2 {
			return m, nil
		}
		// Clicked outside search bar — dismiss it
		m.sidebar.DeactivateSearch()
		m.sidebar.UpdateMatchCounts(nil, 0)
		m.state = stateDefault
		m.filterInstancesByTopic()
		return m, nil
	}
	if m.state != stateDefault {
		return m, nil
	}

	x, y := msg.X, msg.Y

	// Account for PaddingTop(1) on columns
	contentY := y - 1

	// Right-click: show context menu
	if msg.Button == tea.MouseButtonRight {
		return m.handleRightClick(x, y, contentY)
	}

	// Only handle left clicks from here
	if msg.Button != tea.MouseButtonLeft {
		return m, nil
	}

	// Repo switch button click using precise bounds
	log.InfoLog.Printf("MOUSE CLICK: x=%d y=%d button=%d state=%d sidebarW=%d listW=%d", x, y, msg.Button, m.state, m.sidebarWidth, m.listWidth)
	if m.sidebar.IsRepoBtnHit(x, y, sidebarScreenTop) {
		log.InfoLog.Printf("REPO BTN HIT intercepted click")
		m.state = stateRepoSwitch
		m.pickerOverlay = overlay.NewPickerOverlay("Switch repo", m.buildRepoPickerItems())
		m.pickerOverlay.SetHint("↑↓ navigate • enter switch • space toggle • esc cancel")
		return m, nil
	}

	// Determine which column was clicked
	if x < m.sidebarWidth {
		// Click in sidebar
		m.setFocus(0)

		// Search bar is at rows 0-2 in the sidebar content (border takes 3 rows)
		if contentY >= 0 && contentY <= 2 {
			m.sidebar.ActivateSearch()
			m.state = stateSearch
			return m, nil
		}

		// Sidebar items start after search bar (row 0) + border (2 rows) + blank line (1 row) = row 4
		itemRow := contentY - 4
		if itemRow >= 0 {
			m.sidebar.ClickItem(itemRow)
			m.filterInstancesByTopic()
			return m, m.instanceChanged()
		}
	} else if x < m.sidebarWidth+m.listWidth {
		// Click in instance list
		m.setFocus(1)

		localX := x - m.sidebarWidth - m.columnGap // account for left gap
		log.InfoLog.Printf("LIST CLICK: x=%d y=%d contentY=%d localX=%d sidebarW=%d listW=%d gap=%d", x, y, contentY, localX, m.sidebarWidth, m.listWidth, m.columnGap)
		// Check if clicking on filter tabs
		if filter, ok := m.list.HandleTabClick(localX, contentY); ok {
			log.InfoLog.Printf("TAB CLICK: filter=%d", filter)
			m.list.SetStatusFilter(filter)
			return m, m.instanceChanged()
		}

		// Instance list items start after blank line + tabs + blank line
		listY := contentY - 3
		if listY >= 0 {
			itemIdx := m.list.GetItemAtRow(listY)
			if itemIdx >= 0 {
				m.list.SetSelectedInstance(itemIdx)
				return m, m.instanceChanged()
			}
		}
	} else {
		// Click in pane area (tabs or content) → switch tab if needed, then enter focus mode
		m.setFocus(1)
		localX := x - m.sidebarWidth - m.listWidth
		wasGitTab := m.tabbedWindow.IsInGitTab()
		wasTerminalTab := m.tabbedWindow.IsInTerminalTab()
		if m.tabbedWindow.HandleTabClick(localX, contentY) {
			m.menu.SetInDiffTab(m.tabbedWindow.IsInDiffTab())
			if wasGitTab && !m.tabbedWindow.IsInGitTab() {
				m.detachGitTab()
			}
			if wasTerminalTab && !m.tabbedWindow.IsInTerminalTab() {
				m.detachTerminalTab()
			}
		}
		// Enter focus mode for the active tab
		return m.enterFocusModeForActiveTab()
	}

	return m, nil
}

// handleRightClick builds and shows a context menu based on what was right-clicked.
func (m *home) handleRightClick(x, y, contentY int) (tea.Model, tea.Cmd) {
	if x < m.sidebarWidth {
		// Right-click in sidebar
		itemRow := contentY - 4
		if itemRow >= 0 {
			m.sidebar.ClickItem(itemRow)
			m.filterInstancesByTopic()
		}
		selectedID := m.sidebar.GetSelectedID()
		if selectedID == ui.SidebarAll || ui.IsUngroupedID(selectedID) {
			// No context menu for All/Ungrouped
			return m, nil
		}
		// Find the topic
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
		m.contextMenu = overlay.NewContextMenu(x, y, items)
		m.state = stateContextMenu
		return m, nil
	} else if x < m.sidebarWidth+m.listWidth {
		// Right-click in instance list — select the item first
		listY := contentY - 3
		if listY >= 0 {
			itemIdx := m.list.GetItemAtRow(listY)
			if itemIdx >= 0 {
				m.list.SetSelectedInstance(itemIdx)
			}
		}
		selected := m.list.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}
		items := []overlay.ContextMenuItem{
			{Label: "Focus", Action: "focus_instance"},
			{Label: "Zen mode", Action: "zen_mode"},
			{Label: "Kill", Action: "kill_instance"},
		}
		if selected.Status == session.Paused {
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
		m.contextMenu = overlay.NewContextMenu(x, y, items)
		m.state = stateContextMenu
		return m, nil
	}
	return m, nil
}

func (m *home) handleKeyPress(msg tea.KeyMsg) (mod tea.Model, cmd tea.Cmd) {
	cmd, returnEarly := m.handleMenuHighlighting(msg)
	if returnEarly {
		return m, cmd
	}

	switch m.state {
	case stateContextMenu:
		return m.handleContextMenuKeys(msg)
	case stateHelp:
		return m.handleHelpKeys(msg)
	case stateNew:
		return m.handleNewInstanceKeys(msg)
	case statePrompt:
		return m.handlePromptKeys(msg)
	case statePRTitle:
		return m.handlePRTitleKeys(msg)
	case statePRBody:
		return m.handlePRBodyKeys(msg)
	case stateRenameInstance:
		return m.handleRenameInstanceKeys(msg)
	case stateRenameTopic:
		return m.handleRenameTopicKeys(msg)
	case stateFocusAgent:
		return m.handleFocusAgentKeys(msg)
	case stateSendPrompt:
		return m.handleSendPromptKeys(msg)
	case stateConfirm:
		return m.handleConfirmKeys(msg)
	case stateNewTopic:
		return m.handleNewTopicKeys(msg)
	case stateNewTopicConfirm:
		return m.handleNewTopicConfirmKeys(msg)
	case stateMoveTo:
		return m.handleMoveToKeys(msg)
	case stateRepoSwitch:
		return m.handleRepoSwitchKeys(msg)
	case stateNewTopicRepo:
		return m.handleNewTopicRepoKeys(msg)
	case stateSearch:
		return m.handleSearchKeys(msg)
	default:
		return m.handleDefaultKeys(msg)
	}
}

func (m *home) handleContextMenuKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.contextMenu == nil {
		m.state = stateDefault
		return m, nil
	}
	action, closed := m.contextMenu.HandleKeyPress(msg)
	if closed {
		m.contextMenu = nil
		m.state = stateDefault
		if action != "" {
			return m.executeContextAction(action)
		}
		return m, nil
	}
	return m, nil
}

func (m *home) handleHelpKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	return m.handleHelpState(msg)
}

func (m *home) handleNewInstanceKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle quit commands first. Don't handle q because the user might want to type that.
	if msg.String() == "ctrl+c" {
		m.state = stateDefault
		m.promptAfterName = false
		m.pendingInstance = nil
		m.list.Kill()
		return m, tea.Sequence(
			tea.WindowSize(),
			func() tea.Msg {
				m.menu.SetState(ui.StateDefault)
				return nil
			},
		)
	}

	instance := m.pendingInstance
	if instance == nil {
		m.state = stateDefault
		return m, nil
	}
	switch msg.Type {
	// Start the instance (enable previews etc) and go back to the main menu state.
	case tea.KeyEnter:
		if len(instance.Title) == 0 {
			return m, m.handleError(fmt.Errorf("title cannot be empty"))
		}

		// Set loading status and transition to default state immediately
		instance.SetStatus(session.Loading)
		m.state = stateDefault
		m.pendingInstance = nil
		m.menu.SetState(ui.StateDefault)

		// Handle prompt-after-name flow
		if m.promptAfterName {
			m.state = statePrompt
			m.menu.SetState(ui.StatePrompt)
			m.textInputOverlay = overlay.NewTextInputOverlay("Enter prompt", "")
			m.textInputOverlay.SetSize(50, 5)
			m.promptAfterName = false
		}

		// Find topic for shared worktree check
		var topic *session.Topic
		for _, t := range m.topics {
			if t.Name == instance.TopicName {
				topic = t
				break
			}
		}

		// Start instance asynchronously
		startCmd := func() tea.Msg {
			var startErr error
			if topic != nil && topic.SharedWorktree && topic.Started() {
				startErr = instance.StartInSharedWorktree(topic.GetGitWorktree(), topic.Branch)
			} else {
				startErr = instance.Start(true)
			}
			return instanceStartedMsg{instance: instance, err: startErr}
		}

		return m, tea.Batch(tea.WindowSize(), startCmd)
	case tea.KeyRunes:
		if runewidth.StringWidth(instance.Title) >= 32 {
			return m, m.handleError(fmt.Errorf("title cannot be longer than 32 characters"))
		}
		if err := instance.SetTitle(instance.Title + string(msg.Runes)); err != nil {
			return m, m.handleError(err)
		}
	case tea.KeyBackspace:
		runes := []rune(instance.Title)
		if len(runes) == 0 {
			return m, nil
		}
		if err := instance.SetTitle(string(runes[:len(runes)-1])); err != nil {
			return m, m.handleError(err)
		}
	case tea.KeySpace:
		if err := instance.SetTitle(instance.Title + " "); err != nil {
			return m, m.handleError(err)
		}
	case tea.KeyEsc:
		m.list.Kill()
		m.state = stateDefault
		m.pendingInstance = nil
		m.instanceChanged()

		return m, tea.Sequence(
			tea.WindowSize(),
			func() tea.Msg {
				m.menu.SetState(ui.StateDefault)
				return nil
			},
		)
	default:
	}
	return m, nil
}

func (m *home) handlePromptKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Use the new TextInputOverlay component to handle all key events
	shouldClose := m.textInputOverlay.HandleKeyPress(msg)

	// Check if the form was submitted or canceled
	if shouldClose {
		selected := m.list.GetSelectedInstance()
		// TODO: this should never happen since we set the instance in the previous state.
		if selected == nil {
			return m, nil
		}
		if m.textInputOverlay.IsSubmitted() {
			if err := selected.SendPrompt(m.textInputOverlay.GetValue()); err != nil {
				// TODO: we probably end up in a bad state here.
				return m, m.handleError(err)
			}
		}

		// Close the overlay and reset state
		m.textInputOverlay = nil
		m.state = stateDefault
		return m, tea.Sequence(
			tea.WindowSize(),
			func() tea.Msg {
				m.menu.SetState(ui.StateDefault)
				m.showHelpScreen(helpStart(selected), nil)
				return nil
			},
		)
	}

	return m, nil
}

func (m *home) handlePRTitleKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.textInputOverlay == nil {
		m.state = stateDefault
		return m, nil
	}
	shouldClose := m.textInputOverlay.HandleKeyPress(msg)
	if shouldClose {
		if m.textInputOverlay.IsSubmitted() {
			prTitle := m.textInputOverlay.GetValue()
			selected := m.list.GetSelectedInstance()
			if selected != nil && prTitle != "" {
				m.pendingPRTitle = prTitle
				m.textInputOverlay = nil

				// Generate a PR body from git data
				generatedBody := ""
				worktree, err := selected.GetGitWorktree()
				if err == nil {
					body, genErr := worktree.GeneratePRBody()
					if genErr == nil {
						generatedBody = body
					}
				}

				// Transition to PR body editing state
				m.state = statePRBody
				m.textInputOverlay = overlay.NewTextInputOverlay("PR description (edit or submit)", generatedBody)
				m.textInputOverlay.SetSize(80, 20)
				return m, nil
			}
		}
		m.textInputOverlay = nil
		m.state = stateDefault
		m.menu.SetState(ui.StateDefault)
		return m, tea.WindowSize()
	}
	return m, nil
}

func (m *home) handlePRBodyKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.textInputOverlay == nil {
		m.state = stateDefault
		return m, nil
	}
	shouldClose := m.textInputOverlay.HandleKeyPress(msg)
	if shouldClose {
		if m.textInputOverlay.IsSubmitted() {
			prBody := m.textInputOverlay.GetValue()
			prTitle := m.pendingPRTitle
			selected := m.list.GetSelectedInstance()
			if selected != nil && prTitle != "" {
				m.textInputOverlay = nil
				m.pendingPRTitle = ""
				m.state = stateDefault
				m.menu.SetState(ui.StateDefault)
				m.pendingPRToastID = m.toastManager.Loading("Creating PR...")
				prToastID := m.pendingPRToastID
				return m, tea.Batch(tea.WindowSize(), func() tea.Msg {
					commitMsg := fmt.Sprintf("[hivemind] update from '%s' on %s", selected.Title, time.Now().Format(time.RFC822))
					worktree, err := selected.GetGitWorktree()
					if err != nil {
						return prErrorMsg{id: prToastID, err: err}
					}
					if err := worktree.CreatePR(prTitle, prBody, commitMsg); err != nil {
						return prErrorMsg{id: prToastID, err: err}
					}
					return prCreatedMsg{}
				}, m.toastTickCmd())
			}
		}
		m.textInputOverlay = nil
		m.pendingPRTitle = ""
		m.state = stateDefault
		m.menu.SetState(ui.StateDefault)
		return m, tea.WindowSize()
	}
	return m, nil
}

func (m *home) handleRenameInstanceKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.textInputOverlay == nil {
		m.state = stateDefault
		return m, nil
	}
	shouldClose := m.textInputOverlay.HandleKeyPress(msg)
	if shouldClose {
		if m.textInputOverlay.IsSubmitted() {
			newName := m.textInputOverlay.GetValue()
			selected := m.list.GetSelectedInstance()
			if selected != nil && newName != "" {
				selected.Title = newName
				m.saveAllInstances()
			}
		}
		m.textInputOverlay = nil
		m.state = stateDefault
		m.menu.SetState(ui.StateDefault)
		return m, tea.WindowSize()
	}
	return m, nil
}

func (m *home) handleRenameTopicKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.textInputOverlay == nil {
		m.state = stateDefault
		return m, nil
	}
	shouldClose := m.textInputOverlay.HandleKeyPress(msg)
	if shouldClose {
		if m.textInputOverlay.IsSubmitted() {
			newName := m.textInputOverlay.GetValue()
			oldName := m.sidebar.GetSelectedID()
			if newName != "" && newName != oldName {
				// Rename the topic
				for _, t := range m.topics {
					if t.Name == oldName {
						t.Name = newName
						break
					}
				}
				// Update all instances that reference this topic (across all repos)
				for _, inst := range m.allInstances {
					if inst.TopicName == oldName {
						inst.TopicName = newName
					}
				}
				m.updateSidebarItems()
				m.saveAllInstances()
				m.saveAllTopics()
			}
		}
		m.textInputOverlay = nil
		m.state = stateDefault
		m.menu.SetState(ui.StateDefault)
		return m, tea.WindowSize()
	}
	return m, nil
}

func (m *home) handleFocusAgentKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Ctrl+O exits focus mode
	if msg.Type == tea.KeyCtrlO {
		m.exitFocusMode()
		return m, tea.WindowSize()
	}

	// Shift+Left/Right: navigate across all panels (sidebar/instances/tabs)
	if msg.String() == "shift+left" || msg.String() == "shift+right" {
		direction := 1
		if msg.String() == "shift+left" {
			direction = -1
		}
		mod, cmd := m.navigateWithShift(direction)
		return mod, tea.Batch(tea.WindowSize(), cmd)
	}

	// Shift+Up/Down: cycle instances while staying focused on the current tab
	if msg.String() == "shift+up" || msg.String() == "shift+down" {
		direction := 1
		if msg.String() == "shift+up" {
			direction = -1
		}
		return m.cycleInstanceInPlace(direction)
	}

	// Diff tab focus: handle file navigation and scrolling locally
	if m.tabbedWindow.IsInDiffTab() {
		switch msg.String() {
		case "up", "k":
			m.tabbedWindow.GetDiffPane().FileUp()
		case "down", "j":
			m.tabbedWindow.GetDiffPane().FileDown()
		case "J":
			m.tabbedWindow.GetDiffPane().ScrollDown()
		case "K":
			m.tabbedWindow.GetDiffPane().ScrollUp()
		case "enter":
			filePath := m.tabbedWindow.GetDiffPane().GetSelectedFilePath()
			if filePath == "" {
				return m, nil
			}
			return m.openFileInTerminal(filePath)
		case "q", "esc":
			m.exitFocusMode()
			return m, tea.WindowSize()
		}
		return m, nil
	}

	// Git tab focus: forward to lazygit
	if m.tabbedWindow.IsInGitTab() {
		gitPane := m.tabbedWindow.GetGitPane()
		if gitPane == nil || !gitPane.IsRunning() {
			m.exitFocusMode()
			return m, nil
		}
		data := keyToBytes(msg)
		if data == nil {
			return m, nil
		}
		if err := gitPane.SendKey(data); err != nil {
			m.exitFocusMode()
			return m, m.handleError(err)
		}
		return m, nil
	}

	// Terminal tab focus: forward to terminal pane
	if m.tabbedWindow.IsInTerminalTab() {
		termPane := m.tabbedWindow.GetTerminalPane()
		if termPane == nil || !termPane.IsAttached() {
			m.exitFocusMode()
			return m, nil
		}
		data := keyToBytes(msg)
		if data == nil {
			return m, nil
		}
		if err := termPane.SendKey(data); err != nil {
			m.exitFocusMode()
			return m, m.handleError(err)
		}
		return m, nil
	}

	// Preview tab focus: forward to embedded terminal
	if m.embeddedTerminal == nil {
		m.exitFocusMode()
		return m, nil
	}
	data := keyToBytes(msg)
	if data == nil {
		return m, nil
	}
	if err := m.embeddedTerminal.SendKey(data); err != nil {
		return m, m.handleError(err)
	}
	return m, nil
}

func (m *home) handleSendPromptKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.textInputOverlay == nil {
		m.state = stateDefault
		return m, nil
	}
	shouldClose := m.textInputOverlay.HandleKeyPress(msg)
	if shouldClose {
		if m.textInputOverlay.IsSubmitted() {
			value := m.textInputOverlay.GetValue()
			selected := m.list.GetSelectedInstance()
			if selected != nil && value != "" {
				if err := selected.SendPrompt(value); err != nil {
					m.textInputOverlay = nil
					m.state = stateDefault
					m.menu.SetState(ui.StateDefault)
					return m, m.handleError(err)
				}
				selected.SetStatus(session.Running)
			}
		}
		m.textInputOverlay = nil
		m.state = stateDefault
		m.menu.SetState(ui.StateDefault)
		return m, tea.WindowSize()
	}
	return m, nil
}

func (m *home) handleConfirmKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	shouldClose := m.confirmationOverlay.HandleKeyPress(msg)
	if shouldClose {
		m.state = stateDefault
		m.confirmationOverlay = nil
		return m, nil
	}
	return m, nil
}

func (m *home) handleNewTopicKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	shouldClose := m.textInputOverlay.HandleKeyPress(msg)
	if shouldClose {
		if m.textInputOverlay.IsSubmitted() {
			m.pendingTopicName = m.textInputOverlay.GetValue()
			if m.pendingTopicName == "" {
				m.state = stateDefault
				m.menu.SetState(ui.StateDefault)
				m.textInputOverlay = nil
				return m, m.handleError(fmt.Errorf("topic name cannot be empty"))
			}
			// Show shared worktree confirmation
			m.textInputOverlay = nil
			m.confirmationOverlay = overlay.NewConfirmationOverlay(
				fmt.Sprintf("Create shared worktree for topic '%s'?\nAll instances will share one branch and directory.", m.pendingTopicName),
			)
			m.confirmationOverlay.SetWidth(60)
			m.state = stateNewTopicConfirm
			return m, nil
		}
		// Cancelled
		m.state = stateDefault
		m.menu.SetState(ui.StateDefault)
		m.pendingTopicName = ""
		m.textInputOverlay = nil
		return m, tea.WindowSize()
	}
	return m, nil
}

func (m *home) handleNewTopicConfirmKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.confirmationOverlay == nil {
		m.state = stateDefault
		return m, nil
	}
	shouldClose := m.confirmationOverlay.HandleKeyPress(msg)
	if !shouldClose {
		return m, nil // No decision yet
	}

	// Determine if confirmed (y) or cancelled (n/esc) based on which key was pressed
	shared := msg.String() == m.confirmationOverlay.ConfirmKey
	topicRepoPath := m.pendingTopicRepoPath
	if topicRepoPath == "" {
		topicRepoPath = m.activeRepoPaths[0]
	}
	topic := session.NewTopic(session.TopicOptions{
		Name:           m.pendingTopicName,
		SharedWorktree: shared,
		Path:           topicRepoPath,
	})
	if err := topic.Setup(); err != nil {
		m.pendingTopicName = ""
		m.confirmationOverlay = nil
		m.state = stateDefault
		m.menu.SetState(ui.StateDefault)
		return m, m.handleError(err)
	}
	m.allTopics = append(m.allTopics, topic)
	m.topics = append(m.topics, topic)
	m.updateSidebarItems()
	if err := m.saveAllTopics(); err != nil {
		return m, m.handleError(err)
	}
	m.pendingTopicName = ""
	m.pendingTopicRepoPath = ""
	m.confirmationOverlay = nil
	m.state = stateDefault
	m.menu.SetState(ui.StateDefault)
	return m, tea.WindowSize()
}

func (m *home) handleMoveToKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	shouldClose := m.pickerOverlay.HandleKeyPress(msg)
	if shouldClose {
		selected := m.list.GetSelectedInstance()
		if selected != nil && m.pickerOverlay.IsSubmitted() {
			picked := m.pickerOverlay.Value()
			if picked == "(Ungrouped)" {
				selected.TopicName = ""
			} else {
				selected.TopicName = picked
			}
			m.updateSidebarItems()
			if err := m.saveAllInstances(); err != nil {
				m.state = stateDefault
				m.menu.SetState(ui.StateDefault)
				m.pickerOverlay = nil
				return m, m.handleError(err)
			}
		}
		m.state = stateDefault
		m.menu.SetState(ui.StateDefault)
		m.pickerOverlay = nil
		return m, tea.WindowSize()
	}
	return m, nil
}

func (m *home) handleRepoSwitchKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	shouldClose := m.pickerOverlay.HandleKeyPress(msg)
	if shouldClose {
		selected := m.pickerOverlay.Value()
		if selected != "" {
			if selected == "Open folder..." {
				m.state = stateDefault
				m.menu.SetState(ui.StateDefault)
				m.pickerOverlay = nil
				return m, m.openFolderPicker()
			}
			if m.pickerOverlay.IsSubmitted() {
				// Enter = switch exclusively to this repo
				m.switchToRepo(selected)
			} else if m.pickerOverlay.IsToggled() {
				// Space = toggle repo in/out of multi-repo view
				m.toggleRepo(selected)
			}
		}
		m.state = stateDefault
		m.menu.SetState(ui.StateDefault)
		m.pickerOverlay = nil
	}
	return m, nil
}

// handleNewTopicRepoKeys handles the repo picker for multi-repo topic creation.
func (m *home) handleNewTopicRepoKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	shouldClose := m.pickerOverlay.HandleKeyPress(msg)
	if shouldClose {
		if m.pickerOverlay.IsSubmitted() {
			selected := m.pickerOverlay.Value()
			rp, ok := m.repoPickerMap[selected]
			if ok && rp != "" {
				m.pendingTopicRepoPath = rp
				m.pickerOverlay = nil
				// Transition to topic name entry
				m.state = stateNewTopic
				m.textInputOverlay = overlay.NewTextInputOverlay("Topic name", "")
				m.textInputOverlay.SetSize(50, 3)
				return m, nil
			}
		}
		// Cancelled
		m.pendingTopicRepoPath = ""
		m.pickerOverlay = nil
		m.state = stateDefault
		m.menu.SetState(ui.StateDefault)
	}
	return m, nil
}

func (m *home) handleSearchKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.String() == "esc":
		m.sidebar.DeactivateSearch()
		m.sidebar.UpdateMatchCounts(nil, 0)
		m.state = stateDefault
		m.filterInstancesByTopic()
		return m, nil
	case msg.String() == "enter":
		m.sidebar.DeactivateSearch()
		m.sidebar.UpdateMatchCounts(nil, 0)
		m.state = stateDefault
		return m, nil
	case msg.String() == "up":
		m.sidebar.Up()
		m.filterSearchWithTopic()
		return m, m.instanceChanged()
	case msg.String() == "down":
		m.sidebar.Down()
		m.filterSearchWithTopic()
		return m, m.instanceChanged()
	case msg.Type == tea.KeyBackspace:
		q := m.sidebar.GetSearchQuery()
		if len(q) > 0 {
			runes := []rune(q)
			m.sidebar.SetSearchQuery(string(runes[:len(runes)-1]))
		}
		m.filterBySearch()
		return m, nil
	case msg.Type == tea.KeySpace:
		m.sidebar.SetSearchQuery(m.sidebar.GetSearchQuery() + " ")
		m.filterBySearch()
		return m, nil
	case msg.Type == tea.KeyRunes:
		m.sidebar.SetSearchQuery(m.sidebar.GetSearchQuery() + string(msg.Runes))
		m.filterBySearch()
		return m, nil
	}
	return m, nil
}

func (m *home) handleDefaultKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Exit scrolling mode when ESC is pressed and preview pane is in scrolling mode
	// Check if Escape key was pressed and we're not in the diff tab (meaning we're in preview tab)
	// Always check for escape key first to ensure it doesn't get intercepted elsewhere
	if msg.Type == tea.KeyEsc {
		// If in preview tab and in scroll mode, exit scroll mode
		if !m.tabbedWindow.IsInDiffTab() && m.tabbedWindow.IsPreviewInScrollMode() {
			// Use the selected instance from the list
			selected := m.list.GetSelectedInstance()
			err := m.tabbedWindow.ResetPreviewToNormalMode(selected)
			if err != nil {
				return m, m.handleError(err)
			}
			return m, m.instanceChanged()
		}
	}

	// Handle quit commands first
	if msg.String() == "ctrl+c" || msg.String() == "q" {
		return m.handleQuit()
	}

	name, ok := keys.GlobalKeyStringsMap[msg.String()]
	if !ok {
		return m, nil
	}

	switch name {
	case keys.KeyHelp:
		return m.showHelpScreen(helpTypeGeneral{}, nil)
	case keys.KeyPrompt:
		if _, errCmd := m.createNewInstance(false); errCmd != nil {
			return m, errCmd
		}
		m.promptAfterName = true
		return m, nil
	case keys.KeyNew:
		if _, errCmd := m.createNewInstance(false); errCmd != nil {
			return m, errCmd
		}
		return m, nil
	case keys.KeyNewSkipPermissions:
		if _, errCmd := m.createNewInstance(true); errCmd != nil {
			return m, errCmd
		}
		return m, nil
	case keys.KeyUp:
		if m.focusedPanel == 0 {
			m.sidebar.Up()
			m.filterInstancesByTopic()
		} else {
			m.list.Up()
		}
		return m, m.instanceChanged()
	case keys.KeyDown:
		if m.focusedPanel == 0 {
			m.sidebar.Down()
			m.filterInstancesByTopic()
		} else {
			m.list.Down()
		}
		return m, m.instanceChanged()
	case keys.KeyShiftUp:
		if m.navPosition == 0 {
			before := m.sidebar.GetSelectedIdx()
			m.sidebar.Up()
			if m.sidebar.GetSelectedIdx() == before {
				m.sidebar.SelectLast()
			}
			m.filterInstancesByTopic()
			return m, m.instanceChanged()
		}
		if m.navPosition >= 2 {
			return m.cycleInstanceInPlace(-1)
		}
		return m.cycleInstanceInPlace(-1)
	case keys.KeyShiftDown:
		if m.navPosition == 0 {
			before := m.sidebar.GetSelectedIdx()
			m.sidebar.Down()
			if m.sidebar.GetSelectedIdx() == before {
				m.sidebar.SelectFirst()
			}
			m.filterInstancesByTopic()
			return m, m.instanceChanged()
		}
		if m.navPosition >= 2 {
			return m.cycleInstanceInPlace(1)
		}
		return m.cycleInstanceInPlace(1)
	case keys.KeyTab:
		wasGitTab := m.tabbedWindow.IsInGitTab()
		wasTerminalTab := m.tabbedWindow.IsInTerminalTab()
		m.tabbedWindow.Toggle()
		m.menu.SetInDiffTab(m.tabbedWindow.IsInDiffTab())
		// Detach lazygit when leaving git tab
		if wasGitTab && !m.tabbedWindow.IsInGitTab() {
			m.detachGitTab()
		}
		// Detach terminal when leaving terminal tab
		if wasTerminalTab && !m.tabbedWindow.IsInTerminalTab() {
			m.detachTerminalTab()
		}
		// Attach lazygit when entering git tab
		if m.tabbedWindow.IsInGitTab() {
			cmd := m.attachGitTab()
			return m, tea.Batch(m.instanceChanged(), cmd)
		}
		// Attach terminal when entering terminal tab
		if m.tabbedWindow.IsInTerminalTab() {
			cmd := m.spawnTerminalTab()
			return m, tea.Batch(m.instanceChanged(), cmd)
		}
		return m, m.instanceChanged()
	case keys.KeyFilterAll:
		m.list.SetStatusFilter(ui.StatusFilterAll)
		return m, m.instanceChanged()
	case keys.KeyFilterActive:
		m.list.SetStatusFilter(ui.StatusFilterActive)
		return m, m.instanceChanged()
	case keys.KeyCycleSort:
		m.list.CycleSortMode()
		return m, m.instanceChanged()
	case keys.KeyExpandCollapse:
		m.list.ToggleExpanded()
		return m, nil
	case keys.KeySpace:
		return m.openContextMenu()
	case keys.KeyTerminalTab:
		// Jump directly to terminal tab
		if m.tabbedWindow.IsInTerminalTab() {
			return m, nil
		}
		wasGitTab := m.tabbedWindow.IsInGitTab()
		m.tabbedWindow.SetActiveTab(ui.TerminalTab)
		m.menu.SetInDiffTab(false)
		if wasGitTab {
			m.detachGitTab()
		}
		cmd := m.spawnTerminalTab()
		return m, tea.Batch(m.instanceChanged(), cmd)
	case keys.KeyGitTab:
		// Jump directly to git tab
		if m.tabbedWindow.IsInGitTab() {
			return m, nil
		}
		wasTerminalTab := m.tabbedWindow.IsInTerminalTab()
		m.tabbedWindow.SetActiveTab(ui.GitTab)
		m.menu.SetInDiffTab(false)
		if wasTerminalTab {
			m.detachTerminalTab()
		}
		cmd := m.attachGitTab()
		return m, tea.Batch(m.instanceChanged(), cmd)
	case keys.KeyShiftLeft:
		return m.navigateWithShift(-1)
	case keys.KeyShiftRight:
		return m.navigateWithShift(1)
	case keys.KeySendPrompt:
		if m.tabbedWindow.IsInGitTab() {
			return m, m.enterGitFocusMode()
		}
		if m.tabbedWindow.IsInTerminalTab() {
			return m, m.enterTerminalFocusMode()
		}
		if m.tabbedWindow.IsInDiffTab() {
			return m, m.enterDiffFocusMode()
		}
		selected := m.list.GetSelectedInstance()
		if selected == nil || !selected.Started() || selected.Paused() {
			return m, nil
		}
		return m, m.enterFocusMode()
	case keys.KeyKill:
		selected := m.list.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}

		// Create the kill action as a tea.Cmd
		title := selected.Title
		killAction := func() tea.Msg {
			// If instance was never started, just remove it from the list
			if !selected.Started() {
				m.list.Kill()
				m.removeFromAllInstances(title)
				m.saveAllInstances()
				return instanceChangedMsg{}
			}

			// Get worktree and check if branch is checked out
			worktree, err := selected.GetGitWorktree()
			if err != nil {
				return err
			}

			checkedOut, err := worktree.IsBranchCheckedOut()
			if err != nil {
				return err
			}

			if checkedOut {
				return fmt.Errorf("instance %s is currently checked out", selected.Title)
			}

			// Clean up brain entry, kill the instance, remove from master list, and persist
			m.removeAgentFromBrain(selected)
			m.tabbedWindow.GetTerminalPane().KillSession(title)
			m.tabbedWindow.GetGitPane().KillSession(title)
			m.list.Kill()
			m.removeFromAllInstances(title)
			m.saveAllInstances()
			return instanceChangedMsg{}
		}

		// Show confirmation modal
		message := fmt.Sprintf("[!] Kill session '%s'?", selected.Title)
		if title == "" {
			message = "[!] Remove unnamed instance?"
		}
		return m, m.confirmAction(message, killAction)
	case keys.KeySubmit:
		selected := m.list.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}

		// Create the push action as a tea.Cmd
		pushAction := func() tea.Msg {
			// Default commit message with timestamp
			commitMsg := fmt.Sprintf("[hivemind] update from '%s' on %s", selected.Title, time.Now().Format(time.RFC822))
			worktree, err := selected.GetGitWorktree()
			if err != nil {
				return err
			}
			if err = worktree.PushChanges(commitMsg, true); err != nil {
				return err
			}
			return nil
		}

		// Show confirmation modal
		message := fmt.Sprintf("[!] Push changes from session '%s'?", selected.Title)
		return m, m.confirmAction(message, pushAction)
	case keys.KeyCreatePR:
		selected := m.list.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}
		m.state = statePRTitle
		m.textInputOverlay = overlay.NewTextInputOverlay("PR title", selected.Title)
		m.textInputOverlay.SetSize(60, 3)
		return m, nil
	case keys.KeyCheckout:
		selected := m.list.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}

		// Show help screen before pausing
		m.showHelpScreen(helpTypeInstanceCheckout{}, func() {
			// Set Loading immediately to prevent the metadata tick goroutine from
			// overwriting the status while Pause() is running.
			selected.SetStatus(session.Loading)
			selected.LoadingMessage = "Pausing..."
			m.removeAgentFromBrain(selected)
			if err := selected.Pause(); err != nil {
				m.handleError(err)
			}
			m.instanceChanged()
		})
		return m, nil
	case keys.KeyResume:
		selected := m.list.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}
		if selected.IsTmuxDead() {
			// Restart a dead instance (agent process exited)
			selected.SetStatus(session.Loading)
			selected.LoadingMessage = "Restarting agent..."
			restartCmd := func() tea.Msg {
				err := selected.Restart()
				return instanceResumedMsg{instance: selected, err: err, wasDead: true}
			}
			return m, tea.Batch(tea.WindowSize(), restartCmd)
		}
		if selected.Status != session.Paused {
			return m, nil
		}
		selected.SetStatus(session.Loading)
		selected.LoadingMessage = "Resuming..."
		resumeCmd := func() tea.Msg {
			err := selected.Resume()
			return instanceResumedMsg{instance: selected, err: err}
		}
		return m, tea.Batch(tea.WindowSize(), resumeCmd)
	case keys.KeyEnter:
		// Sidebar focused: move to instance list for the selected topic
		if m.focusedPanel == 0 {
			m.filterInstancesByTopic()
			m.setFocus(1)
			return m, m.instanceChanged()
		}
		// Instance list: enter focus mode for the active tab
		if m.tabbedWindow.IsInGitTab() {
			return m, m.enterGitFocusMode()
		}
		if m.tabbedWindow.IsInTerminalTab() {
			return m, m.enterTerminalFocusMode()
		}
		if m.tabbedWindow.IsInDiffTab() {
			return m, m.enterDiffFocusMode()
		}
		selected := m.list.GetSelectedInstance()
		if selected == nil || !selected.Started() || selected.Paused() {
			return m, nil
		}
		return m, m.enterFocusMode()
	case keys.KeyZenMode:
		if m.list.NumInstances() == 0 {
			return m, nil
		}
		selected := m.list.GetSelectedInstance()
		if selected == nil || selected.Paused() || !selected.TmuxAlive() {
			return m, nil
		}
		m.showHelpScreen(helpTypeZenMode{}, func() {
			ch, err := m.list.Attach()
			if err != nil {
				m.handleError(err)
				return
			}
			<-ch
			m.state = stateDefault
		})
		return m, nil
	case keys.KeyLeft:
		m.setFocus(0)
		return m, nil
	case keys.KeyRight:
		if m.focusedPanel == 1 {
			// Already on instance list → enter focus mode on the active tab's pane
			if m.tabbedWindow.IsInGitTab() {
				return m, m.enterGitFocusMode()
			}
			if m.tabbedWindow.IsInTerminalTab() {
				return m, m.enterTerminalFocusMode()
			}
			if m.tabbedWindow.IsInDiffTab() {
				return m, m.enterDiffFocusMode()
			}
			selected := m.list.GetSelectedInstance()
			if selected != nil && selected.Started() && !selected.Paused() {
				return m, m.enterFocusMode()
			}
		}
		m.setFocus(1)
		return m, nil
	case keys.KeyNewTopic:
		if m.isMultiRepoView() {
			// Multi-repo: pick repo first
			m.state = stateNewTopicRepo
			m.pickerOverlay = overlay.NewPickerOverlay("Select repo for topic", m.buildRepoPickerItems())
			return m, nil
		}
		m.state = stateNewTopic
		m.textInputOverlay = overlay.NewTextInputOverlay("Topic name", "")
		m.textInputOverlay.SetSize(50, 3)
		return m, nil
	case keys.KeyMoveTo:
		selected := m.list.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}
		// Can't move shared-worktree instances (they're tied to their topic's worktree)
		if selected.TopicName != "" {
			for _, t := range m.topics {
				if t.Name == selected.TopicName && t.SharedWorktree {
					return m, m.handleError(fmt.Errorf("cannot move instances in shared-worktree topics"))
				}
			}
		}
		m.state = stateMoveTo
		m.pickerOverlay = overlay.NewPickerOverlay("Move to topic", m.getMovableTopicNames())
		return m, nil
	case keys.KeyKillAllInTopic:
		selectedID := m.sidebar.GetSelectedID()
		if selectedID == ui.SidebarAll || ui.IsUngroupedID(selectedID) {
			return m, m.handleError(fmt.Errorf("select a topic first"))
		}
		killAction := func() tea.Msg {
			// Remove from allInstances before killing
			for i := len(m.allInstances) - 1; i >= 0; i-- {
				if m.allInstances[i].TopicName == selectedID {
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
	case keys.KeyRepoSwitch:
		m.state = stateRepoSwitch
		m.pickerOverlay = overlay.NewPickerOverlay("Switch repo", m.buildRepoPickerItems())
		m.pickerOverlay.SetHint("↑↓ navigate • enter switch • space toggle • esc cancel")
		return m, nil
	case keys.KeyAutoYes:
		if m.focusedPanel == 1 {
			// Toggle auto-accept on the selected instance
			selected := m.list.GetSelectedInstance()
			if selected == nil {
				return m, nil
			}
			selected.AutoYes = !selected.AutoYes
			m.saveAllInstances()
			state := "OFF"
			if selected.AutoYes {
				state = "ON"
			}
			m.toastManager.Info(fmt.Sprintf("Auto-accept %s for %s", state, selected.Title))
			return m, m.toastTickCmd()
		}
		// Sidebar focused: toggle auto-accept on the topic
		selectedID := m.sidebar.GetSelectedID()
		if selectedID == ui.SidebarAll || ui.IsUngroupedID(selectedID) {
			return m, m.handleError(fmt.Errorf("select a topic first"))
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
		topic.AutoYes = !topic.AutoYes
		// Cascade to all instances in this topic
		for _, inst := range m.allInstances {
			if inst.TopicName == topic.Name {
				inst.AutoYes = topic.AutoYes
			}
		}
		m.saveAllInstances()
		m.saveAllTopics()
		m.updateSidebarItems()
		state := "OFF"
		if topic.AutoYes {
			state = "ON"
		}
		m.toastManager.Info(fmt.Sprintf("Auto-accept %s for topic %s", state, topic.Name))
		return m, m.toastTickCmd()
	case keys.KeySearch:
		m.sidebar.ActivateSearch()
		m.sidebar.SelectFirst() // Reset to "All" when starting search
		m.state = stateSearch
		m.setFocus(0)
		m.list.SetFilter("") // Show all instances
		return m, nil
	default:
		return m, nil
	}
}

// handleFocusModeMouseEvent forwards mouse events to the focused pane's PTY
// by encoding them as SGR mouse escape sequences with coordinates relative
// to the content area.
func (m *home) handleFocusModeMouseEvent(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Compute content area origin relative to the terminal window.
	// Layout: PaddingTop(1) → tab bar → content (no top border on window).
	tabHeight := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), true).
		GetVerticalFrameSize() + 1
	contentOriginX := m.sidebarWidth + m.listWidth + 1 + 1 // +1 for left window border, +1 for left window padding
	contentOriginY := 1 + tabHeight                    // PaddingTop + tab bar

	// Translate to pane-relative coordinates (1-based for SGR).
	relX := msg.X - contentOriginX + 1
	relY := msg.Y - contentOriginY + 1

	contentW, contentH := m.tabbedWindow.GetPreviewSize()
	if relX < 1 || relY < 1 || relX > contentW || relY > contentH {
		return m, nil // outside content area
	}

	data := mouseToSGR(msg, relX, relY)
	if data == nil {
		return m, nil
	}

	// Forward to the appropriate pane.
	if m.tabbedWindow.IsInGitTab() {
		gitPane := m.tabbedWindow.GetGitPane()
		if gitPane != nil && gitPane.IsRunning() {
			gitPane.SendKey(data)
		}
	} else if m.tabbedWindow.IsInTerminalTab() {
		termPane := m.tabbedWindow.GetTerminalPane()
		if termPane != nil && termPane.IsAttached() {
			termPane.SendKey(data)
		}
	} else if m.embeddedTerminal != nil {
		m.embeddedTerminal.SendKey(data)
	}
	return m, nil
}

// mouseToSGR encodes a Bubble Tea mouse event as an SGR mouse escape sequence.
// Format: \x1b[<Cb;Cx;Cy{M|m} where M=press, m=release.
func mouseToSGR(msg tea.MouseMsg, x, y int) []byte {
	var button int
	switch msg.Button {
	case tea.MouseButtonLeft:
		button = 0
	case tea.MouseButtonMiddle:
		button = 1
	case tea.MouseButtonRight:
		button = 2
	case tea.MouseButtonWheelUp:
		button = 64
	case tea.MouseButtonWheelDown:
		button = 65
	case tea.MouseButtonWheelLeft:
		button = 66
	case tea.MouseButtonWheelRight:
		button = 67
	case tea.MouseButtonNone:
		button = 3
	default:
		return nil
	}

	if msg.Action == tea.MouseActionMotion {
		button += 32
	}

	suffix := byte('M') // press
	if msg.Action == tea.MouseActionRelease {
		suffix = byte('m')
	}

	return []byte(fmt.Sprintf("\x1b[<%d;%d;%d%c", button, x, y, suffix))
}

// keyToBytes translates a Bubble Tea key message to raw bytes for PTY forwarding.
func keyToBytes(msg tea.KeyMsg) []byte {
	switch msg.Type {
	case tea.KeyRunes:
		return []byte(string(msg.Runes))
	case tea.KeyEnter:
		return []byte{0x0D}
	case tea.KeyBackspace:
		return []byte{0x7F}
	case tea.KeyTab:
		return []byte{0x09}
	case tea.KeySpace:
		return []byte{0x20}
	case tea.KeyUp:
		return []byte("\x1b[A")
	case tea.KeyDown:
		return []byte("\x1b[B")
	case tea.KeyRight:
		return []byte("\x1b[C")
	case tea.KeyLeft:
		return []byte("\x1b[D")
	case tea.KeyCtrlC:
		return []byte{0x03}
	case tea.KeyCtrlD:
		return []byte{0x04}
	case tea.KeyCtrlA:
		return []byte{0x01}
	case tea.KeyCtrlE:
		return []byte{0x05}
	case tea.KeyCtrlL:
		return []byte{0x0C}
	case tea.KeyCtrlU:
		return []byte{0x15}
	case tea.KeyCtrlK:
		return []byte{0x0B}
	case tea.KeyCtrlW:
		return []byte{0x17}
	case tea.KeyDelete:
		return []byte("\x1b[3~")
	case tea.KeyEsc:
		return []byte{0x1b}
	case tea.KeyShiftTab:
		return []byte("\x1b[Z")
	default:
		return nil
	}
}

// createNewInstance handles the shared logic for creating a new instance.
// skipPermissions controls whether the instance skips permission prompts.
// Returns the new instance or an error command if creation fails.
func (m *home) createNewInstance(skipPermissions bool) (*session.Instance, tea.Cmd) {
	if m.list.TotalInstances() >= GlobalInstanceLimit {
		return nil, m.handleError(
			fmt.Errorf("you can't create more than %d instances", GlobalInstanceLimit))
	}

	topicName := m.selectedTopicNameForNewInstance()
	repoPath := m.repoPathForNewInstance()

	instance, err := session.NewInstance(session.InstanceOptions{
		Title:           "",
		Path:            repoPath,
		Program:         m.program,
		SkipPermissions: skipPermissions,
		TopicName:       topicName,
	})
	if err != nil {
		return nil, m.handleError(err)
	}

	m.inheritAutoYesFromTopic(instance, topicName)

	m.newInstanceFinalizer = m.list.AddInstance(instance)
	m.list.SelectInstanceByRef(instance)
	m.pendingInstance = instance
	m.state = stateNew
	m.menu.SetState(ui.StateNewInstance)

	return instance, nil
}

// selectedTopicNameForNewInstance returns the topic name to assign to a new instance
// based on the current sidebar selection. Returns empty string for "All" or "Ungrouped".
func (m *home) selectedTopicNameForNewInstance() string {
	selectedID := m.sidebar.GetSelectedID()
	if selectedID == ui.SidebarAll || ui.IsUngroupedID(selectedID) {
		return ""
	}
	return selectedID
}

// inheritAutoYesFromTopic sets AutoYes on the instance if the topic has it enabled.
func (m *home) inheritAutoYesFromTopic(instance *session.Instance, topicName string) {
	for _, t := range m.topics {
		if t.Name == topicName && t.AutoYes {
			instance.AutoYes = true
			return
		}
	}
}

// repoPathForNewInstance determines the repo path for a new instance.
// Uses the sidebar's selected repo if available, otherwise falls back to the primary repo.
func (m *home) repoPathForNewInstance() string {
	selectedRepoPath := m.sidebar.GetSelectedRepoPath()
	if selectedRepoPath != "" {
		return selectedRepoPath
	}
	return m.primaryRepoPath
}

func (m *home) handleError(err error) tea.Cmd {
	log.ErrorLog.Printf("%v", err)
	m.toastManager.Error(err.Error())
	return m.toastTickCmd()
}

// confirmAction shows a confirmation modal and stores the action to execute on confirm
func (m *home) confirmAction(message string, action tea.Cmd) tea.Cmd {
	m.state = stateConfirm

	// Create and show the confirmation overlay using ConfirmationOverlay
	m.confirmationOverlay = overlay.NewConfirmationOverlay(message)
	// Set a fixed width for consistent appearance
	m.confirmationOverlay.SetWidth(50)

	// Set callbacks for confirmation and cancellation
	m.confirmationOverlay.OnConfirm = func() {
		m.state = stateDefault
		// Execute the action and handle any errors
		if action != nil {
			if result := action(); result != nil {
				if err, ok := result.(error); ok {
					m.handleError(err)
				}
			}
		}
		m.updateSidebarItems()
	}

	m.confirmationOverlay.OnCancel = func() {
		m.state = stateDefault
	}

	return nil
}

// keydownCallback clears the menu option highlighting after 500ms.
func (m *home) keydownCallback(name keys.KeyName) tea.Cmd {
	m.menu.Keydown(name)
	return func() tea.Msg {
		select {
		case <-m.ctx.Done():
		case <-time.After(500 * time.Millisecond):
		}

		return keyupMsg{}
	}
}
