package gui

import (
	"claude-squad/config"
	"claude-squad/gui/dialogs"
	"claude-squad/gui/panes"
	"claude-squad/gui/sidebar"
	"claude-squad/log"
	"claude-squad/session"
	"fmt"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
)

// guiState holds mutable application state shared across callbacks.
type guiState struct {
	mu        sync.Mutex
	instances []*session.Instance
	storage   *session.Storage
}

func (s *guiState) addInstance(inst *session.Instance) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.instances = append(s.instances, inst)
}

func (s *guiState) removeInstance(title string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, inst := range s.instances {
		if inst.Title == title {
			s.instances = append(s.instances[:i], s.instances[i+1:]...)
			return
		}
	}
}

func (s *guiState) getInstances() []*session.Instance {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]*session.Instance, len(s.instances))
	copy(cp, s.instances)
	return cp
}

// Run starts the GUI application.
func Run(program string, autoYes bool) error {
	appConfig := config.LoadConfig()
	appStateConfig := config.LoadState()
	storage, err := session.NewStorage(appStateConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Load saved instances
	instances, err := storage.LoadInstances()
	if err != nil {
		return fmt.Errorf("failed to load instances: %w", err)
	}

	state := &guiState{
		instances: instances,
		storage:   storage,
	}

	a := app.New()
	a.Settings().SetTheme(&squadTheme{})
	w := a.NewWindow("Claude Squad")

	var sidebarWidget *sidebar.Sidebar
	var paneManager *panes.Manager
	sidebarVisible := true

	// Pane manager
	paneManager = panes.NewManager(func(p *panes.Pane) {
		// Focus callback
	})

	// Sidebar
	sidebarWidget = sidebar.NewSidebar(
		func(inst *session.Instance) {
			// On select — open in focused pane
			openSessionInFocusedPane(paneManager, inst)
		},
		func(inst *session.Instance) {
			// On activate (double-click) — also open in focused pane
			openSessionInFocusedPane(paneManager, inst)
		},
		func() {
			// On new
			showNewSessionDialog(w, appConfig, program, state, sidebarWidget, paneManager, autoYes)
		},
	)

	// Layout: sidebar | panes
	sidebarObj := sidebarWidget.Widget()
	rootSplit := container.NewHSplit(sidebarObj, paneManager.Widget())
	rootSplit.SetOffset(0.2)
	rootContainer := container.NewStack(rootSplit)

	// Register hotkeys
	RegisterHotkeys(w.Canvas(), Handlers{
		NewSession: func() {
			showNewSessionDialog(w, appConfig, program, state, sidebarWidget, paneManager, autoYes)
		},
		SplitVertical: func() {
			paneManager.SplitVertical()
			rootSplit.Trailing = paneManager.Widget()
			rootSplit.Refresh()
		},
		SplitHorizontal: func() {
			paneManager.SplitHorizontal()
			rootSplit.Trailing = paneManager.Widget()
			rootSplit.Refresh()
		},
		ClosePane: func() {
			paneManager.CloseFocused()
			rootSplit.Trailing = paneManager.Widget()
			rootSplit.Refresh()
		},
		NavigateLeft:  paneManager.NavigateLeft,
		NavigateRight: paneManager.NavigateRight,
		NavigateUp:    paneManager.NavigateUp,
		NavigateDown:  paneManager.NavigateDown,
		SidebarUp:     sidebarWidget.SelectUp,
		SidebarDown:   sidebarWidget.SelectDown,
		OpenInPane: func() {
			inst := sidebarWidget.SelectedInstance()
			if inst != nil {
				openSessionInFocusedPane(paneManager, inst)
			}
		},
		KillSession: func() {
			inst := sidebarWidget.SelectedInstance()
			if inst == nil {
				return
			}
			dialogs.ShowConfirm("Kill Session",
				fmt.Sprintf("Kill session '%s'?", inst.Title),
				func() {
					killSession(inst, state, sidebarWidget, paneManager)
				}, w)
		},
		PushChanges: func() {
			inst := sidebarWidget.SelectedInstance()
			if inst == nil {
				return
			}
			dialogs.ShowConfirm("Push Changes",
				fmt.Sprintf("Push changes from '%s'?", inst.Title),
				func() {
					pushSession(inst)
				}, w)
		},
		PauseResume: func() {
			inst := sidebarWidget.SelectedInstance()
			if inst == nil {
				return
			}
			togglePauseResume(inst, state, sidebarWidget)
		},
		ToggleSidebar: func() {
			sidebarVisible = !sidebarVisible
			if sidebarVisible {
				rootSplit.Leading = sidebarObj
				rootSplit.SetOffset(0.2)
			} else {
				rootSplit.Leading = container.NewStack() // empty
				rootSplit.SetOffset(0.0)
			}
			rootSplit.Refresh()
		},
		Quit: func() {
			paneManager.DisconnectAll()
			if err := state.storage.SaveInstances(state.getInstances()); err != nil {
				log.ErrorLog.Printf("failed to save instances on quit: %v", err)
			}
			a.Quit()
		},
	})

	// Initial sidebar update
	sidebarWidget.Update(state.getInstances())

	// Status polling goroutine
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			for _, inst := range state.getInstances() {
				if !inst.Started() || inst.Paused() {
					continue
				}
				inst.CheckAndHandleTrustPrompt()
				updated, prompt := inst.HasUpdated()
				if updated {
					inst.SetStatus(session.Running)
				} else if prompt {
					if autoYes {
						inst.TapEnter()
					} else {
						inst.SetStatus(session.Ready)
					}
				} else {
					inst.SetStatus(session.Running)
				}
				if err := inst.UpdateDiffStats(); err != nil {
					log.WarningLog.Printf("failed to update diff stats: %v", err)
				}
			}
			fyne.Do(func() {
				sidebarWidget.Update(state.getInstances())
				for _, p := range paneManager.AllPanes() {
					p.UpdateStatus()
				}
			})
		}
	}()

	w.SetContent(rootContainer)
	w.Resize(fyne.NewSize(1200, 800))
	w.SetOnClosed(func() {
		paneManager.DisconnectAll()
		state.storage.SaveInstances(state.getInstances())
	})
	w.ShowAndRun()
	return nil
}

func openSessionInFocusedPane(pm *panes.Manager, inst *session.Instance) {
	pane := pm.FocusedPane()
	if pane == nil || inst == nil {
		return
	}
	// Disconnect from any other pane showing this session
	for _, p := range pm.AllPanes() {
		if p.Instance() != nil && p.Instance().Title == inst.Title && p != pane {
			p.CloseSession()
		}
	}
	if err := pane.OpenSession(inst); err != nil {
		log.ErrorLog.Printf("failed to open session in pane: %v", err)
	}
}

func showNewSessionDialog(w fyne.Window, cfg *config.Config, defaultProgram string, state *guiState, sb *sidebar.Sidebar, pm *panes.Manager, autoYes bool) {
	dialogs.ShowNewSession(cfg.GetProfiles(), w, func(opts dialogs.SessionOptions) {
		if opts.Name == "" {
			return
		}
		prog := opts.Program
		if prog == "" {
			prog = defaultProgram
		}
		inst, err := session.NewInstance(session.InstanceOptions{
			Title:   opts.Name,
			Path:    ".",
			Program: prog,
		})
		if err != nil {
			log.ErrorLog.Printf("failed to create instance: %v", err)
			return
		}
		inst.AutoYes = autoYes
		inst.Prompt = opts.Prompt
		inst.SetStatus(session.Loading)
		state.addInstance(inst)
		sb.Update(state.getInstances())

		go func() {
			if err := inst.Start(true); err != nil {
				log.ErrorLog.Printf("failed to start instance: %v", err)
				return
			}
			if opts.Prompt != "" {
				if err := inst.SendPrompt(opts.Prompt); err != nil {
					log.ErrorLog.Printf("failed to send prompt: %v", err)
				}
				inst.Prompt = ""
			}
			sb.Update(state.getInstances())
			if err := state.storage.SaveInstances(state.getInstances()); err != nil {
				log.ErrorLog.Printf("failed to save instances: %v", err)
			}
		}()
	})
}

func killSession(inst *session.Instance, state *guiState, sb *sidebar.Sidebar, pm *panes.Manager) {
	// Close any pane showing this session
	for _, p := range pm.AllPanes() {
		if p.Instance() != nil && p.Instance().Title == inst.Title {
			p.CloseSession()
		}
	}

	if err := inst.Kill(); err != nil {
		log.ErrorLog.Printf("failed to kill instance: %v", err)
	}

	if err := state.storage.DeleteInstance(inst.Title); err != nil {
		log.ErrorLog.Printf("failed to delete instance: %v", err)
	}

	state.removeInstance(inst.Title)
	sb.Update(state.getInstances())
}

func pushSession(inst *session.Instance) {
	worktree, err := inst.GetGitWorktree()
	if err != nil {
		log.ErrorLog.Printf("failed to get worktree: %v", err)
		return
	}
	commitMsg := fmt.Sprintf("[claudesquad] update from '%s' on %s", inst.Title, time.Now().Format(time.RFC822))
	if err := worktree.PushChanges(commitMsg, true); err != nil {
		log.ErrorLog.Printf("failed to push changes: %v", err)
	}
}

func togglePauseResume(inst *session.Instance, state *guiState, sb *sidebar.Sidebar) {
	if inst.Status == session.Paused {
		if err := inst.Resume(); err != nil {
			log.ErrorLog.Printf("failed to resume: %v", err)
		}
	} else {
		if err := inst.Pause(); err != nil {
			log.ErrorLog.Printf("failed to pause: %v", err)
		}
	}
	sb.Update(state.getInstances())
}
