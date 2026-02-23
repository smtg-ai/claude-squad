package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ByteMirror/hivemind/config"
	"github.com/ByteMirror/hivemind/session"
	"github.com/ByteMirror/hivemind/session/git"
	"github.com/ByteMirror/hivemind/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *home) updateSidebarItems() {
	if m.isMultiRepoView() {
		m.updateSidebarItemsMultiRepo()
		return
	}
	m.updateSidebarItemsSingleRepo()
}

// topicMeta extracts topic names, shared worktree flags, and auto-yes flags from a slice of topics.
func topicMeta(topics []*session.Topic) (names []string, shared map[string]bool, autoYes map[string]bool) {
	names = make([]string, len(topics))
	shared = make(map[string]bool)
	autoYes = make(map[string]bool)
	for i, t := range topics {
		names[i] = t.Name
		if t.SharedWorktree {
			shared[t.Name] = true
		}
		if t.AutoYes {
			autoYes[t.Name] = true
		}
	}
	return
}

// accumulateInstanceStats counts instances per topic and computes running/notification status.
func accumulateInstanceStats(instances []*session.Instance) (countByTopic map[string]int, ungroupedCount int, statuses map[string]ui.TopicStatus) {
	countByTopic = make(map[string]int)
	statuses = make(map[string]ui.TopicStatus)
	for _, inst := range instances {
		if inst.TopicName == "" {
			ungroupedCount++
		} else {
			countByTopic[inst.TopicName]++
		}
		st := statuses[inst.TopicName]
		if inst.Started() && !inst.Paused() && !inst.PromptDetected {
			st.HasRunning = true
		}
		if inst.Notified {
			st.HasNotification = true
		}
		statuses[inst.TopicName] = st
	}
	return
}

func (m *home) updateSidebarItemsSingleRepo() {
	topicNames, sharedTopics, autoYesTopics := topicMeta(m.topics)
	countByTopic, ungroupedCount, topicStatuses := accumulateInstanceStats(m.list.GetInstances())
	m.sidebar.SetItems(topicNames, countByTopic, ungroupedCount, sharedTopics, autoYesTopics, topicStatuses)
}

func (m *home) updateSidebarItemsMultiRepo() {
	allInstances := m.list.GetInstances()
	groups := make([]ui.RepoGroup, 0, len(m.activeRepoPaths))

	for _, rp := range m.activeRepoPaths {
		repoTopics := m.filterTopicsByRepo(m.allTopics, rp)
		topicNames, sharedTopics, autoYesTopics := topicMeta(repoTopics)

		// Filter instances belonging to this repo.
		var repoInstances []*session.Instance
		for _, inst := range allInstances {
			instRepo := inst.GetRepoPath()
			if instRepo == "" {
				instRepo = inst.Path
			}
			if instRepo == rp {
				repoInstances = append(repoInstances, inst)
			}
		}

		countByTopic, ungroupedCount, topicStatuses := accumulateInstanceStats(repoInstances)

		groups = append(groups, ui.RepoGroup{
			RepoPath:       rp,
			RepoName:       filepath.Base(rp),
			TopicNames:     topicNames,
			CountByTopic:   countByTopic,
			UngroupedCount: ungroupedCount,
			SharedTopics:   sharedTopics,
			AutoYesTopics:  autoYesTopics,
			TopicStatuses:  topicStatuses,
		})
	}

	m.sidebar.SetGroupedItems(groups)
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
	m.navPosition = panel // 0=sidebar, 1=instances
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
	m.list.SetFocused(false)
	m.navPosition = 2 // agent tab

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
		gitPane.Attach(worktree.GetWorktreePath(), selected.Title)
	}

	m.state = stateFocusAgent
	m.tabbedWindow.SetFocusMode(true)
	m.list.SetFocused(false)
	m.navPosition = 5 // git tab

	return func() tea.Msg {
		return gitTabTickMsg{}
	}
}

// enterTerminalFocusMode enters focus mode for the terminal tab.
// Attaches to the persistent shell session for the selected instance.
func (m *home) enterTerminalFocusMode() tea.Cmd {
	selected := m.list.GetSelectedInstance()
	if selected == nil || !selected.Started() || selected.Paused() {
		return nil
	}

	termPane := m.tabbedWindow.GetTerminalPane()
	worktree, err := selected.GetGitWorktree()
	if err != nil {
		return m.handleError(err)
	}
	termPane.Attach(worktree.GetWorktreePath(), selected.Title)

	m.state = stateFocusAgent
	m.tabbedWindow.SetFocusMode(true)
	m.list.SetFocused(false)
	m.navPosition = 3 // terminal tab

	return func() tea.Msg {
		return terminalTabTickMsg{}
	}
}

// enterDiffFocusMode enters focus mode for the diff tab.
// Unlike other focus modes, this doesn't spawn any subprocess — it just captures
// keyboard input for file navigation and scrolling.
func (m *home) enterDiffFocusMode() tea.Cmd {
	m.state = stateFocusAgent
	m.tabbedWindow.SetFocusMode(true)
	m.list.SetFocused(false)
	m.navPosition = 4
	return nil
}

// openFileInTerminal opens the given file (relative to the worktree) in the user's
// $EDITOR (defaulting to nvim) by switching to the terminal tab and sending the command.
func (m *home) openFileInTerminal(relativePath string) (tea.Model, tea.Cmd) {
	selected := m.list.GetSelectedInstance()
	if selected == nil || !selected.Started() || selected.Paused() {
		return m, nil
	}
	worktree, err := selected.GetGitWorktree()
	if err != nil {
		return m, m.handleError(err)
	}
	fullPath := filepath.Join(worktree.GetWorktreePath(), relativePath)

	// Exit current focus mode
	m.exitFocusMode()

	// Switch to terminal tab
	m.tabbedWindow.SetActiveTab(ui.TerminalTab)
	m.menu.SetInDiffTab(false)

	// Attach terminal
	termPane := m.tabbedWindow.GetTerminalPane()
	termPane.Attach(worktree.GetWorktreePath(), selected.Title)

	m.state = stateFocusAgent
	m.tabbedWindow.SetFocusMode(true)
	m.list.SetFocused(false)
	m.navPosition = 3

	// Use $EDITOR, fall back to nvim
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nvim"
	}

	// Send editor command after a short delay for the terminal to attach
	go func() {
		time.Sleep(100 * time.Millisecond)
		termPane.SendKey([]byte(editor + " " + fullPath + "\n"))
	}()

	return m, func() tea.Msg { return terminalTabTickMsg{} }
}

// detachTerminalTab detaches the terminal pane without killing the tmux session.
func (m *home) detachTerminalTab() {
	m.tabbedWindow.GetTerminalPane().Detach()
	m.tabbedWindow.SetTerminalContent("")
}

// detachGitTab detaches the git pane without killing the lazygit tmux session.
func (m *home) detachGitTab() {
	m.tabbedWindow.GetGitPane().Detach()
	m.tabbedWindow.SetGitContent("")
}

// exitFocusMode shuts down the embedded terminal and resets state.
// Handles cleanup for whichever tab type is currently focused.
func (m *home) exitFocusMode() {
	if m.tabbedWindow.IsInTerminalTab() {
		m.detachTerminalTab()
	}
	if m.tabbedWindow.IsInGitTab() {
		m.detachGitTab()
	}
	if m.embeddedTerminal != nil {
		m.embeddedTerminal.Close()
		m.embeddedTerminal = nil
	}
	m.state = stateDefault
	m.tabbedWindow.SetFocusMode(false)
	m.list.SetFocused(true)
	m.navPosition = 1 // back to instance list
}

// enterFocusModeForActiveTab enters the appropriate focus mode based on the active tab.
// If the instance is nil, not started, or paused, it only calls instanceChanged.
func (m *home) enterFocusModeForActiveTab() (tea.Model, tea.Cmd) {
	selected := m.list.GetSelectedInstance()
	if selected == nil || !selected.Started() || selected.Paused() {
		return m, m.instanceChanged()
	}

	switch m.tabbedWindow.GetActiveTab() {
	case ui.PreviewTab:
		focusCmd := m.enterFocusMode()
		return m, tea.Batch(m.instanceChanged(), focusCmd)
	case ui.TerminalTab:
		focusCmd := m.enterTerminalFocusMode()
		return m, tea.Batch(m.instanceChanged(), focusCmd)
	case ui.DiffTab:
		focusCmd := m.enterDiffFocusMode()
		return m, tea.Batch(m.instanceChanged(), focusCmd)
	case ui.GitTab:
		focusCmd := m.enterGitFocusMode()
		return m, tea.Batch(m.instanceChanged(), focusCmd)
	}
	return m, m.instanceChanged()
}

// cycleTab cycles the active tab forward (direction=1) or backward (direction=-1),
// handling git/terminal tab spawn/kill lifecycle.
func (m *home) cycleTab(direction int) (tea.Model, tea.Cmd) {
	wasGitTab := m.tabbedWindow.IsInGitTab()
	wasTerminalTab := m.tabbedWindow.IsInTerminalTab()

	current := m.tabbedWindow.GetActiveTab()
	numTabs := 4 // PreviewTab, TerminalTab, DiffTab, GitTab
	next := (current + direction + numTabs) % numTabs
	m.tabbedWindow.SetActiveTab(next)
	m.menu.SetInDiffTab(next == ui.DiffTab)

	if wasGitTab && next != ui.GitTab {
		m.detachGitTab()
	}
	if wasTerminalTab && next != ui.TerminalTab {
		m.detachTerminalTab()
	}
	if next == ui.GitTab {
		cmd := m.attachGitTab()
		return m, tea.Batch(m.instanceChanged(), cmd)
	}
	if next == ui.TerminalTab {
		cmd := m.spawnTerminalTab()
		return m, tea.Batch(m.instanceChanged(), cmd)
	}
	return m, m.instanceChanged()
}

// navigateWithShift handles Shift+Arrow traversal across all app panels.
// The navigation order loops: Sidebar(0) → Instances(1) → Agent(2) → Terminal(3) → Diff(4) → Git(5) → Sidebar...
// Agent, Terminal, and Git positions auto-enter focus mode for seamless interaction.
func (m *home) navigateWithShift(direction int) (tea.Model, tea.Cmd) {
	const numPositions = 6

	// Determine current position from tracked navPosition, with focus mode override
	current := m.navPosition
	if m.state == stateFocusAgent {
		switch m.tabbedWindow.GetActiveTab() {
		case ui.PreviewTab:
			current = 2
		case ui.TerminalTab:
			current = 3
		case ui.GitTab:
			current = 5
		}
	}

	next := (current + direction + numPositions) % numPositions

	// Exit current focus mode if active
	wasGitTab := m.tabbedWindow.IsInGitTab()
	wasTerminalTab := m.tabbedWindow.IsInTerminalTab()
	if m.state == stateFocusAgent {
		m.exitFocusMode()
	}

	m.navPosition = next

	switch next {
	case 0: // Sidebar
		if wasGitTab {
			m.detachGitTab()
		}
		if wasTerminalTab {
			m.detachTerminalTab()
		}
		m.setFocus(0)
		m.navPosition = 0 // setFocus sets this too, but be explicit
		return m, nil

	case 1: // Instance list
		if wasGitTab {
			m.detachGitTab()
		}
		if wasTerminalTab {
			m.detachTerminalTab()
		}
		m.setFocus(1)
		m.navPosition = 1
		return m, m.instanceChanged()

	case 2: // Agent tab (with focus)
		if wasGitTab {
			m.detachGitTab()
		}
		if wasTerminalTab {
			m.detachTerminalTab()
		}
		m.setFocus(1)
		m.list.SetFocused(false)
		m.tabbedWindow.SetActiveTab(ui.PreviewTab)
		m.menu.SetInDiffTab(false)
		selected := m.list.GetSelectedInstance()
		if selected == nil || !selected.Started() || selected.Paused() {
			m.navPosition = 2
			return m, m.instanceChanged()
		}
		focusCmd := m.enterFocusMode() // sets navPosition=2
		return m, tea.Batch(m.instanceChanged(), focusCmd)

	case 3: // Terminal tab (with focus)
		if wasGitTab {
			m.detachGitTab()
		}
		m.setFocus(1)
		m.list.SetFocused(false)
		m.tabbedWindow.SetActiveTab(ui.TerminalTab)
		m.menu.SetInDiffTab(false)
		selected := m.list.GetSelectedInstance()
		if selected == nil || !selected.Started() || selected.Paused() {
			m.navPosition = 3
			return m, m.instanceChanged()
		}
		focusCmd := m.enterTerminalFocusMode()
		return m, tea.Batch(m.instanceChanged(), focusCmd)

	case 4: // Diff tab (with focus — Up/Down navigate files, scroll via viewport)
		if wasGitTab {
			m.detachGitTab()
		}
		if wasTerminalTab {
			m.detachTerminalTab()
		}
		m.setFocus(1)
		m.list.SetFocused(false)
		m.tabbedWindow.SetActiveTab(ui.DiffTab)
		m.menu.SetInDiffTab(true)
		selected := m.list.GetSelectedInstance()
		if selected == nil || !selected.Started() || selected.Paused() {
			m.navPosition = 4
			return m, m.instanceChanged()
		}
		m.state = stateFocusAgent
		m.tabbedWindow.SetFocusMode(true)
		m.navPosition = 4
		return m, m.instanceChanged()

	case 5: // Git tab (with focus)
		if wasTerminalTab {
			m.detachTerminalTab()
		}
		m.setFocus(1)
		m.list.SetFocused(false)
		m.tabbedWindow.SetActiveTab(ui.GitTab)
		m.menu.SetInDiffTab(false)
		selected := m.list.GetSelectedInstance()
		if selected == nil || !selected.Started() || selected.Paused() {
			m.navPosition = 5
			return m, m.instanceChanged()
		}
		focusCmd := m.enterGitFocusMode()
		if focusCmd == nil {
			m.navPosition = 5 // still at git tab even without focus
		}
		cmd := m.instanceChanged()
		return m, tea.Batch(cmd, focusCmd)
	}

	return m, nil
}

// cycleInstanceInPlace changes the selected instance while staying on the current tab.
// If in focus mode, it exits focus, switches the instance, and re-enters focus seamlessly.
func (m *home) cycleInstanceInPlace(direction int) (tea.Model, tea.Cmd) {
	n := m.list.NumInstances()
	if n == 0 {
		return m, nil
	}
	idx := m.list.GetSelectedIdx()
	if direction > 0 {
		if idx >= n-1 {
			// Wrap to top: move up to index 0
			for i := idx; i > 0; i-- {
				m.list.Up()
			}
		} else {
			m.list.Down()
		}
	} else {
		if idx <= 0 {
			// Wrap to bottom: move down to last index
			for i := idx; i < n-1; i++ {
				m.list.Down()
			}
		} else {
			m.list.Up()
		}
	}

	wasFocused := m.state == stateFocusAgent
	activeTab := m.tabbedWindow.GetActiveTab()
	savedNavPos := m.navPosition

	if wasFocused {
		m.exitFocusMode()
	}

	// For git tab: detach from old lazygit, then use enterGitFocusMode to attach
	// to the new instance BEFORE instanceChanged, so NeedsRespawn sees the fresh session.
	if wasFocused && activeTab == ui.GitTab {
		m.detachGitTab()
		focusCmd := m.enterGitFocusMode()
		cmd := m.instanceChanged()
		return m, tea.Batch(tea.WindowSize(), cmd, focusCmd)
	}

	// For terminal tab: detach from old instance, attach to new one.
	if wasFocused && activeTab == ui.TerminalTab {
		m.detachTerminalTab()
		focusCmd := m.enterTerminalFocusMode()
		cmd := m.instanceChanged()
		return m, tea.Batch(tea.WindowSize(), cmd, focusCmd)
	}

	// For agent tab: use enterFocusMode which creates the embedded terminal.
	if wasFocused && activeTab == ui.PreviewTab {
		cmd := m.instanceChanged()
		focusCmd := m.enterFocusMode()
		return m, tea.Batch(tea.WindowSize(), cmd, focusCmd)
	}

	// For diff tab: re-enter focus mode and update diff content.
	if wasFocused && activeTab == ui.DiffTab {
		focusCmd := m.enterDiffFocusMode()
		cmd := m.instanceChanged()
		return m, tea.Batch(tea.WindowSize(), cmd, focusCmd)
	}

	// Non-focused tabs: just update content.
	cmd := m.instanceChanged()
	m.navPosition = savedNavPos
	return m, cmd
}

func (m *home) filterInstancesByTopic() {
	selectedID := m.sidebar.GetSelectedID()
	selectedRepoPath := m.sidebar.GetSelectedRepoPath()

	topicFilter := selectedID
	if selectedID == ui.SidebarAll {
		topicFilter = ""
	}
	m.list.SetFilterByRepoAndTopic(topicFilter, selectedRepoPath)
}

// filterSearchWithTopic applies the search query scoped to the currently selected topic.
func (m *home) filterSearchWithTopic() {
	query := strings.ToLower(m.sidebar.GetSearchQuery())
	selectedID := m.sidebar.GetSelectedID()
	selectedRepoPath := m.sidebar.GetSelectedRepoPath()

	topicFilter := selectedID
	if selectedID == ui.SidebarAll {
		topicFilter = ""
	}
	m.list.SetSearchFilterWithTopicAndRepo(query, topicFilter, selectedRepoPath)
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

// rebuildInstanceList clears the list and repopulates with instances matching activeRepoPaths.
func (m *home) rebuildInstanceList() {
	m.list.Clear()
	for _, inst := range m.allInstances {
		if m.instanceMatchesActiveRepos(inst) {
			m.list.AddInstance(inst)()
		}
	}
	m.topics = m.filterTopicsByActiveRepos()
	m.filterInstancesByTopic()
	m.updateSidebarItems()
}

// getKnownRepos returns distinct repo paths from allInstances, recent repos, plus activeRepoPaths.
func (m *home) getKnownRepos() []string {
	seen := make(map[string]bool)
	for _, rp := range m.activeRepoPaths {
		seen[rp] = true
	}
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

	activeSet := m.activeRepoSet()
	m.repoPickerMap = make(map[string]string)
	items := make([]string, 0, len(repos)+1)
	for _, rp := range repos {
		base := filepath.Base(rp)
		name := base
		if baseCount[base] > 1 {
			name = filepath.Base(filepath.Dir(rp)) + "/" + base
		}
		count := countByRepo[rp]
		var label string
		if activeSet[rp] {
			label = fmt.Sprintf("\u2713 %s (%d)", name, count)
		} else {
			label = fmt.Sprintf("  %s (%d)", name, count)
		}
		items = append(items, label)
		m.repoPickerMap[label] = rp
	}
	items = append(items, "Open folder...")
	return items
}

// toggleRepo toggles a repo on/off in the active set. Cannot remove the last repo.
// switchToRepo replaces activeRepoPaths with a single repo (exclusive switch via Enter).
func (m *home) switchToRepo(selection string) {
	rp, ok := m.repoPickerMap[selection]
	if !ok {
		return
	}
	m.activeRepoPaths = []string{rp}
	if state, ok := m.appState.(*config.State); ok {
		state.AddRecentRepo(rp)
	}
	m.updateRepoDisplay()
	m.rebuildInstanceList()
}

func (m *home) toggleRepo(selection string) {
	rp, ok := m.repoPickerMap[selection]
	if !ok {
		return
	}
	activeSet := m.activeRepoSet()
	if activeSet[rp] {
		// Remove — but only if it's not the last one
		if len(m.activeRepoPaths) <= 1 {
			return
		}
		newPaths := make([]string, 0, len(m.activeRepoPaths)-1)
		for _, p := range m.activeRepoPaths {
			if p != rp {
				newPaths = append(newPaths, p)
			}
		}
		m.activeRepoPaths = newPaths
	} else {
		// Add
		m.activeRepoPaths = append(m.activeRepoPaths, rp)
		if state, ok := m.appState.(*config.State); ok {
			state.AddRecentRepo(rp)
		}
	}
	m.updateRepoDisplay()
	m.rebuildInstanceList()
}

// addActiveRepo adds a repo path to the active set if not already present.
func (m *home) addActiveRepo(path string) {
	for _, rp := range m.activeRepoPaths {
		if rp == path {
			return // already active
		}
	}
	m.activeRepoPaths = append(m.activeRepoPaths, path)
	if state, ok := m.appState.(*config.State); ok {
		state.AddRecentRepo(path)
	}
	m.updateRepoDisplay()
	m.rebuildInstanceList()
}

// updateRepoDisplay updates the sidebar's repo name display based on active repos.
func (m *home) updateRepoDisplay() {
	names := make([]string, len(m.activeRepoPaths))
	for i, rp := range m.activeRepoPaths {
		names[i] = filepath.Base(rp)
	}
	m.sidebar.SetRepoNames(names)
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

// filterTopicsByActiveRepos returns topics matching any of the active repo paths.
func (m *home) filterTopicsByActiveRepos() []*session.Topic {
	activeSet := m.activeRepoSet()
	var filtered []*session.Topic
	for _, t := range m.allTopics {
		if activeSet[t.Path] {
			filtered = append(filtered, t)
		}
	}
	return filtered
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

	m.tabbedWindow.SetInstance(selected)
	m.tabbedWindow.MarkContentStale()
	// Invalidate any in-flight async preview fetch so stale content isn't applied
	m.previewGeneration++
	m.previewFetching = false
	// Update menu with current instance
	m.menu.SetInstance(selected)

	// Reattach lazygit if the selected instance changed while on the git tab
	if m.tabbedWindow.IsInGitTab() {
		gitPane := m.tabbedWindow.GetGitPane()
		title := ""
		if selected != nil {
			title = selected.Title
		}
		if gitPane.NeedsRespawn(title) {
			return m.attachGitTab()
		}
	}

	// Reattach terminal if the selected instance changed while on the terminal tab
	if m.tabbedWindow.IsInTerminalTab() {
		termPane := m.tabbedWindow.GetTerminalPane()
		title := ""
		if selected != nil {
			title = selected.Title
		}
		if termPane.CurrentInstance() != title {
			return m.spawnTerminalTab()
		}
	}

	return nil
}

// attachGitTab attaches lazygit for the selected instance and starts the render ticker.
// The underlying tmux session persists across switches — only the PTY connection is re-created.
func (m *home) attachGitTab() tea.Cmd {
	selected := m.list.GetSelectedInstance()
	if selected == nil || !selected.Started() || selected.Paused() {
		return nil
	}

	worktree, err := selected.GetGitWorktree()
	if err != nil {
		return m.handleError(err)
	}

	gitPane := m.tabbedWindow.GetGitPane()
	gitPane.Attach(worktree.GetWorktreePath(), selected.Title)

	return func() tea.Msg {
		return gitTabTickMsg{}
	}
}

// spawnTerminalTab attaches to the terminal for the selected instance and starts the render ticker.
func (m *home) spawnTerminalTab() tea.Cmd {
	selected := m.list.GetSelectedInstance()
	if selected == nil || !selected.Started() || selected.Paused() {
		return nil
	}

	worktree, err := selected.GetGitWorktree()
	if err != nil {
		return m.handleError(err)
	}

	termPane := m.tabbedWindow.GetTerminalPane()
	termPane.Attach(worktree.GetWorktreePath(), selected.Title)

	return func() tea.Msg {
		return terminalTabTickMsg{}
	}
}

// buildBranchPickerItems returns sorted local branch names for the given repo path.
func (m *home) buildBranchPickerItems(repoPath string) []string {
	branches, err := git.ListLocalBranches(repoPath)
	if err != nil {
		return nil
	}
	sort.Strings(branches)
	return branches
}

// killGitTab kills the lazygit subprocess.
func (m *home) killGitTab() {
	m.tabbedWindow.GetGitPane().Kill()
	m.tabbedWindow.SetGitContent("")
}

// handleGitWorktreeChanged is called when the lazygit session has drifted to a
// different worktree path than the current instance expects. This happens when
// the user follows a lazygit "switch to worktree" prompt for a branch that is
// checked out elsewhere.
//
// Because the Claude Code session's working directory is fixed at startup, it
// remains on the original branch regardless of where lazygit navigates — causing
// a confusing split where lazygit shows branch B but Claude Code reports branch A.
//
// This handler resets lazygit to the instance's own worktree so both sessions
// stay in sync, and shows a toast explaining why.
func (m *home) handleGitWorktreeChanged(newPath string) (tea.Model, tea.Cmd) {
	selected := m.list.GetSelectedInstance()
	if selected == nil {
		return m, nil
	}

	// Identify which instance (if any) owns the destination path for the toast.
	var otherTitle string
	for _, inst := range m.allInstances {
		if !inst.Started() || inst.Paused() {
			continue
		}
		wt, err := inst.GetGitWorktree()
		if err != nil {
			continue
		}
		if wt.GetWorktreePath() == newPath {
			otherTitle = inst.Title
			break
		}
	}

	// Exit focus mode first so the UI is in a clean state before the reset.
	if m.state == stateFocusAgent {
		m.exitFocusMode()
	}

	// Kill the drifted lazygit session; attachGitTab will restart it in the
	// correct worktree path.
	gitPane := m.tabbedWindow.GetGitPane()
	gitPane.KillSession(selected.Title)

	toastMsg := "Lazygit reset to current instance's worktree"
	if otherTitle != "" {
		toastMsg = fmt.Sprintf("Lazygit reset: '%s' has its own worktree", otherTitle)
	}
	m.toastManager.Info(toastMsg)

	return m, tea.Batch(m.attachGitTab(), m.toastTickCmd())
}
