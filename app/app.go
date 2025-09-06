package app

import (
	"claude-squad/config"
	"claude-squad/keys"
	"claude-squad/log"
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
	// stateBranch is the state when the user is entering branch information.
	stateBranch
	// stateSourceBranch is the state when the user is selecting the source branch.
	stateSourceBranch
	// stateSyncConfirm is the state when asking user to sync with remote branch.
	stateSyncConfirm
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
	// branchAfterName tracks if we should enter branch input mode after naming
	branchAfterName bool
	// pendingBranchName stores the branch name while we collect source branch info
	pendingBranchName string
	// pendingSourceBranch stores the source branch while we handle sync confirmation
	pendingSourceBranch string

	// keySent is used to manage underlining menu items
	keySent bool

	// -- UI Components --

	// list displays the list of instances
	list *ui.List
	// menu displays the bottom menu
	menu *ui.Menu
	// tabbedWindow displays the tabbed window with preview and diff panes
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

	h := &home{
		ctx:          ctx,
		spinner:      spinner.New(spinner.WithSpinner(spinner.MiniDot)),
		menu:         ui.NewMenu(),
		tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewDiffPane()),
		errBox:       ui.NewErrBox(),
		storage:      storage,
		appConfig:    appConfig,
		program:      program,
		autoYes:      autoYes,
		state:        stateDefault,
		appState:     appState,
	}
	h.list = ui.NewList(&h.spinner, autoYes)

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
		// Handle mouse wheel events for scrolling the diff/preview pane
		if msg.Action == tea.MouseActionPress {
			if msg.Button == tea.MouseButtonWheelDown || msg.Button == tea.MouseButtonWheelUp {
				selected := m.list.GetSelectedInstance()
				if selected == nil || selected.Status == session.Paused {
					return m, nil
				}

				switch msg.Button {
				case tea.MouseButtonWheelUp:
					m.tabbedWindow.ScrollUp()
				case tea.MouseButtonWheelDown:
					m.tabbedWindow.ScrollDown()
				}
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
	if m.state == statePrompt || m.state == stateHelp || m.state == stateConfirm {
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
	if name == keys.KeyShiftDown || name == keys.KeyShiftUp {
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

func (m *home) handleKeyPress(msg tea.KeyMsg) (mod tea.Model, cmd tea.Cmd) {
	cmd, returnEarly := m.handleMenuHighlighting(msg)
	if returnEarly {
		return m, cmd
	}

	if m.state == stateHelp {
		return m.handleHelpState(msg)
	}

	switch m.state {
	case stateNew:
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

			// Only start the instance if we don't need to collect additional info
			if !m.branchAfterName && !m.promptAfterName {
				if err := instance.Start(true); err != nil {
					m.list.Kill()
					m.state = stateDefault
					return m, m.handleError(err)
				}
			}
			if m.promptAfterName {
				m.state = statePrompt
				m.menu.SetState(ui.StatePrompt)
				// Initialize the text input overlay
				m.textInputOverlay = overlay.NewTextInputOverlay("Enter prompt", "")
				m.promptAfterName = false
			} else if m.branchAfterName {
				m.state = stateBranch
				m.menu.SetState(ui.StateNewInstance)
				// Initialize the text input overlay for branch input
				m.textInputOverlay = overlay.NewTextInputOverlay("Enter branch name (e.g., feat/new-feature, fix/bug-123)", "")
				m.branchAfterName = false
			} else {
				// Instance is complete, finalize it
				m.newInstanceFinalizer()
				if m.autoYes {
					instance.AutoYes = true
				}
				m.state = stateDefault
				m.menu.SetState(ui.StateDefault)
				m.showHelpScreen(helpStart(instance), nil)
			}

			// Save after adding new instance
			if err := m.storage.SaveInstances(m.list.GetInstances()); err != nil {
				return m, m.handleError(err)
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
	case statePrompt:
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
	case stateBranch:
		// Handle branch input state using TextInputOverlay
		shouldClose := m.textInputOverlay.HandleKeyPress(msg)

		// Check if the form was submitted or canceled
		if shouldClose {
			selected := m.list.GetSelectedInstance()
			if selected == nil {
				return m, nil
			}

			wasSubmitted := m.textInputOverlay.IsSubmitted()

			if wasSubmitted {
				// Parse the branch name (simplified - no "from" parsing)
				branchName, err := m.simplifyBranchInput(m.textInputOverlay.GetValue())
				if err != nil {
					return m, m.handleError(err)
				}

				// Store the branch name temporarily and move to source branch selection
				m.pendingBranchName = branchName
				m.state = stateSourceBranch
				m.menu.SetState(ui.StateNewInstance)

				// Get default source branch
				defaultSource := m.getDefaultSourceBranch()

				// Initialize the text input overlay for source branch selection
				m.textInputOverlay = overlay.NewTextInputOverlay("Enter source branch", defaultSource)

				return m, nil
			} else {
				// User canceled - remove the instance
				m.list.Kill()
			}

			// Close the overlay and reset state
			m.textInputOverlay = nil
			m.state = stateDefault
			m.menu.SetState(ui.StateDefault)

			return m, tea.Batch(tea.WindowSize(), m.instanceChanged())
		}

		return m, nil
	case stateSourceBranch:
		// Handle source branch input state using TextInputOverlay
		shouldClose := m.textInputOverlay.HandleKeyPress(msg)

		// Check if the form was submitted or canceled
		if shouldClose {
			selected := m.list.GetSelectedInstance()
			if selected == nil {
				return m, nil
			}

			wasSubmitted := m.textInputOverlay.IsSubmitted()

			if wasSubmitted {
				// Get the source branch (with default if empty)
				sourceBranch := strings.TrimSpace(m.textInputOverlay.GetValue())
				if sourceBranch == "" {
					sourceBranch = m.getDefaultSourceBranch()
				}

				// Store source branch for potential sync confirmation
				m.pendingSourceBranch = sourceBranch

				// First check if the SOURCE branch needs sync with remote origin
				sourceExists, sourceNeedsSync, err := session.CheckRemoteBranchStatic(".", sourceBranch)
				if err != nil {
					return m, m.handleError(fmt.Errorf("failed to check remote source branch: %w", err))
				}

				if sourceExists && sourceNeedsSync {
					// Source branch exists on remote and differs - ask user if they want to sync source first
					m.state = stateSyncConfirm
					message := fmt.Sprintf("Source branch '%s' is out of sync with remote. Sync before creating new branch? (y/n)", sourceBranch)
					m.confirmationOverlay = overlay.NewConfirmationOverlay(message)
					m.confirmationOverlay.SetWidth(70)

					m.confirmationOverlay.OnConfirm = func() {
						// Sync source branch then proceed to check target branch
						m.syncSourceThenCheckTarget(selected, sourceBranch)
					}
					m.confirmationOverlay.OnCancel = func() {
						// Skip source sync, proceed to check target branch
						m.checkTargetBranchAndProceed(selected, sourceBranch)
					}

					// Close text input overlay
					m.textInputOverlay = nil
					return m, nil
				} else {
					// Source branch doesn't need sync, check target branch
					return m.checkTargetBranchAndProceed(selected, sourceBranch)
				}
			} else {
				// User canceled - remove the instance
				m.list.Kill()
			}

			// Close the overlay and reset state
			m.textInputOverlay = nil
			m.state = stateDefault
			m.menu.SetState(ui.StateDefault)
			m.pendingBranchName = "" // Clear the pending branch name

			if wasSubmitted {
				m.showHelpScreen(helpStart(selected), nil)
			}

			return m, tea.Batch(tea.WindowSize(), m.instanceChanged())
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

	// Handle sync confirmation state
	if m.state == stateSyncConfirm {
		shouldClose := m.confirmationOverlay.HandleKeyPress(msg)
		if shouldClose {
			m.state = stateDefault
			m.confirmationOverlay = nil
			return m, nil
		}
		return m, nil
	}

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
		if m.list.NumInstances() >= GlobalInstanceLimit {
			return m, m.handleError(
				fmt.Errorf("you can't create more than %d instances", GlobalInstanceLimit))
		}
		instance, err := session.NewInstance(session.InstanceOptions{
			Title:   "",
			Path:    ".",
			Program: m.program,
		})
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
		instance, err := session.NewInstance(session.InstanceOptions{
			Title:   "",
			Path:    ".",
			Program: m.program,
		})
		if err != nil {
			return m, m.handleError(err)
		}

		m.newInstanceFinalizer = m.list.AddInstance(instance)
		m.list.SetSelectedInstance(m.list.NumInstances() - 1)
		m.state = stateNew
		m.menu.SetState(ui.StateNewInstance)

		return m, nil
	case keys.KeyBranch:
		if m.list.NumInstances() >= GlobalInstanceLimit {
			return m, m.handleError(
				fmt.Errorf("you can't create more than %d instances", GlobalInstanceLimit))
		}
		instance, err := session.NewInstance(session.InstanceOptions{
			Title:   "",
			Path:    ".",
			Program: m.program,
		})
		if err != nil {
			return m, m.handleError(err)
		}

		m.newInstanceFinalizer = m.list.AddInstance(instance)
		m.list.SetSelectedInstance(m.list.NumInstances() - 1)
		m.state = stateNew
		m.menu.SetState(ui.StateNewInstance)
		// Set flags to indicate we want branch input after naming
		m.promptAfterName = false
		m.branchAfterName = true

		return m, nil
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
	case keys.KeyTab:
		m.tabbedWindow.Toggle()
		m.menu.SetInDiffTab(m.tabbedWindow.IsInDiffTab())
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
		m.showHelpScreen(helpTypeInstanceCheckout{}, func() {
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
		if selected == nil || selected.Paused() || !selected.TmuxAlive() {
			return m, nil
		}
		// Show help screen before attaching
		m.showHelpScreen(helpTypeInstanceAttach{}, func() {
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

// instanceChanged updates the preview pane, menu, and diff pane based on the selected instance. It returns an error
// Cmd if there was any error.
func (m *home) instanceChanged() tea.Cmd {
	// selected may be nil
	selected := m.list.GetSelectedInstance()

	m.tabbedWindow.UpdateDiff(selected)
	m.tabbedWindow.SetInstance(selected)
	// Update menu with current instance
	m.menu.SetInstance(selected)

	// If there's no selected instance, we don't need to update the preview.
	if err := m.tabbedWindow.UpdatePreview(selected); err != nil {
		return m.handleError(err)
	}
	return nil
}

// parseBranchInput parses user input like "feat/new-feature from main"
// Returns: branchName, sourceBranch, error
func (m *home) parseBranchInput(input string) (string, string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", fmt.Errorf("branch name cannot be empty")
	}

	parts := strings.Split(input, " from ")
	branchName := strings.TrimSpace(parts[0])
	var sourceBranch string

	// Check for multiple from clauses first
	if len(parts) > 2 {
		return "", "", fmt.Errorf("invalid format: use 'branch-name' or 'branch-name from source-branch'")
	}

	// Handle source branch if provided
	if len(parts) == 2 {
		sourceBranch = strings.TrimSpace(parts[1])
		if sourceBranch == "" {
			return "", "", fmt.Errorf("source branch cannot be empty")
		}
	}

	// Validate branch name format
	if err := m.validateBranchName(branchName); err != nil {
		return "", "", err
	}

	return branchName, sourceBranch, nil
}

// validateBranchName validates git branch name format
func (m *home) validateBranchName(branchName string) error {
	if branchName == "" {
		return fmt.Errorf("branch name cannot be empty")
	}

	// Basic git branch name validation
	if strings.HasPrefix(branchName, "-") || strings.HasPrefix(branchName, "/") || strings.HasPrefix(branchName, ".") {
		return fmt.Errorf("branch name cannot start with -, /, or")
	}

	if strings.HasSuffix(branchName, ".lock") {
		return fmt.Errorf("branch name cannot end with .lock")
	}

	if strings.Contains(branchName, "..") || strings.Contains(branchName, "@{") {
		return fmt.Errorf("branch name cannot contain .. or @{")
	}

	// Check for invalid characters (spaces and some special chars)
	if strings.ContainsAny(branchName, " \t\n\r\f\v~^:?*[\\") {
		return fmt.Errorf("branch name contains invalid characters")
	}

	return nil
}

// getDefaultSourceBranch detects the default source branch (main, master, develop, dev)
func (m *home) getDefaultSourceBranch() string {
	// Common default branch names in order of preference
	// defaultBranches := []string{"main", "master", "develop", "dev"}

	// For now, return "main" as default - in a full implementation,
	// we would check git remote branches to see which exists
	// TODO: Add git remote branch checking logic
	return "main"
}

// simplifyBranchInput removes complex parsing and just asks for branch name
func (m *home) simplifyBranchInput(input string) (string, error) {
	branchName := strings.TrimSpace(input)
	if branchName == "" {
		return "", fmt.Errorf("branch name cannot be empty")
	}

	// Validate branch name format
	if err := m.validateBranchName(branchName); err != nil {
		return "", err
	}

	return branchName, nil
}

// finalizeBranchCreation completes the branch creation process without sync
func (m *home) finalizeBranchCreation(selected *session.Instance, sourceBranch string) (tea.Model, tea.Cmd) {
	// Only set the branch name as the instance title if no title was provided
	// This preserves user-entered session names from the 'b' key workflow
	if selected.Title == "" {
		if err := selected.SetTitle(m.pendingBranchName); err != nil {
			return m, m.handleError(err)
		}
	}

	// Store the custom branch and source branch information
	selected.CustomBranch = m.pendingBranchName
	selected.SourceBranch = sourceBranch

	// Start the instance
	if err := selected.Start(true); err != nil {
		m.list.Kill()
		m.state = stateDefault
		return m, m.handleError(err)
	}

	// Save after adding new instance
	if err := m.storage.SaveInstances(m.list.GetInstances()); err != nil {
		return m, m.handleError(err)
	}

	// Instance added successfully
	m.newInstanceFinalizer()
	if m.autoYes {
		selected.AutoYes = true
	}

	// Reset state
	m.state = stateDefault
	m.menu.SetState(ui.StateDefault)
	m.pendingBranchName = ""
	m.pendingSourceBranch = ""

	m.showHelpScreen(helpStart(selected), nil)
	return m, tea.Batch(tea.WindowSize(), m.instanceChanged())
}

// syncSourceThenCheckTarget syncs the source branch and then checks target branch
func (m *home) syncSourceThenCheckTarget(selected *session.Instance, sourceBranch string) {
	// Sync source branch with remote
	if err := session.SyncWithRemoteBranchStatic(".", sourceBranch); err != nil {
		m.handleError(fmt.Errorf("failed to sync source branch: %w", err))
		return
	}

	// Now check target branch and proceed
	_, _ = m.checkTargetBranchAndProceed(selected, sourceBranch)
}

// checkTargetBranchAndProceed checks if target branch needs sync and proceeds with creation
func (m *home) checkTargetBranchAndProceed(selected *session.Instance, sourceBranch string) (tea.Model, tea.Cmd) {
	// Check if the TARGET branch exists on remote and needs sync
	exists, needsSync, err := session.CheckRemoteBranchStatic(".", m.pendingBranchName)
	if err != nil {
		return m, m.handleError(fmt.Errorf("failed to check remote target branch: %w", err))
	}

	if exists && needsSync {
		// Target branch exists and differs - ask user if they want to sync
		m.state = stateSyncConfirm
		message := fmt.Sprintf("Target branch '%s' has different commits. Sync before creating? (y/n)", m.pendingBranchName)
		m.confirmationOverlay = overlay.NewConfirmationOverlay(message)
		m.confirmationOverlay.SetWidth(60)

		m.confirmationOverlay.OnConfirm = func() {
			m.finalizeBranchCreationWithSync(selected, true)
		}
		m.confirmationOverlay.OnCancel = func() {
			m.finalizeBranchCreationWithSync(selected, false)
		}

		return m, nil
	} else {
		// No remote target branch or no sync needed, proceed directly
		return m.finalizeBranchCreation(selected, sourceBranch)
	}
}

// finalizeBranchCreationWithSync completes branch creation with optional remote sync
func (m *home) finalizeBranchCreationWithSync(selected *session.Instance, shouldSync bool) {
	if shouldSync {
		// Sync with remote branch first using static method
		if err := session.SyncWithRemoteBranchStatic(".", m.pendingBranchName); err != nil {
			m.handleError(fmt.Errorf("failed to sync with remote: %w", err))
			return
		}
	}

	// Proceed with normal branch creation (ignore return values since this is a callback)
	_, _ = m.finalizeBranchCreation(selected, m.pendingSourceBranch)
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

	switch m.state {
	case statePrompt:
		if m.textInputOverlay == nil {
			log.ErrorLog.Printf("text input overlay is nil")
		}
		return overlay.PlaceOverlay(0, 0, m.textInputOverlay.Render(), mainView, true, true)
	case stateBranch:
		if m.textInputOverlay == nil {
			log.ErrorLog.Printf("text input overlay is nil")
		}
		return overlay.PlaceOverlay(0, 0, m.textInputOverlay.Render(), mainView, true, true)
	case stateSourceBranch:
		if m.textInputOverlay == nil {
			log.ErrorLog.Printf("text input overlay is nil")
		}
		return overlay.PlaceOverlay(0, 0, m.textInputOverlay.Render(), mainView, true, true)
	case stateHelp:
		if m.textOverlay == nil {
			log.ErrorLog.Printf("text overlay is nil")
		}
		return overlay.PlaceOverlay(0, 0, m.textOverlay.Render(), mainView, true, true)
	case stateConfirm:
		if m.confirmationOverlay == nil {
			log.ErrorLog.Printf("confirmation overlay is nil")
		}
		return overlay.PlaceOverlay(0, 0, m.confirmationOverlay.Render(), mainView, true, true)
	case stateSyncConfirm:
		if m.confirmationOverlay == nil {
			log.ErrorLog.Printf("sync confirmation overlay is nil")
		}
		return overlay.PlaceOverlay(0, 0, m.confirmationOverlay.Render(), mainView, true, true)
	}

	return mainView
}
