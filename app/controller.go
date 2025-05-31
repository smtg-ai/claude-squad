package app

import (
	"claude-squad/instance/orchestrator"
	"claude-squad/keys"
	"claude-squad/log"
	"claude-squad/ui"
	"claude-squad/ui/overlay"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Agent interface {
	StatusText() string
	MenuItems() []keys.KeyName
}

type orchestratorState int

const (
	// orchestratorStateDefault is the default state for orchestrator
	orchestratorStateDefault orchestratorState = iota
	// orchestratorStatePrompt is the state when the user is entering a prompt for orchestrator
	orchestratorStatePrompt
	// orchestratorStatePlan is the state when the orchestrator plan is being displayed
	orchestratorStatePlan
)

// controller manages instances and orchestrators
type controller struct {
	// newInstanceFinalizer is the finalizer for new instance
	newInstanceFinalizer func()
	// promptAfterName is whether to prompt after name
	promptAfterName bool
	// orchestratorState is the state of the orchestrator
	orchestratorState orchestratorState

	// agents is the list of agents being managed
	agents []Agent

	// UI components
	list             *ui.List
	tabbedWindow     *ui.TabbedWindow
	textInputOverlay *overlay.TextInputOverlay
	textOverlay      *overlay.TextOverlay
}

func newController(spinner *spinner.Model, autoYes bool) *controller {
	return &controller{
		list:         ui.NewList(spinner, autoYes),
		tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewDiffPane()),
	}
}

// LoadExistingInstances loads instances from storage into the list
func (im *controller) LoadExistingInstances(h *home) error {
	instances, err := h.storage.LoadInstances()
	if err != nil {
		return err
	}

	for _, instance := range instances {
		finalizer := im.list.AddInstance(instance)
		finalizer() // Call finalizer immediately since instance is already started
	}

	return nil
}

func (im *controller) Render(h *home) string {
	listWithPadding := lipgloss.NewStyle().PaddingTop(1).Render(im.list.String())
	previewWithPadding := lipgloss.NewStyle().PaddingTop(1).Render(im.tabbedWindow.String())
	listAndPreview := lipgloss.JoinHorizontal(lipgloss.Top, listWithPadding, previewWithPadding)

	mainView := lipgloss.JoinVertical(
		lipgloss.Center,
		listAndPreview,
		h.menu.String(),
		h.errBox.String(),
	)

	if h.state == tuiStatePrompt {
		if im.textInputOverlay == nil {
			log.ErrorLog.Printf("text input overlay is nil")
		}
		return overlay.PlaceOverlay(0, 0, im.textInputOverlay.Render(), mainView, true, true)
	} else if h.state == tuiStateHelp {
		if im.textOverlay == nil {
			log.ErrorLog.Printf("text overlay is nil")
		}
		return overlay.PlaceOverlay(0, 0, im.textOverlay.Render(), mainView, true, true)
	}

	return mainView
}

func (im *controller) Update(h *home, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case hideErrMsg:
		h.errBox.Clear()
	case previewTickMsg:
		cmd := im.instanceChanged(h)
		return h, tea.Batch(
			cmd,
			func() tea.Msg {
				time.Sleep(100 * time.Millisecond)
				return previewTickMsg{}
			},
		)
	case keyupMsg:
		h.menu.ClearKeydown()
		return h, nil
	case tickUpdateMetadataMessage:
		return h, im.handleMetadataUpdate(h)
	case tea.MouseMsg:
		return im.handleMouseEvent(h, msg)
	case tea.KeyMsg:
		return im.handleKeyEvent(h, msg)
	case tea.WindowSizeMsg:
		h.updateHandleWindowSizeEvent(msg)
		return h, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		h.spinner, cmd = h.spinner.Update(msg)
		return h, cmd
	}
	return h, nil
}

func (im *controller) handleMetadataUpdate(h *home) tea.Cmd {
	for _, instance := range im.list.GetInstances() {
		if !instance.Started() || instance.Paused() {
			continue
		}
		updated, prompt := instance.HasUpdated()
		if updated {
			instance.SetStatus(instance.Running)
		} else {
			if prompt {
				instance.TapEnter()
			} else {
				instance.SetStatus(instance.Ready)
			}
		}
		if err := instance.UpdateDiffStats(); err != nil {
			log.WarningLog.Printf("could not update diff stats: %v", err)
		}
	}
	return tickUpdateMetadataCmd
}

func (im *controller) handleMouseEvent(h *home, msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Handle mouse wheel scrolling in the diff view
	if im.tabbedWindow.IsInDiffTab() {
		if msg.Action == tea.MouseActionPress {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				im.tabbedWindow.ScrollUp()
				return h, im.instanceChanged(h)
			case tea.MouseButtonWheelDown:
				im.tabbedWindow.ScrollDown()
				return h, im.instanceChanged(h)
			default:
				break
			}
		}
	}
	return h, nil
}

func (im *controller) handleKeyEvent(h *home, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle prompt state key events
	if h.state == tuiStatePrompt && im.textInputOverlay != nil {
		return im.handlePromptKeyEvent(h, msg)
	}

	// Handle other key events
	return im.handleKeyPress(h, msg)
}

func (im *controller) handlePromptKeyEvent(h *home, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	shouldClose := im.textInputOverlay.HandleKeyPress(msg)
	if !shouldClose {
		return h, nil
	}

	if im.textInputOverlay.IsSubmitted() {
		if im.orchestratorState == orchestratorStatePrompt {
			// Handle orchestrator prompt - generate plan first
			prompt := im.textInputOverlay.GetValue()
			im.textInputOverlay = nil
			im.orchestratorState = orchestratorStatePrompt
			return im.generateOrchestratorPlan(h, prompt)
		} else {
			// Handle regular prompt for selected instance
			selected := im.list.GetSelectedInstance()
			if selected != nil {
				if err := selected.SendPrompt(im.textInputOverlay.GetValue()); err != nil {
					return h, h.handleError(err)
				}
			}
		}
	}

	// Close the overlay and reset state
	im.textInputOverlay = nil
	// im.isOrchestratorPrompt = false
	h.state = tuiStateDefault
	return h, tea.Sequence(
		tea.WindowSize(),
		func() tea.Msg {
			h.menu.SetState(ui.StateDefault)
			h.showHelpScreen(helpTypeInstanceStart, nil, nil, nil)
			return nil
		},
	)
}

func (im *controller) handleKeyPress(h *home, msg tea.KeyMsg) (mod tea.Model, cmd tea.Cmd) {
	cmd, returnEarly := h.handleMenuHighlighting(msg)
	if returnEarly {
		return h, cmd
	}

	if h.state == tuiStateHelp {
		// // Check if we're showing an orchestrator plan for approval
		// if im.orchestratorPlan != "" && im.textOverlay != nil {
		// 	return im.handleOrchestratorPlanKeyPress(h, msg)
		// }
		return h.handleHelpState(msg, im.textOverlay)
	}

	if h.state == tuiStateNew {
		return im.handleNewInstanceState(h, msg)
	}

	// Handle quit commands first
	if msg.String() == "ctrl+c" || msg.String() == "q" {
		return h.handleQuit()
	}

	name, ok := keys.InstanceModeKeyMap[msg.String()]
	if !ok {
		return h, nil
	}

	switch name {
	case keys.KeyHelp:
		return h.showHelpScreen(helpTypeGeneral, nil, nil, nil)
	case keys.KeyPrompt, keys.KeyNew:
		return im.handleNewInstance(h, name == keys.KeyPrompt)
	case keys.KeyOrchestrator:
		return im.handleNewOrchestrator(h)
	case keys.KeyUp:
		im.list.Up()
		return h, im.instanceChanged(h)
	case keys.KeyDown:
		im.list.Down()
		return h, im.instanceChanged(h)
	case keys.KeyShiftUp:
		if im.tabbedWindow.IsInDiffTab() {
			im.tabbedWindow.ScrollUp()
		}
		return h, im.instanceChanged(h)
	case keys.KeyShiftDown:
		if im.tabbedWindow.IsInDiffTab() {
			im.tabbedWindow.ScrollDown()
		}
		return h, im.instanceChanged(h)
	case keys.KeyTab:
		im.tabbedWindow.Toggle()
		h.menu.SetInDiffTab(im.tabbedWindow.IsInDiffTab())
		return h, im.instanceChanged(h)
	case keys.KeyKill:
		return im.handleKillInstance(h)
	case keys.KeySubmit:
		return im.handleSubmitChanges(h)
	case keys.KeyCheckout:
		return im.handleCheckoutInstance(h)
	case keys.KeyResume:
		return im.handleResumeInstance(h)
	case keys.KeyEnter:
		return im.handleAttachInstance(h)
	default:
		return h, nil
	}
}

func (im *controller) handleNewInstanceState(h *home, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle quit commands first. Don't handle q because the user might want to type that.
	if msg.String() == "ctrl+c" {
		h.state = tuiStateDefault
		im.promptAfterName = false
		im.list.Kill()
		return h, tea.Sequence(
			tea.WindowSize(),
			func() tea.Msg {
				h.menu.SetState(ui.StateDefault)
				return nil
			},
		)
	}

	instance := im.list.GetInstances()[im.list.NumInstances()-1]
	switch msg.Type {
	case tea.KeyEnter:
		return im.finalizeNewInstance(h, instance)
	case tea.KeyRunes:
		if len(instance.Title) >= 32 {
			return h, h.handleError(fmt.Errorf("title cannot be longer than 32 characters"))
		}
		if err := instance.SetTitle(instance.Title + string(msg.Runes)); err != nil {
			return h, h.handleError(err)
		}
	case tea.KeyBackspace:
		if len(instance.Title) == 0 {
			return h, nil
		}
		if err := instance.SetTitle(instance.Title[:len(instance.Title)-1]); err != nil {
			return h, h.handleError(err)
		}
	case tea.KeySpace:
		if err := instance.SetTitle(instance.Title + " "); err != nil {
			return h, h.handleError(err)
		}
	case tea.KeyEsc:
		im.list.Kill()
		h.state = tuiStateDefault
		im.instanceChanged(h)

		return h, tea.Sequence(
			tea.WindowSize(),
			func() tea.Msg {
				h.menu.SetState(ui.StateDefault)
				return nil
			},
		)
	default:
	}
	return h, nil
}

func (im *controller) finalizeNewInstance(h *home, instance *session.Instance) (tea.Model, tea.Cmd) {
	if len(instance.Title) == 0 {
		return h, h.handleError(fmt.Errorf("title cannot be empty"))
	}

	if err := instance.Start(true); err != nil {
		im.list.Kill()
		h.state = tuiStateDefault
		return h, h.handleError(err)
	}
	// Save after adding new instance
	if err := h.storage.SaveInstances(im.list.GetInstances()); err != nil {
		return h, h.handleError(err)
	}
	// Instance added successfully, call the finalizer.
	im.newInstanceFinalizer()
	if h.autoYes {
		instance.AutoYes = true
	}

	im.newInstanceFinalizer()
	h.state = tuiStateDefault
	if im.promptAfterName {
		h.state = tuiStatePrompt
		h.menu.SetState(ui.StatePrompt)
		// Initialize the text input overlay
		im.textInputOverlay = overlay.NewTextInputOverlay("Enter prompt", "")
		// Set proper size for the overlay
		im.textInputOverlay.SetSize(80, 20) // Match orchestrator overlay size
		im.promptAfterName = false
	} else {
		h.menu.SetState(ui.StateDefault)
		h.showHelpScreen(helpTypeInstanceStart, instance, nil, nil)
	}

	return h, tea.Batch(tea.WindowSize(), im.instanceChanged(h))
}

func (im *controller) handleNewInstance(h *home, promptAfter bool) (tea.Model, tea.Cmd) {
	if im.list.NumInstances() >= GlobalInstanceLimit {
		return h, h.handleError(
			fmt.Errorf("you can't create more than %d instances", GlobalInstanceLimit))
	}
	instance, err := session.NewInstance(session.InstanceOptions{
		Title:   "",
		Path:    ".",
		Program: h.program,
	})
	if err != nil {
		return h, h.handleError(err)
	}

	im.newInstanceFinalizer = im.list.AddInstance(instance)
	im.list.SetSelectedInstance(im.list.NumInstances() - 1)
	h.state = tuiStateNew
	h.menu.SetState(ui.StateNewInstance)
	im.promptAfterName = promptAfter

	return h, nil
}

func (im *controller) handleNewOrchestrator(h *home) (tea.Model, tea.Cmd) {
	// Create an orchestrator instance - similar to KeyPrompt but for orchestration
	h.state = tuiStatePrompt
	h.menu.SetState(ui.StatePrompt)
	// Initialize the text input overlay for orchestrator goal
	im.textInputOverlay = overlay.NewTextInputOverlay("Enter orchestration goal", "")
	// Set proper size for the overlay (should match other overlays)
	im.textInputOverlay.SetSize(80, 20)
	im.promptAfterName = false
	// im.isOrchestratorPrompt = true
	return h, nil
}

func (im *controller) handleKillInstance(h *home) (tea.Model, tea.Cmd) {
	selected := im.list.GetSelectedInstance()
	if selected == nil {
		return h, nil
	}

	worktree, err := selected.GetGitWorktree()
	if err != nil {
		return h, h.handleError(err)
	}

	checkedOut, err := worktree.IsBranchCheckedOut()
	if err != nil {
		return h, h.handleError(err)
	}

	if checkedOut {
		return h, h.handleError(fmt.Errorf("instance %s is currently checked out", selected.Title))
	}

	// Delete from storage first
	if err := h.storage.DeleteInstance(selected.Title); err != nil {
		return h, h.handleError(err)
	}

	// Then kill the instance
	im.list.Kill()
	return h, im.instanceChanged(h)
}

func (im *controller) handleSubmitChanges(h *home) (tea.Model, tea.Cmd) {
	selected := im.list.GetSelectedInstance()
	if selected == nil {
		return h, nil
	}

	// Default commit message with timestamp
	commitMsg := fmt.Sprintf("[claudesquad] update from '%s' on %s", selected.Title, time.Now().Format(time.RFC822))
	worktree, err := selected.GetGitWorktree()
	if err != nil {
		return h, h.handleError(err)
	}
	if err = worktree.PushChanges(commitMsg, true); err != nil {
		return h, h.handleError(err)
	}

	return h, nil
}

func (im *controller) handleCheckoutInstance(h *home) (tea.Model, tea.Cmd) {
	selected := im.list.GetSelectedInstance()
	if selected == nil {
		return h, nil
	}

	// Show help screen before pausing
	h.showHelpScreen(helpTypeInstanceCheckout, selected, nil, func() {
		if err := selected.Pause(); err != nil {
			h.handleError(err)
		}
		im.instanceChanged(h)
	})
	return h, nil
}

func (im *controller) handleResumeInstance(h *home) (tea.Model, tea.Cmd) {
	selected := im.list.GetSelectedInstance()
	if selected == nil {
		return h, nil
	}
	if err := selected.Resume(); err != nil {
		return h, h.handleError(err)
	}
	return h, tea.WindowSize()
}

func (im *controller) handleAttachInstance(h *home) (tea.Model, tea.Cmd) {
	if im.list.NumInstances() == 0 {
		return h, nil
	}
	selected := im.list.GetSelectedInstance()
	if selected == nil || selected.Paused() || !selected.TmuxAlive() {
		return h, nil
	}
	// Show help screen before attaching
	h.showHelpScreen(helpTypeInstanceAttach, selected, nil, func() {
		ch, err := im.list.Attach()
		if err != nil {
			h.handleError(err)
			return
		}
		<-ch
		h.state = tuiStateDefault
	})
	return h, nil
}

func (im *controller) instanceChanged(h *home) tea.Cmd {
	// selected may be nil
	selected := im.list.GetSelectedInstance()

	im.tabbedWindow.UpdateDiff(selected)
	// Update menu with current instance
	h.menu.SetInstance(selected)

	// If there's no selected instance, we don't need to update the preview.
	if err := im.tabbedWindow.UpdatePreview(selected); err != nil {
		return h.handleError(err)
	}
	return nil
}

// generateOrchestratorPlan generates a plan from the user's prompt and shows it for approval
func (im *controller) generateOrchestratorPlan(h *home, prompt string) (tea.Model, tea.Cmd) {
	return h, func() tea.Msg {
		orch := orchestrator.NewOrchestrator(h.program, prompt)
		im.agents = append(im.agents, orch)

		orch.ForumulatePlan()

		return tea.WindowSize()
	}
}

// handleOrchestratorPlanApproval handles when user approves the orchestrator plan
func (im *controller) handleOrchestratorPlanApproval(h *home) (tea.Model, tea.Cmd) {
	// For testing purposes, just show a success message
	return h, func() tea.Msg {
		// Show success message
		successMessage := "Plan Approved\n\nOrchestration plan has been approved. For testing purposes, no workers will be created."
		im.textOverlay = overlay.NewTextOverlay(successMessage)

		h.state = tuiStateHelp // Show the text overlay
		return tea.WindowSize()
	}
}

// handleOrchestratorPlanKeyPress handles key presses when showing orchestrator plan for approval
func (im *controller) handleOrchestratorPlanKeyPress(h *home, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// User approved the plan
		// im.orchestratorPlan = ""
		return im.handleOrchestratorPlanApproval(h)
	case "esc", "q":
		// User cancelled the plan
		// im.orchestratorPlan = ""
		im.textOverlay = nil
		h.state = tuiStateDefault
		return h, tea.Sequence(
			tea.WindowSize(),
			func() tea.Msg {
				h.menu.SetState(ui.StateDefault)
				return nil
			},
		)
	default:
		// Any other key shows help about the plan approval
		return h, nil
	}
}

func (im *controller) HandleQuit(h *home) (tea.Model, tea.Cmd) {
	if err := h.storage.SaveInstances(im.list.GetInstances()); err != nil {
		return h, h.handleError(err)
	}
	return h, tea.Quit
}
