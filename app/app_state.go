package app

import (
	"fmt"
	"github.com/ByteMirror/hivemind/config"
	"github.com/ByteMirror/hivemind/keys"
	"github.com/ByteMirror/hivemind/session"
	"github.com/ByteMirror/hivemind/ui"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *home) updateSidebarItems() {
	topicNames := make([]string, len(m.topics))
	countByTopic := make(map[string]int)
	sharedTopics := make(map[string]bool)
	topicStatuses := make(map[string]ui.TopicStatus)
	ungroupedCount := 0

	for i, t := range m.topics {
		topicNames[i] = t.Name
		if t.SharedWorktree {
			sharedTopics[t.Name] = true
		}
	}

	for _, inst := range m.list.GetInstances() {
		if inst.TopicName == "" {
			ungroupedCount++
		} else {
			countByTopic[inst.TopicName]++
		}

		// Track running and notification status per topic key.
		// An instance is "active" if it's started, not paused, and hasn't shown
		// a prompt yet (meaning the program is still working).
		topicKey := inst.TopicName // "" for ungrouped
		st := topicStatuses[topicKey]
		if inst.Started() && !inst.Paused() && !inst.PromptDetected {
			st.HasRunning = true
		}
		if inst.Notified {
			st.HasNotification = true
		}
		topicStatuses[topicKey] = st
	}

	m.sidebar.SetItems(topicNames, countByTopic, ungroupedCount, sharedTopics, topicStatuses)
}

// getMovableTopicNames returns topic names that a non-shared instance can be moved to.
func (m *home) getMovableTopicNames() []string {
	names := []string{"(Ungrouped)"}
	for _, t := range m.topics {
		names = append(names, t.Name)
	}
	return names
}

// setFocus updates which panel has focus and syncs the focused state to sidebar and list.
func (m *home) setFocus(panel int) {
	m.focusedPanel = panel
	m.sidebar.SetFocused(panel == 0)
	m.list.SetFocused(panel == 1)
}

// enterFocusMode enters focus/insert mode and starts the fast preview ticker.
// enterFocusMode directly attaches to the selected instance's tmux session.
// This takes over the terminal for native performance. Ctrl+Q detaches.
// enterFocusMode creates an embedded terminal emulator connected to the instance's
// PTY and starts the 30fps render ticker. Input goes directly to the PTY (zero latency),
// display is rendered from the emulator's screen buffer (no subprocess calls).
func (m *home) enterFocusMode() tea.Cmd {
	selected := m.list.GetSelectedInstance()
	if selected == nil {
		return nil
	}

	cols, rows := m.tabbedWindow.GetPreviewSize()
	if cols < 10 {
		cols = 80
	}
	if rows < 5 {
		rows = 24
	}
	term, err := selected.NewEmbeddedTerminalForInstance(cols, rows)
	if err != nil {
		return m.handleError(err)
	}

	m.embeddedTerminal = term
	m.state = stateFocusAgent
	m.tabbedWindow.SetFocusMode(true)

	// Start the 30fps render ticker
	return func() tea.Msg {
		return focusPreviewTickMsg{}
	}
}

// enterGitFocusMode enters focus mode for the git tab (lazygit).
// Spawns lazygit if it's not already running.
func (m *home) enterGitFocusMode() tea.Cmd {
	selected := m.list.GetSelectedInstance()
	if selected == nil || !selected.Started() || selected.Paused() {
		return nil
	}

	gitPane := m.tabbedWindow.GetGitPane()
	if !gitPane.IsRunning() {
		worktree, err := selected.GetGitWorktree()
		if err != nil {
			return m.handleError(err)
		}
		gitPane.Spawn(worktree.GetWorktreePath(), selected.Title)
	}

	m.state = stateFocusAgent
	m.tabbedWindow.SetFocusMode(true)

	return func() tea.Msg {
		return gitTabTickMsg{}
	}
}

// exitFocusMode shuts down the embedded terminal and resets state.
func (m *home) exitFocusMode() {
	if m.embeddedTerminal != nil {
		m.embeddedTerminal.Close()
		m.embeddedTerminal = nil
	}
	m.state = stateDefault
	m.tabbedWindow.SetFocusMode(false)
}

// fkeyToTab maps F1/F2/F3 key strings to tab indices.
func fkeyToTab(key string) (int, bool) {
	switch key {
	case "f1":
		return ui.PreviewTab, true
	case "f2":
		return ui.DiffTab, true
	case "f3":
		return ui.GitTab, true
	default:
		return 0, false
	}
}

// switchToTab switches to the specified tab, handling git tab spawn/kill lifecycle.
func (m *home) switchToTab(name keys.KeyName) (tea.Model, tea.Cmd) {
	var targetTab int
	switch name {
	case keys.KeyTabAgent:
		targetTab = ui.PreviewTab
	case keys.KeyTabDiff:
		targetTab = ui.DiffTab
	case keys.KeyTabGit:
		targetTab = ui.GitTab
	default:
		return m, nil
	}

	if m.tabbedWindow.GetActiveTab() == targetTab {
		return m, nil
	}

	wasGitTab := m.tabbedWindow.IsInGitTab()
	m.tabbedWindow.SetActiveTab(targetTab)
	m.menu.SetInDiffTab(targetTab == ui.DiffTab)

	if wasGitTab && targetTab != ui.GitTab {
		m.killGitTab()
	}
	if targetTab == ui.GitTab {
		cmd := m.spawnGitTab()
		return m, tea.Batch(m.instanceChanged(), cmd)
	}
	return m, m.instanceChanged()
}

func (m *home) filterInstancesByTopic() {
	selectedID := m.sidebar.GetSelectedID()
	switch selectedID {
	case ui.SidebarAll:
		m.list.SetFilter("")
	case ui.SidebarUngrouped:
		m.list.SetFilter(ui.SidebarUngrouped)
	default:
		m.list.SetFilter(selectedID)
	}
}

// filterSearchWithTopic applies the search query scoped to the currently selected topic.
func (m *home) filterSearchWithTopic() {
	query := strings.ToLower(m.sidebar.GetSearchQuery())
	selectedID := m.sidebar.GetSelectedID()
	topicFilter := ""
	switch selectedID {
	case ui.SidebarAll:
		topicFilter = ""
	case ui.SidebarUngrouped:
		topicFilter = ui.SidebarUngrouped
	default:
		topicFilter = selectedID
	}
	m.list.SetSearchFilterWithTopic(query, topicFilter)
}

func (m *home) filterBySearch() {
	query := strings.ToLower(m.sidebar.GetSearchQuery())
	if query == "" {
		m.sidebar.UpdateMatchCounts(nil, 0)
		m.filterInstancesByTopic()
		return
	}
	m.list.SetSearchFilter(query)

	// Calculate match counts per topic for sidebar dimming
	matchesByTopic := make(map[string]int)
	totalMatches := 0
	for _, inst := range m.list.GetInstances() {
		if strings.Contains(strings.ToLower(inst.Title), query) ||
			strings.Contains(strings.ToLower(inst.TopicName), query) {
			matchesByTopic[inst.TopicName]++
			totalMatches++
		}
	}
	m.sidebar.UpdateMatchCounts(matchesByTopic, totalMatches)
}

// rebuildInstanceList clears the list and repopulates with instances matching activeRepoPath.
func (m *home) rebuildInstanceList() {
	m.list.Clear()
	for _, inst := range m.allInstances {
		repoPath := inst.GetRepoPath()
		if repoPath == "" || repoPath == m.activeRepoPath {
			m.list.AddInstance(inst)()
		}
	}
	m.topics = m.filterTopicsByRepo(m.allTopics, m.activeRepoPath)
	m.filterInstancesByTopic()
	m.updateSidebarItems()
}

// getKnownRepos returns distinct repo paths from allInstances, recent repos, plus activeRepoPath.
func (m *home) getKnownRepos() []string {
	seen := make(map[string]bool)
	seen[m.activeRepoPath] = true
	for _, inst := range m.allInstances {
		rp := inst.GetRepoPath()
		if rp != "" {
			seen[rp] = true
		}
	}
	// Include recent repos from persisted state
	if state, ok := m.appState.(*config.State); ok {
		for _, rp := range state.GetRecentRepos() {
			seen[rp] = true
		}
	}
	repos := make([]string, 0, len(seen))
	for rp := range seen {
		repos = append(repos, rp)
	}
	sort.Strings(repos)
	return repos
}

// buildRepoPickerItems returns display strings for the repo picker.
func (m *home) buildRepoPickerItems() []string {
	repos := m.getKnownRepos()
	countByRepo := make(map[string]int)
	for _, inst := range m.allInstances {
		rp := inst.GetRepoPath()
		if rp != "" {
			countByRepo[rp]++
		}
	}

	// Detect duplicate basenames to disambiguate
	baseCount := make(map[string]int)
	for _, rp := range repos {
		baseCount[filepath.Base(rp)]++
	}

	m.repoPickerMap = make(map[string]string)
	items := make([]string, 0, len(repos)+1)
	for _, rp := range repos {
		base := filepath.Base(rp)
		name := base
		if baseCount[base] > 1 {
			// Disambiguate with parent directory
			name = filepath.Base(filepath.Dir(rp)) + "/" + base
		}
		count := countByRepo[rp]
		var label string
		if rp == m.activeRepoPath {
			label = fmt.Sprintf("%s (%d) ●", name, count)
		} else {
			label = fmt.Sprintf("%s (%d)", name, count)
		}
		items = append(items, label)
		m.repoPickerMap[label] = rp
	}
	items = append(items, "Open folder...")
	return items
}

// switchToRepo switches the active repo based on picker selection text.
func (m *home) switchToRepo(selection string) {
	rp, ok := m.repoPickerMap[selection]
	if !ok {
		return
	}
	m.activeRepoPath = rp
	m.sidebar.SetRepoName(filepath.Base(rp))
	if state, ok := m.appState.(*config.State); ok {
		state.AddRecentRepo(rp)
	}
	m.rebuildInstanceList()
}

// saveAllInstances saves allInstances (all repos) to storage.
func (m *home) saveAllInstances() error {
	return m.storage.SaveInstances(m.allInstances)
}

// removeFromAllInstances removes an instance from the master list by title.
func (m *home) removeFromAllInstances(title string) {
	for i, inst := range m.allInstances {
		if inst.Title == title {
			m.allInstances = append(m.allInstances[:i], m.allInstances[i+1:]...)
			return
		}
	}
}

// filterTopicsByRepo returns topics that belong to the given repo path.
func (m *home) filterTopicsByRepo(topics []*session.Topic, repoPath string) []*session.Topic {
	var filtered []*session.Topic
	for _, t := range topics {
		if t.Path == repoPath {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// saveAllTopics saves all topics (across all repos) to storage.
func (m *home) saveAllTopics() error {
	return m.storage.SaveTopics(m.allTopics)
}

// instanceChanged updates the preview pane, menu, and diff pane based on the selected instance. It returns an error
// Cmd if there was any error.
func (m *home) instanceChanged() tea.Cmd {
	// selected may be nil
	selected := m.list.GetSelectedInstance()

	// Clear notification when user selects this instance — they've seen it
	if selected != nil && selected.Notified {
		selected.Notified = false
		m.updateSidebarItems()
	}

	m.tabbedWindow.UpdateDiff(selected)
	m.tabbedWindow.SetInstance(selected)
	// Update menu with current instance
	m.menu.SetInstance(selected)

	// If there's no selected instance, we don't need to update the preview.
	if err := m.tabbedWindow.UpdatePreview(selected); err != nil {
		return m.handleError(err)
	}

	// Respawn lazygit if the selected instance changed while on the git tab
	if m.tabbedWindow.IsInGitTab() {
		gitPane := m.tabbedWindow.GetGitPane()
		title := ""
		if selected != nil {
			title = selected.Title
		}
		if gitPane.NeedsRespawn(title) {
			return m.spawnGitTab()
		}
	}

	return nil
}

// spawnGitTab spawns lazygit for the selected instance and starts the render ticker.
func (m *home) spawnGitTab() tea.Cmd {
	selected := m.list.GetSelectedInstance()
	if selected == nil || !selected.Started() || selected.Paused() {
		return nil
	}

	worktree, err := selected.GetGitWorktree()
	if err != nil {
		return m.handleError(err)
	}

	gitPane := m.tabbedWindow.GetGitPane()
	gitPane.Spawn(worktree.GetWorktreePath(), selected.Title)

	return func() tea.Msg {
		return gitTabTickMsg{}
	}
}

// killGitTab kills the lazygit subprocess.
func (m *home) killGitTab() {
	m.tabbedWindow.GetGitPane().Kill()
}
