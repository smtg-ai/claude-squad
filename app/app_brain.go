package app

import (
	"fmt"
	"time"

	"github.com/ByteMirror/hivemind/brain"
	"github.com/ByteMirror/hivemind/log"
	"github.com/ByteMirror/hivemind/session"

	tea "github.com/charmbracelet/bubbletea"
)

// pollBrainActions returns a Bubble Tea command that reads from the brain server's
// action channel and converts actions into brainActionMsg messages.
func (m *home) pollBrainActions() tea.Cmd {
	ch := m.brainServer.Actions()
	return func() tea.Msg {
		action := <-ch
		return brainActionMsg{action: action}
	}
}

// handleBrainAction processes a Tier 3 action request from an agent.
func (m *home) handleBrainAction(action brain.ActionRequest) (tea.Model, tea.Cmd) {
	switch action.Type {
	case brain.ActionCreateInstance:
		return m.handleActionCreateInstance(action)
	case brain.ActionInjectMessage:
		return m.handleActionInjectMessage(action)
	case brain.ActionPauseInstance:
		return m.handleActionPauseInstance(action)
	case brain.ActionResumeInstance:
		return m.handleActionResumeInstance(action)
	case brain.ActionKillInstance:
		return m.handleActionKillInstance(action)
	default:
		action.ResponseCh <- brain.ActionResponse{
			Error: fmt.Sprintf("unknown action type: %s", action.Type),
		}
	}

	return m, m.pollBrainActions()
}

// handleActionCreateInstance spawns a new instance in response to a brain action.
func (m *home) handleActionCreateInstance(action brain.ActionRequest) (tea.Model, tea.Cmd) {
	title, _ := action.Params["title"].(string)
	program, _ := action.Params["program"].(string)
	prompt, _ := action.Params["prompt"].(string)
	topic, _ := action.Params["topic"].(string)
	sourceInstance, _ := action.Params["source_instance"].(string)
	role, _ := action.Params["role"].(string)

	if title == "" {
		action.ResponseCh <- brain.ActionResponse{Error: "title is required"}
		return m, m.pollBrainActions()
	}

	// Check instance limit.
	if len(m.allInstances) >= GlobalInstanceLimit {
		action.ResponseCh <- brain.ActionResponse{
			Error: fmt.Sprintf("instance limit reached (%d)", GlobalInstanceLimit),
		}
		return m, m.pollBrainActions()
	}

	// Check for duplicate title.
	if m.findInstanceByTitle(title) != nil {
		action.ResponseCh <- brain.ActionResponse{Error: fmt.Sprintf("instance %q already exists", title)}
		return m, m.pollBrainActions()
	}

	// Default program to the TUI's program.
	if program == "" {
		program = m.program
	}

	// Determine topic: use specified, or inherit from the requesting agent.
	topicName := topic
	if topicName == "" && sourceInstance != "" {
		if parent := m.findInstanceByTitle(sourceInstance); parent != nil {
			topicName = parent.TopicName
			log.InfoLog.Printf("brain: inheriting topic %q from parent %q", topicName, sourceInstance)
		} else {
			log.WarningLog.Printf("brain: parent instance %q not found in allInstances, cannot inherit topic", sourceInstance)
		}
	}

	// Spawned agents default to --dangerously-skip-permissions for autonomous operation.
	// Callers can explicitly set skip_permissions=false to override.
	skipPerms := true
	if v, ok := action.Params["skip_permissions"].(bool); ok {
		skipPerms = v
	}

	instance, err := session.NewInstance(session.InstanceOptions{
		Title:           title,
		Path:            m.repoPathForNewInstance(),
		Program:         program,
		TopicName:       topicName,
		SkipPermissions: skipPerms,
		Role:            role,
		ParentTitle:     sourceInstance,
	})
	if err != nil {
		action.ResponseCh <- brain.ActionResponse{Error: "failed to create instance: " + err.Error()}
		return m, m.pollBrainActions()
	}

	m.inheritAutoYesFromTopic(instance, topicName)

	log.InfoLog.Printf("brain: creating instance %q (program=%s, topic=%s, skipPerms=%v)", title, program, topicName, skipPerms)

	// Add to list UI immediately.
	finalizer := m.list.AddInstance(instance)

	// Start the instance asynchronously. The response is sent after start completes.
	responseCh := action.ResponseCh
	brainSrv := m.brainServer
	startCmd := func() tea.Msg {
		// Find topic for shared worktree.
		var topicObj *session.Topic
		for _, t := range m.topics {
			if t.Name == instance.TopicName {
				topicObj = t
				break
			}
		}

		var startErr error
		if topicObj != nil && topicObj.SharedWorktree && topicObj.Started() {
			startErr = instance.StartInSharedWorktree(topicObj.GetGitWorktree(), topicObj.Branch)
		} else {
			startErr = instance.Start(true)
		}

		if startErr != nil {
			responseCh <- brain.ActionResponse{Error: "failed to start instance: " + startErr.Error()}
			return brainInstanceFailedMsg{title: instance.Title}
		}

		// Send initial prompt if provided.
		if prompt != "" {
			// Give the agent a moment to initialize.
			time.Sleep(2 * time.Second)
			instance.SendPrompt(prompt)
		}

		responseCh <- brain.ActionResponse{
			OK: true,
			Data: map[string]any{
				"title":  instance.Title,
				"status": "created",
			},
		}

		if brainSrv != nil {
			brainSrv.PushEvent(brain.Event{
				Type:     brain.EventInstanceCreated,
				Source:   instance.Title,
				RepoPath: instance.GetRepoPath(),
				Data: map[string]any{
					"parent_title": instance.ParentTitle,
					"role":         instance.Role,
				},
			})
		}

		return brainInstanceStartedMsg{instance: instance, finalizer: finalizer}
	}

	return m, tea.Batch(startCmd, m.pollBrainActions())
}

// findInstanceByTitle returns the instance with the given title, or nil.
func (m *home) findInstanceByTitle(title string) *session.Instance {
	for _, inst := range m.allInstances {
		if inst.Title == title {
			return inst
		}
	}
	return nil
}

// handleActionInjectMessage injects text directly into a target agent's terminal.
func (m *home) handleActionInjectMessage(action brain.ActionRequest) (tea.Model, tea.Cmd) {
	to, _ := action.Params["to"].(string)
	content, _ := action.Params["content"].(string)

	if to == "" || content == "" {
		action.ResponseCh <- brain.ActionResponse{Error: "to and content are required"}
		return m, m.pollBrainActions()
	}

	target := m.findInstanceByTitle(to)
	if target == nil {
		action.ResponseCh <- brain.ActionResponse{Error: fmt.Sprintf("instance %q not found", to)}
		return m, m.pollBrainActions()
	}

	if !target.Started() || target.Paused() {
		action.ResponseCh <- brain.ActionResponse{Error: fmt.Sprintf("instance %q is not running", to)}
		return m, m.pollBrainActions()
	}

	// Format and submit the message into the target agent's terminal.
	from, _ := action.Params["from"].(string)
	formattedMsg := fmt.Sprintf("[HIVEMIND] %s says: %s", from, content)

	if err := target.SendPrompt(formattedMsg); err != nil {
		action.ResponseCh <- brain.ActionResponse{Error: "failed to inject message: " + err.Error()}
		return m, m.pollBrainActions()
	}

	log.InfoLog.Printf("brain: injected message from %s → %s", from, to)
	action.ResponseCh <- brain.ActionResponse{OK: true}
	return m, m.pollBrainActions()
}

// handleActionPauseInstance pauses a target agent instance.
func (m *home) handleActionPauseInstance(action brain.ActionRequest) (tea.Model, tea.Cmd) {
	target, _ := action.Params["target"].(string)
	if target == "" {
		action.ResponseCh <- brain.ActionResponse{Error: "target is required"}
		return m, m.pollBrainActions()
	}

	inst := m.findInstanceByTitle(target)
	if inst == nil {
		action.ResponseCh <- brain.ActionResponse{Error: fmt.Sprintf("instance %q not found", target)}
		return m, m.pollBrainActions()
	}

	if inst.Status == session.Paused {
		action.ResponseCh <- brain.ActionResponse{Error: fmt.Sprintf("instance %q is already paused", target)}
		return m, m.pollBrainActions()
	}

	// Set Loading immediately to prevent the metadata tick goroutine from
	// overwriting the status while Pause() is running.
	inst.SetStatus(session.Loading)
	inst.LoadingMessage = "Pausing..."

	m.removeAgentFromBrain(inst)
	if err := inst.Pause(); err != nil {
		action.ResponseCh <- brain.ActionResponse{Error: "failed to pause: " + err.Error()}
		return m, m.pollBrainActions()
	}

	m.saveAllInstances()
	log.InfoLog.Printf("brain: paused instance %q", target)
	action.ResponseCh <- brain.ActionResponse{OK: true}
	return m, tea.Batch(m.pollBrainActions(), m.instanceChanged())
}

// handleActionResumeInstance resumes a paused agent instance.
func (m *home) handleActionResumeInstance(action brain.ActionRequest) (tea.Model, tea.Cmd) {
	target, _ := action.Params["target"].(string)
	if target == "" {
		action.ResponseCh <- brain.ActionResponse{Error: "target is required"}
		return m, m.pollBrainActions()
	}

	inst := m.findInstanceByTitle(target)
	if inst == nil {
		action.ResponseCh <- brain.ActionResponse{Error: fmt.Sprintf("instance %q not found", target)}
		return m, m.pollBrainActions()
	}

	if inst.Status != session.Paused {
		action.ResponseCh <- brain.ActionResponse{Error: fmt.Sprintf("instance %q is not paused", target)}
		return m, m.pollBrainActions()
	}

	inst.SetStatus(session.Loading)
	inst.LoadingMessage = "Resuming..."

	responseCh := action.ResponseCh
	resumeCmd := func() tea.Msg {
		err := inst.Resume()
		if err != nil {
			responseCh <- brain.ActionResponse{Error: "failed to resume: " + err.Error()}
		} else {
			responseCh <- brain.ActionResponse{OK: true}
		}
		return instanceResumedMsg{instance: inst, err: err}
	}

	return m, tea.Batch(resumeCmd, m.pollBrainActions())
}

// handleActionKillInstance terminates a target agent instance.
func (m *home) handleActionKillInstance(action brain.ActionRequest) (tea.Model, tea.Cmd) {
	target, _ := action.Params["target"].(string)
	if target == "" {
		action.ResponseCh <- brain.ActionResponse{Error: "target is required"}
		return m, m.pollBrainActions()
	}

	inst := m.findInstanceByTitle(target)
	if inst == nil {
		action.ResponseCh <- brain.ActionResponse{Error: fmt.Sprintf("instance %q not found", target)}
		return m, m.pollBrainActions()
	}

	repoPath := inst.GetRepoPath()
	parentTitle := inst.ParentTitle

	m.removeAgentFromBrain(inst)
	m.tabbedWindow.GetTerminalPane().KillSession(target)
	m.removeFromAllInstances(target)
	m.list.KillInstanceByTitle(target)
	m.saveAllInstances()
	m.updateSidebarItems()

	if m.brainServer != nil {
		m.brainServer.PushEvent(brain.Event{
			Type:     brain.EventInstanceKilled,
			Source:   target,
			RepoPath: repoPath,
			Data: map[string]any{
				"parent_title": parentTitle,
			},
		})
	}

	log.InfoLog.Printf("brain: killed instance %q", target)
	action.ResponseCh <- brain.ActionResponse{OK: true}
	return m, tea.Batch(m.pollBrainActions(), m.instanceChanged())
}
