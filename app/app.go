package app

import (
	"claude-squad/config"
	"claude-squad/keys"
	"claude-squad/log"
	"claude-squad/project"
	"claude-squad/session"
	"claude-squad/ui"
	"claude-squad/ui/overlay"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const GlobalInstanceLimit = 10

// Run is the main entrypoint into the application.
func Run(ctx context.Context, program string, autoYes bool) error {
	p := tea.NewProgram(
		newHome(ctx, program, autoYes),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(), // Mouse scroll
	)
	_, err := p.Run()
	return err
}

type state int

const (
	stateDefault state = iota
	// stateNew is the state when the user is creating a new instance.
	stateNew
	// statePrompt is the state when the user is entering a prompt.
	statePrompt
	// stateHelp is the state when a help screen is displayed.
	stateHelp
	// stateConfirm is the state when a confirmation modal is displayed.
	stateConfirm
	// stateAddProject is the state when the user is adding a new project.
	stateAddProject
	// stateMCPManage is the state when the MCP management overlay is displayed.
	stateMCPManage
	// stateProjectHistory is the state when the project history overlay is displayed.
	stateProjectHistory
)

type home struct {
	ctx context.Context

	// -- Storage and Configuration --

	program string
	autoYes bool

	// storage is the interface for saving/loading data to/from the app's state
	storage *session.Storage
	// appConfig stores persistent application configuration
	appConfig *config.Config
	// appState stores persistent application state like seen help screens
	appState config.AppState

	// -- State --

	// state is the current discrete state of the application
	state state
	// newInstanceFinalizer is called when the state is stateNew and then you press enter.
	// It registers the new instance in the list after the instance has been started.
	newInstanceFinalizer func()

	// promptAfterName tracks if we should enter prompt mode after naming
	promptAfterName bool

	// keySent is used to manage underlining menu items
	keySent bool

	// -- UI Components --

	// list displays the list of instances
	list *ui.List
	// menu displays the bottom menu
	menu *ui.Menu
	// tabbedWindow displays the tabbed window with preview, diff, and console panes
	tabbedWindow *ui.TabbedWindow
	// errBox displays error messages
	errBox *ui.ErrBox
	// global spinner instance. we plumb this down to where it's needed
	spinner spinner.Model
	// textInputOverlay handles text input with state
	textInputOverlay *overlay.TextInputOverlay
	// textOverlay displays text information
	textOverlay *overlay.TextOverlay
	// confirmationOverlay displays confirmation modals
	confirmationOverlay *overlay.ConfirmationOverlay
	// projectManager manages multiple projects
	projectManager *project.ProjectManager
	// projectInputOverlay handles project input
	projectInputOverlay *ui.ProjectInputOverlay
	// mcpOverlay handles MCP server management
	mcpOverlay *overlay.MCPOverlay
	// projectHistoryOverlay handles project history selection
	projectHistoryOverlay *overlay.ProjectHistoryOverlay
}

func newHome(ctx context.Context, program string, autoYes bool) *home {
	// Load application config
	appConfig := config.LoadConfig()

	// Load application state
	appState := config.LoadState()

	// Initialize storage
	storage, err := session.NewStorage(appState)
	if err != nil {
		fmt.Printf("Failed to initialize storage: %v\n", err)
		os.Exit(1)
	}

	// Initialize project manager
	projectStorage := project.NewStateProjectStorage(appState)
	projectManager, err := project.NewProjectManager(projectStorage)
	if err != nil {
		fmt.Printf("Failed to initialize project manager: %v\n", err)
		os.Exit(1)
	}

	h := &home{
		ctx:                 ctx,
		spinner:             spinner.New(spinner.WithSpinner(spinner.MiniDot)),
		menu:                ui.NewMenu(),
		tabbedWindow:        ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewDiffPane(), ui.NewConsolePane()),
		errBox:              ui.NewErrBox(),
		storage:             storage,
		appConfig:           appConfig,
		program:             program,
		autoYes:             autoYes,
		state:               stateDefault,
		appState:            appState,
		projectManager:      projectManager,
		projectInputOverlay: ui.NewProjectInputOverlay(),
	}
	h.list = ui.NewList(&h.spinner, autoYes, projectManager, appConfig)

	// Load saved instances
	instances, err := storage.LoadInstances()
	if err != nil {
		fmt.Printf("Failed to load instances: %v\n", err)
		os.Exit(1)
	}

	// Add loaded instances to the list
	for _, instance := range instances {
		// Call the finalizer immediately.
		h.list.AddInstance(instance)()
		if autoYes {
			instance.AutoYes = true
		}
	}

	return h
}

// updateHandleWindowSizeEvent sets the sizes of the components.
// The components will try to render inside their bounds.
func (m *home) updateHandleWindowSizeEvent(msg tea.WindowSizeMsg) {
	// List takes 30% of width, preview takes 70%
	listWidth := int(float32(msg.Width) * 0.3)
	tabsWidth := msg.Width - listWidth

	// Menu takes 10% of height, list and window take 90%
	contentHeight := int(float32(msg.Height) * 0.9)
	menuHeight := msg.Height - contentHeight - 1     // minus 1 for error box
	m.errBox.SetSize(int(float32(msg.Width)*0.9), 1) // error box takes 1 row

	m.tabbedWindow.SetSize(tabsWidth, contentHeight)
	m.list.SetSize(listWidth, contentHeight)

	if m.textInputOverlay != nil {
		m.textInputOverlay.SetSize(int(float32(msg.Width)*0.6), int(float32(msg.Height)*0.4))
	}
	if m.textOverlay != nil {
		m.textOverlay.SetWidth(int(float32(msg.Width) * 0.6))
	}
	if m.projectInputOverlay != nil {
		m.projectInputOverlay.SetSize(msg.Width, msg.Height)
	}
	if m.mcpOverlay != nil {
		m.mcpOverlay.SetSize(int(float32(msg.Width)*0.8), int(float32(msg.Height)*0.8))
	}
	if m.projectHistoryOverlay != nil {
		m.projectHistoryOverlay.SetSize(int(float32(msg.Width)*0.8), int(float32(msg.Height)*0.8))
	}

	previewWidth, previewHeight := m.tabbedWindow.GetPreviewSize()
	if err := m.list.SetSessionPreviewSize(previewWidth, previewHeight); err != nil {
		log.ErrorLog.Print(err)
	}
	m.menu.SetSize(msg.Width, menuHeight)
}

func (m *home) Init() tea.Cmd {
	// Upon starting, we want to start the spinner. Whenever we get a spinner.TickMsg, we
	// update the spinner, which sends a new spinner.TickMsg. I think this lasts forever lol.
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			time.Sleep(100 * time.Millisecond)
			return previewTickMsg{}
		},
		tickUpdateMetadataCmd,
	)
}

func (m *home) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case hideErrMsg:
		m.errBox.Clear()
	case previewTickMsg:
		cmd := m.instanceChanged()
		return m, tea.Batch(
			cmd,
			func() tea.Msg {
				time.Sleep(100 * time.Millisecond)
				return previewTickMsg{}
			},
		)
	case keyupMsg:
		m.menu.ClearKeydown()
		return m, nil
	case tickUpdateMetadataMessage:
		for _, instance := range m.list.GetInstances() {
			if !instance.Started() || instance.Paused() {
				continue
			}
			updated, prompt := instance.HasUpdated()
			if updated {
				instance.SetStatus(session.Running)
			} else {
				if prompt {
					instance.TapEnter()
				} else {
					instance.SetStatus(session.Ready)
				}
			}
			if err := instance.UpdateDiffStats(); err != nil {
				log.WarningLog.Printf("could not update diff stats: %v", err)
			}
		}
		return m, tickUpdateMetadataCmd
	case tea.MouseMsg:
		// Handle mouse wheel scrolling in both preview and diff views
		if msg.Action == tea.MouseActionPress {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				m.tabbedWindow.ScrollUp()
				return m, m.instanceChanged()
			case tea.MouseButtonWheelDown:
				m.tabbedWindow.ScrollDown()
				return m, m.instanceChanged()
			}
		}
		return m, nil
	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	case tea.WindowSizeMsg:
		m.updateHandleWindowSizeEvent(msg)
		return m, nil
	case error:
		// Handle errors from confirmation actions
		return m, m.handleError(msg)
	case instanceChangedMsg:
		// Handle instance changed after confirmation action
		return m, m.instanceChanged()
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *home) handleQuit() (tea.Model, tea.Cmd) {
	if err := m.storage.SaveInstances(m.list.GetInstances()); err != nil {
		return m, m.handleError(err)
	}
	return m, tea.Quit
}

func (m *home) handleMenuHighlighting(msg tea.KeyMsg) (cmd tea.Cmd, returnEarly bool) {
	// Handle menu highlighting when you press a button. We intercept it here and immediately return to
	// update the ui while re-sending the keypress. Then, on the next call to this, we actually handle the keypress.
	if m.keySent {
		m.keySent = false
		return nil, false
	}
	if m.state == statePrompt || m.state == stateHelp || m.state == stateConfirm || m.state == stateAddProject || m.state == stateMCPManage || m.state == stateProjectHistory {
		return nil, false
	}
	// If it's in the global keymap, we should try to highlight it.
	name, ok := keys.GlobalKeyStringsMap[msg.String()]
	if !ok {
		return nil, false
	}

	if m.list.GetSelectedInstance() != nil && m.list.GetSelectedInstance().Paused() && name == keys.KeyEnter {
		return nil, false
	}
	// Remove the shift key blocking to allow scroll functionality

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

func (m *home) handleKeyPress(msg tea.KeyMsg) (mod tea.Model, cmd tea.Cmd) {
	cmd, returnEarly := m.handleMenuHighlighting(msg)
	if returnEarly {
		return m, cmd
	}

	if m.state == stateHelp {
		return m.handleHelpState(msg)
	}

	if m.state == stateNew {
		// Handle quit commands first. Don't handle q because the user might want to type that.
		if msg.String() == "ctrl+c" {
			m.state = stateDefault
			m.promptAfterName = false
			m.list.Kill()
			return m, tea.Sequence(
				tea.WindowSize(),
				func() tea.Msg {
					m.menu.SetState(ui.StateDefault)
					return nil
				},
			)
		}

		instance := m.list.GetInstances()[m.list.NumInstances()-1]
		switch msg.Type {
		// Start the instance (enable previews etc) and go back to the main menu state.
		case tea.KeyEnter:
			if len(instance.Title) == 0 {
				return m, m.handleError(fmt.Errorf("title cannot be empty"))
			}

			if err := instance.Start(true); err != nil {
				m.list.Kill()
				m.state = stateDefault
				return m, m.handleError(err)
			}
			// Save after adding new instance
			if err := m.storage.SaveInstances(m.list.GetInstances()); err != nil {
				return m, m.handleError(err)
			}
			// Instance added successfully, call the finalizer.
			m.newInstanceFinalizer()
			if m.autoYes {
				instance.AutoYes = true
			}

			m.newInstanceFinalizer()
			m.state = stateDefault
			if m.promptAfterName {
				m.state = statePrompt
				m.menu.SetState(ui.StatePrompt)
				// Initialize the text input overlay
				m.textInputOverlay = overlay.NewTextInputOverlay("Enter prompt", "")
				m.promptAfterName = false
			} else {
				m.menu.SetState(ui.StateDefault)
				m.showHelpScreen(helpTypeInstanceStart, nil)
			}

			return m, tea.Batch(tea.WindowSize(), m.instanceChanged())
		case tea.KeyRunes:
			if len(instance.Title) >= 32 {
				return m, m.handleError(fmt.Errorf("title cannot be longer than 32 characters"))
			}
			if err := instance.SetTitle(instance.Title + string(msg.Runes)); err != nil {
				return m, m.handleError(err)
			}
		case tea.KeyBackspace:
			if len(instance.Title) == 0 {
				return m, nil
			}
			if err := instance.SetTitle(instance.Title[:len(instance.Title)-1]); err != nil {
				return m, m.handleError(err)
			}
		case tea.KeySpace:
			if err := instance.SetTitle(instance.Title + " "); err != nil {
				return m, m.handleError(err)
			}
		case tea.KeyEsc:
			m.list.Kill()
			m.state = stateDefault
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
	} else if m.state == statePrompt {
		// Use the new TextInputOverlay component to handle all key events
		shouldClose := m.textInputOverlay.HandleKeyPress(msg)

		// Check if the form was submitted or canceled
		if shouldClose {
			if m.textInputOverlay.IsSubmitted() {
				// Form was submitted, process the input
				selected := m.list.GetSelectedInstance()
				if selected == nil {
					return m, nil
				}
				if err := selected.SendPrompt(m.textInputOverlay.GetValue()); err != nil {
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
					m.showHelpScreen(helpTypeInstanceStart, nil)
					return nil
				},
			)
		}

		return m, nil
	}

	// Handle confirmation state
	if m.state == stateConfirm {
		shouldClose := m.confirmationOverlay.HandleKeyPress(msg)
		if shouldClose {
			m.state = stateDefault
			m.confirmationOverlay = nil
			return m, nil
		}
		return m, nil
	}

	// Handle add project state
	if m.state == stateAddProject {
		// Handle escape to cancel
		if msg.String() == "ctrl+c" || msg.Type == tea.KeyEsc {
			m.state = stateDefault
			m.projectInputOverlay.Hide()
			return m, tea.WindowSize()
		}

		// Update the project input overlay
		var cmd tea.Cmd
		m.projectInputOverlay, cmd = m.projectInputOverlay.Update(msg)

		// Handle enter to submit
		if msg.Type == tea.KeyEnter {
			path := m.projectInputOverlay.GetValue()
			if path != "" {
				// Try to add the project
				if err := m.projectManager.ValidateProjectPath(path); err != nil {
					m.projectInputOverlay.SetError(err.Error())
					return m, cmd
				}

				// Add the project
				projectName := "" // Let NewProject generate name from path
				project, err := m.projectManager.AddProject(path, projectName)
				if err != nil {
					m.projectInputOverlay.SetError(err.Error())
					return m, cmd
				}

				// Update project history
				if err := m.projectManager.UpdateProjectHistory(path); err != nil {
					log.WarningLog.Printf("Failed to update project history: %v", err)
				}

				// Success - hide overlay and return to default state
				m.state = stateDefault
				m.projectInputOverlay.Hide()

				// Optional: Set the new project as active
				m.projectManager.SetActiveProject(project.ID)

				return m, tea.WindowSize()
			}
		}

		return m, cmd
	}

	// Handle MCP management state
	if m.state == stateMCPManage {
		// Handle escape to cancel
		if msg.String() == "ctrl+c" || msg.Type == tea.KeyEsc {
			m.state = stateDefault
			m.mcpOverlay = nil
			m.list.UpdateConfig() // Refresh MCP config for display
			return m, tea.WindowSize()
		}

		// Let the MCP overlay handle the key press
		if m.mcpOverlay != nil {
			shouldClose := m.mcpOverlay.HandleKeyPress(msg)
			if shouldClose || m.mcpOverlay.IsSubmitted() || m.mcpOverlay.IsCanceled() {
				// Check if we need to restart Claude due to MCP changes
				if m.mcpOverlay.IsSubmitted() && m.mcpOverlay.AssignmentsChanged() {
					instance := m.mcpOverlay.GetInstance()
					if instance != nil && instance.Started() && !instance.Paused() {
						// Check if this is a Claude instance (only Claude needs restart for MCP changes)
						if isClaudeInstance(instance) {
							// Create restart command
							restartCmd := func() tea.Msg {
								if err := instance.Restart(); err != nil {
									return fmt.Errorf("failed to restart Claude with new MCP configuration: %w", err)
								}
								return instanceChangedMsg{}
							}

							// Auto-restart without confirmation using --continue
							m.state = stateDefault
							m.mcpOverlay = nil
							m.list.UpdateConfig() // Refresh MCP config for display
							return m, func() tea.Msg { return restartCmd() }
						}
					}
				}

				m.state = stateDefault
				m.mcpOverlay = nil
				m.list.UpdateConfig() // Refresh MCP config for display
				return m, tea.WindowSize()
			}
		}

		return m, nil
	}

	// Handle project history state
	if m.state == stateProjectHistory {
		// Handle escape to cancel
		if msg.String() == "ctrl+c" || msg.Type == tea.KeyEsc {
			m.state = stateDefault
			m.projectHistoryOverlay = nil
			return m, tea.WindowSize()
		}

		// Let the project history overlay handle the key press
		if m.projectHistoryOverlay != nil {
			shouldClose := m.projectHistoryOverlay.HandleKeyPress(msg)
			if shouldClose || m.projectHistoryOverlay.IsSubmitted() || m.projectHistoryOverlay.IsCanceled() {
				selectedPath := m.projectHistoryOverlay.GetSelectedPath()

				m.state = stateDefault
				m.projectHistoryOverlay = nil

				// Handle the selected path
				if m.projectHistoryOverlay.IsSubmitted() && selectedPath != "" {
					if selectedPath == "NEW_MANUAL" {
						// User chose "new manual" - show the add project overlay
						m.state = stateAddProject
						m.projectInputOverlay.Show()
						return m, tea.WindowSize()
					} else {
						// User selected an existing project path
						// Update history to move this to the front
						m.projectManager.UpdateProjectHistory(selectedPath)

						// Try to add as a project if not already added
						projectName := "" // Let NewProject generate name from path
						if project, err := m.projectManager.AddProject(selectedPath, projectName); err == nil {
							// Set as active project
							m.projectManager.SetActiveProject(project.ID)
						} else {
							// Project might already exist, just update history
							log.InfoLog.Printf("Project already exists or error adding: %v", err)
						}

						return m, tea.WindowSize()
					}
				}

				return m, tea.WindowSize()
			}
		}

		return m, nil
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
		return m.showHelpScreen(helpTypeGeneral, nil)
	case keys.KeyPrompt:
		if m.list.NumInstances() >= GlobalInstanceLimit {
			return m, m.handleError(
				fmt.Errorf("you can't create more than %d instances", GlobalInstanceLimit))
		}
		// Get active project path, default to current directory
		projectPath := "."
		var projectID string
		if activeProject := m.projectManager.GetActiveProject(); activeProject != nil {
			projectPath = activeProject.Path
			projectID = activeProject.ID
			// Update project history when using active project
			if err := m.projectManager.UpdateProjectHistory(projectPath); err != nil {
				log.WarningLog.Printf("Failed to update project history: %v", err)
			}
		}
		instance, err := session.NewInstance(session.InstanceOptions{
			Title:   "",
			Path:    projectPath,
			Program: m.program,
		})
		if err == nil {
			instance.ProjectID = projectID
			// Add instance to project if there's an active project
			if projectID != "" {
				m.projectManager.AddInstanceToProject(projectID, instance.Title)
			}
		}
		if err != nil {
			return m, m.handleError(err)
		}

		m.newInstanceFinalizer = m.list.AddInstance(instance)
		m.list.SetSelectedInstance(m.list.NumInstances() - 1)
		m.state = stateNew
		m.menu.SetState(ui.StateNewInstance)
		m.promptAfterName = true

		return m, nil
	case keys.KeyNew:
		if m.list.NumInstances() >= GlobalInstanceLimit {
			return m, m.handleError(
				fmt.Errorf("you can't create more than %d instances", GlobalInstanceLimit))
		}
		// Get active project path, default to current directory
		projectPath := "."
		var projectID string
		if activeProject := m.projectManager.GetActiveProject(); activeProject != nil {
			projectPath = activeProject.Path
			projectID = activeProject.ID
			// Update project history when using active project
			if err := m.projectManager.UpdateProjectHistory(projectPath); err != nil {
				log.WarningLog.Printf("Failed to update project history: %v", err)
			}
		}
		instance, err := session.NewInstance(session.InstanceOptions{
			Title:   "",
			Path:    projectPath,
			Program: m.program,
		})
		if err == nil {
			instance.ProjectID = projectID
			// Add instance to project if there's an active project
			if projectID != "" {
				m.projectManager.AddInstanceToProject(projectID, instance.Title)
			}
		}
		if err != nil {
			return m, m.handleError(err)
		}

		m.newInstanceFinalizer = m.list.AddInstance(instance)
		m.list.SetSelectedInstance(m.list.NumInstances() - 1)
		m.state = stateNew
		m.menu.SetState(ui.StateNewInstance)

		return m, nil
	case keys.KeyAddProject:
		// Show project input overlay
		m.state = stateAddProject
		m.projectInputOverlay.Show()
		return m, tea.WindowSize()
	case keys.KeyMCPManage:
		// Show MCP management overlay for selected instance
		selectedInstance := m.list.GetSelectedInstance()
		m.state = stateMCPManage
		m.mcpOverlay = overlay.NewMCPOverlay(selectedInstance)
		return m, tea.WindowSize()
	case keys.KeyProjectHistory:
		// Show project history overlay
		m.state = stateProjectHistory
		m.projectHistoryOverlay = overlay.NewProjectHistoryOverlay(m.projectManager)
		return m, tea.WindowSize()
	case keys.KeyUp:
		m.list.Up()
		return m, m.instanceChanged()
	case keys.KeyDown:
		m.list.Down()
		return m, m.instanceChanged()
	case keys.KeyShiftUp:
		m.tabbedWindow.ScrollUp()
		return m, m.instanceChanged()
	case keys.KeyShiftDown:
		m.tabbedWindow.ScrollDown()
		return m, m.instanceChanged()
	case keys.KeyCtrlShiftUp:
		m.tabbedWindow.FastScrollUp()
		return m, m.instanceChanged()
	case keys.KeyCtrlShiftDown:
		m.tabbedWindow.FastScrollDown()
		return m, m.instanceChanged()
	case keys.KeyTab:
		m.tabbedWindow.Toggle()
		m.menu.SetInDiffTab(m.tabbedWindow.IsInDiffTab())
		m.menu.SetInConsoleTab(m.tabbedWindow.IsInConsoleTab())
		return m, m.instanceChanged()
	case keys.KeyKill:
		selected := m.list.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}

		// Create the kill action as a tea.Cmd
		killAction := func() tea.Msg {
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

			// Delete from storage first
			if err := m.storage.DeleteInstance(selected.Title); err != nil {
				return err
			}

			// Then kill the instance
			m.list.Kill()
			return instanceChangedMsg{}
		}

		// Show confirmation modal
		message := fmt.Sprintf("[!] Kill session '%s'?", selected.Title)
		return m, m.confirmAction(message, killAction)
	case keys.KeySubmit:
		selected := m.list.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}

		// Create the push action as a tea.Cmd
		pushAction := func() tea.Msg {
			// Default commit message with timestamp
			commitMsg := fmt.Sprintf("[claudesquad] update from '%s' on %s", selected.Title, time.Now().Format(time.RFC822))
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
	case keys.KeyCheckout:
		selected := m.list.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}

		// Show help screen before pausing
		m.showHelpScreen(helpTypeInstanceCheckout, func() {
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
		if err := selected.Resume(); err != nil {
			return m, m.handleError(err)
		}
		return m, tea.WindowSize()
	case keys.KeyEnter:
		if m.list.NumInstances() == 0 {
			return m, nil
		}
		selected := m.list.GetSelectedInstance()
		if selected == nil || selected.Paused() {
			return m, nil
		}

		// Handle console tab attachment
		if m.tabbedWindow.IsInConsoleTab() {
			if !selected.ConsoleAlive() {
				return m, m.handleError(fmt.Errorf("console session not available"))
			}
			// Show help screen before attaching to console
			m.showHelpScreen(helpTypeInstanceAttach, func() {
				ch, err := selected.AttachToConsole()
				if err != nil {
					m.handleError(err)
					return
				}
				<-ch
				m.state = stateDefault
			})
			return m, nil
		}

		// Handle regular instance attachment
		if !selected.TmuxAlive() {
			return m, nil
		}
		// Show help screen before attaching
		m.showHelpScreen(helpTypeInstanceAttach, func() {
			ch, err := m.list.Attach()
			if err != nil {
				m.handleError(err)
				return
			}
			<-ch
			m.state = stateDefault
		})
		return m, nil
	default:
		return m, nil
	}
}

// instanceChanged updates the preview pane, menu, diff pane, and console pane based on the selected instance. It returns an error
// Cmd if there was any error.
func (m *home) instanceChanged() tea.Cmd {
	// selected may be nil
	selected := m.list.GetSelectedInstance()

	m.tabbedWindow.UpdateDiff(selected)
	// Update menu with current instance
	m.menu.SetInstance(selected)

	// Update preview pane
	if err := m.tabbedWindow.UpdatePreview(selected); err != nil {
		return m.handleError(err)
	}

	// Update console pane
	if err := m.tabbedWindow.UpdateConsole(selected); err != nil {
		return m.handleError(err)
	}

	// Save instances after changes (including restart/MCP updates)
	if err := m.storage.SaveInstances(m.list.GetInstances()); err != nil {
		return m.handleError(err)
	}

	return nil
}

type keyupMsg struct{}

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

// hideErrMsg implements tea.Msg and clears the error text from the screen.
type hideErrMsg struct{}

// previewTickMsg implements tea.Msg and triggers a preview update
type previewTickMsg struct{}

type tickUpdateMetadataMessage struct{}

type instanceChangedMsg struct{}

// tickUpdateMetadataCmd is the callback to update the metadata of the instances every 500ms. Note that we iterate
// overall the instances and capture their output. It's a pretty expensive operation. Let's do it 2x a second only.
var tickUpdateMetadataCmd = func() tea.Msg {
	time.Sleep(500 * time.Millisecond)
	return tickUpdateMetadataMessage{}
}

// handleError handles all errors which get bubbled up to the app. sets the error message. We return a callback tea.Cmd that returns a hideErrMsg message
// which clears the error message after 3 seconds.
func (m *home) handleError(err error) tea.Cmd {
	log.ErrorLog.Printf("%v", err)
	m.errBox.SetError(err)
	return func() tea.Msg {
		select {
		case <-m.ctx.Done():
		case <-time.After(3 * time.Second):
		}

		return hideErrMsg{}
	}
}

// isClaudeInstance checks if the given instance is running Claude
func isClaudeInstance(instance *session.Instance) bool {
	if instance == nil {
		return false
	}

	// Check if the program contains "claude" (case-insensitive)
	program := strings.ToLower(instance.Program)
	return strings.Contains(program, "claude")
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
		// Execute the action if it exists
		if action != nil {
			_ = action()
		}
	}

	m.confirmationOverlay.OnCancel = func() {
		m.state = stateDefault
	}

	return nil
}

func (m *home) View() string {
	listWithPadding := lipgloss.NewStyle().PaddingTop(1).Render(m.list.String())
	previewWithPadding := lipgloss.NewStyle().PaddingTop(1).Render(m.tabbedWindow.String())
	listAndPreview := lipgloss.JoinHorizontal(lipgloss.Top, listWithPadding, previewWithPadding)

	mainView := lipgloss.JoinVertical(
		lipgloss.Center,
		listAndPreview,
		m.menu.String(),
		m.errBox.String(),
	)

	if m.state == statePrompt {
		if m.textInputOverlay == nil {
			log.ErrorLog.Printf("text input overlay is nil")
		}
		return overlay.PlaceOverlay(0, 0, m.textInputOverlay.Render(), mainView, true, true)
	} else if m.state == stateHelp {
		if m.textOverlay == nil {
			log.ErrorLog.Printf("text overlay is nil")
		}
		return overlay.PlaceOverlay(0, 0, m.textOverlay.Render(), mainView, true, true)
	} else if m.state == stateConfirm {
		if m.confirmationOverlay == nil {
			log.ErrorLog.Printf("confirmation overlay is nil")
		}
		return overlay.PlaceOverlay(0, 0, m.confirmationOverlay.Render(), mainView, true, true)
	} else if m.state == stateAddProject {
		if m.projectInputOverlay == nil {
			log.ErrorLog.Printf("project input overlay is nil")
		}
		return overlay.PlaceOverlay(0, 0, m.projectInputOverlay.View(), mainView, true, true)
	} else if m.state == stateMCPManage {
		if m.mcpOverlay == nil {
			log.ErrorLog.Printf("MCP overlay is nil")
		}
		return overlay.PlaceOverlay(0, 0, m.mcpOverlay.Render(), mainView, true, true)
	} else if m.state == stateProjectHistory {
		if m.projectHistoryOverlay == nil {
			log.ErrorLog.Printf("project history overlay is nil")
		}
		return overlay.PlaceOverlay(0, 0, m.projectHistoryOverlay.Render(), mainView, true, true)
	}

	return mainView
}
