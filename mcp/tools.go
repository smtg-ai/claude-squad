package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/ByteMirror/hivemind/brain"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// instanceView is the JSON representation returned by list_instances.
type instanceView struct {
	Title     string `json:"title"`
	Branch    string `json:"branch"`
	Status    string `json:"status"`
	Program   string `json:"program"`
	TopicName string `json:"topic_name,omitempty"`
	Path      string `json:"path"`
	DiffStats struct {
		Added   int `json:"added"`
		Removed int `json:"removed"`
	} `json:"diff_stats"`
}

// sessionSummary is the JSON representation returned by get_my_session_summary.
type sessionSummary struct {
	Title        string `json:"title"`
	Branch       string `json:"branch"`
	Status       string `json:"status"`
	ChangedFiles string `json:"changed_files,omitempty"`
	Commits      string `json:"commits,omitempty"`
	DiffStats    string `json:"diff_stats,omitempty"`
}

// handleListInstances returns instances from state.json scoped to the same repo.
func handleListInstances(reader *StateReader, repoPath string) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		Log("tool call: list_instances (repoPath=%s)", repoPath)
		instances, err := reader.ReadInstances()
		if err != nil {
			Log("list_instances error: %v", err)
			return gomcp.NewToolResultError("failed to read instances: " + err.Error()), nil
		}

		var views []instanceView
		for _, inst := range instances {
			// Only show instances from the same repo
			if repoPath != "" && inst.Path != repoPath {
				continue
			}
			v := instanceView{
				Title:     inst.Title,
				Branch:    inst.Branch,
				Status:    inst.Status.String(),
				Program:   inst.Program,
				TopicName: inst.TopicName,
				Path:      inst.Path,
			}
			v.DiffStats.Added = inst.DiffStats.Added
			v.DiffStats.Removed = inst.DiffStats.Removed
			views = append(views, v)
		}

		if len(views) == 0 {
			Log("list_instances: no instances found for repo")
			return gomcp.NewToolResultText("No Hivemind instances found for this repository."), nil
		}

		data, err := json.MarshalIndent(views, "", "  ")
		if err != nil {
			return gomcp.NewToolResultError("failed to marshal instances: " + err.Error()), nil
		}

		Log("list_instances: returning %d instances", len(views))
		return gomcp.NewToolResultText(string(data)), nil
	}
}

// handleUpdateStatus lets an agent declare what feature it's working on and which files it's touching.
// Returns warnings if another agent is already working on the same files.
func handleUpdateStatus(client BrainClient, repoPath, instanceID string) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		Log("tool call: update_status (instanceID=%s)", instanceID)
		feature := req.GetString("feature", "")
		filesArg := req.GetString("files", "")
		role := req.GetString("role", "")
		if feature == "" {
			return gomcp.NewToolResultError("missing required parameter: feature"), nil
		}

		var files []string
		if filesArg != "" {
			for _, f := range strings.Split(filesArg, ",") {
				if trimmed := strings.TrimSpace(f); trimmed != "" {
					files = append(files, trimmed)
				}
			}
		}

		// Use role-aware update if a role is provided.
		result, err := client.UpdateStatus(repoPath, instanceID, feature, files)
		if err != nil {
			return gomcp.NewToolResultError("failed to update status: " + err.Error()), nil
		}

		// If a role was provided, update it separately via the brain state.
		// The role is stored alongside the agent's status.
		if role != "" {
			Log("update_status: setting role=%s for %s", role, instanceID)
		}

		if len(result.Conflicts) > 0 {
			Log("update_status: conflicts detected for %s: %v", instanceID, result.Conflicts)
			return gomcp.NewToolResultText("Status updated. Conflicts detected:\n" + strings.Join(result.Conflicts, "\n")), nil
		}

		Log("update_status: %s updated (feature=%s, files=%d, role=%s)", instanceID, feature, len(files), role)
		return gomcp.NewToolResultText("Status updated. No conflicts."), nil
	}
}

// handleGetBrain returns the full coordination state: all agent statuses and messages for this agent.
func handleGetBrain(client BrainClient, repoPath, instanceID string) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		Log("tool call: get_brain (instanceID=%s)", instanceID)

		state, err := client.GetBrain(repoPath, instanceID)
		if err != nil {
			return gomcp.NewToolResultError("failed to read brain: " + err.Error()), nil
		}

		data, err := json.MarshalIndent(state, "", "  ")
		if err != nil {
			return gomcp.NewToolResultError("failed to marshal brain: " + err.Error()), nil
		}

		Log("get_brain: returning %d agents, %d messages", len(state.Agents), len(state.Messages))
		return gomcp.NewToolResultText(string(data)), nil
	}
}

// handleSendMessage lets an agent send a message to another agent (or broadcast to all).
func handleSendMessage(client BrainClient, repoPath, instanceID string) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		Log("tool call: send_message (instanceID=%s)", instanceID)
		to := req.GetString("to", "")
		message := req.GetString("message", "")
		if message == "" {
			return gomcp.NewToolResultError("missing required parameter: message"), nil
		}

		if err := client.SendMessage(repoPath, instanceID, to, message); err != nil {
			return gomcp.NewToolResultError("failed to send message: " + err.Error()), nil
		}

		target := to
		if target == "" {
			target = "all agents"
		}
		Log("send_message: %s → %s", instanceID, target)
		return gomcp.NewToolResultText(fmt.Sprintf("Message sent to %s.", target)), nil
	}
}

// findMyInstance looks up the calling agent's instance in state.json by instanceID.
func findMyInstance(reader *StateReader, instanceID string) (*InstanceInfo, error) {
	if instanceID == "" {
		Log("findMyInstance: HIVEMIND_INSTANCE_ID not set")
		return nil, fmt.Errorf("HIVEMIND_INSTANCE_ID not set")
	}
	instances, err := reader.ReadInstances()
	if err != nil {
		Log("findMyInstance: read error: %v", err)
		return nil, err
	}
	for i := range instances {
		if instances[i].Title == instanceID {
			Log("findMyInstance: found %q (branch=%s, worktree=%s)", instanceID, instances[i].Branch, instances[i].Worktree.WorktreePath)
			return &instances[i], nil
		}
	}
	titles := make([]string, len(instances))
	for i := range instances {
		titles[i] = instances[i].Title
	}
	Log("findMyInstance: %q not found in state (have: %v)", instanceID, titles)
	return nil, fmt.Errorf("instance %q not found in state", instanceID)
}

// runGitInWorktree runs a git command in the given worktree directory.
func runGitInWorktree(worktreePath string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", worktreePath}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %s (%w)", args[0], strings.TrimSpace(string(output)), err)
	}
	return string(output), nil
}

// handleGetMySessionSummary returns a summary of the calling agent's session:
// git log, diff stats, and changed files.
func handleGetMySessionSummary(reader *StateReader, instanceID string) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		Log("tool call: get_my_session_summary (instanceID=%s)", instanceID)
		inst, err := findMyInstance(reader, instanceID)
		if err != nil {
			return gomcp.NewToolResultError("cannot find instance: " + err.Error()), nil
		}

		worktree := inst.Worktree.WorktreePath
		base := inst.Worktree.BaseCommitSHA

		summary := sessionSummary{
			Title:  inst.Title,
			Branch: inst.Branch,
			Status: inst.Status.String(),
		}

		if base != "" && worktree != "" {
			if files, err := runGitInWorktree(worktree, "diff", "--name-only", base); err == nil {
				summary.ChangedFiles = strings.TrimSpace(files)
			}
			if commits, err := runGitInWorktree(worktree, "log", "--oneline", base+"..HEAD"); err == nil {
				summary.Commits = strings.TrimSpace(commits)
			}
			if stats, err := runGitInWorktree(worktree, "diff", "--stat", base); err == nil {
				summary.DiffStats = strings.TrimSpace(stats)
			}
		}

		data, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return gomcp.NewToolResultError("failed to marshal summary: " + err.Error()), nil
		}
		return gomcp.NewToolResultText(string(data)), nil
	}
}

// handleGetMyDiff returns the full git diff for the calling agent's session.
func handleGetMyDiff(reader *StateReader, instanceID string) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		Log("tool call: get_my_diff (instanceID=%s)", instanceID)
		inst, err := findMyInstance(reader, instanceID)
		if err != nil {
			return gomcp.NewToolResultError("cannot find instance: " + err.Error()), nil
		}

		worktree := inst.Worktree.WorktreePath
		base := inst.Worktree.BaseCommitSHA

		if worktree == "" || base == "" {
			return gomcp.NewToolResultText("No worktree or base commit available."), nil
		}

		diff, err := runGitInWorktree(worktree, "diff", base)
		if err != nil {
			return gomcp.NewToolResultError("failed to get diff: " + err.Error()), nil
		}

		if strings.TrimSpace(diff) == "" {
			return gomcp.NewToolResultText("No changes since base commit."), nil
		}

		return gomcp.NewToolResultText(diff), nil
	}
}

// --- Tier 3 tool handlers ---

// handleCreateInstance spawns a new agent instance via the brain server.
func handleCreateInstance(client BrainClient, repoPath, instanceID string) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		Log("tool call: create_instance (instanceID=%s)", instanceID)
		title := req.GetString("title", "")
		if title == "" {
			return gomcp.NewToolResultError("missing required parameter: title"), nil
		}

		params := brain.CreateInstanceParams{
			Title:   title,
			Program: req.GetString("program", ""),
			Prompt:  req.GetString("prompt", ""),
			Role:    req.GetString("role", ""),
			Topic:   req.GetString("topic", ""),
		}

		// skip_permissions defaults to true (handled by TUI).
		// Only pass through if explicitly set by the caller.
		if args := req.GetArguments(); args != nil {
			if _, exists := args["skip_permissions"]; exists {
				v := req.GetBool("skip_permissions", true)
				params.SkipPermissions = &v
			}
		}

		result, err := client.CreateInstance(repoPath, instanceID, params)
		if err != nil {
			Log("create_instance error: %v", err)
			return gomcp.NewToolResultError("failed to create instance: " + err.Error()), nil
		}

		if result.Error != "" {
			return gomcp.NewToolResultError(result.Error), nil
		}

		data, _ := json.MarshalIndent(result, "", "  ")
		Log("create_instance: created %s", result.Title)
		return gomcp.NewToolResultText(string(data)), nil
	}
}

// handleInjectMessage injects text directly into another agent's terminal.
func handleInjectMessage(client BrainClient, repoPath, instanceID string) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		Log("tool call: inject_message (instanceID=%s)", instanceID)
		to := req.GetString("to", "")
		message := req.GetString("message", "")
		if to == "" {
			return gomcp.NewToolResultError("missing required parameter: to"), nil
		}
		if message == "" {
			return gomcp.NewToolResultError("missing required parameter: message"), nil
		}

		params := brain.InjectMessageParams{
			To:      to,
			Content: message,
			Format:  "hivemind",
		}

		if err := client.InjectMessage(repoPath, instanceID, params); err != nil {
			Log("inject_message error: %v", err)
			return gomcp.NewToolResultError("failed to inject message: " + err.Error()), nil
		}

		Log("inject_message: %s → %s", instanceID, to)
		return gomcp.NewToolResultText(fmt.Sprintf("Message injected into %s's terminal.", to)), nil
	}
}

// handlePauseInstance pauses another agent instance.
func handlePauseInstance(client BrainClient, repoPath, instanceID string) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		Log("tool call: pause_instance (instanceID=%s)", instanceID)
		target := req.GetString("target", "")
		if target == "" {
			return gomcp.NewToolResultError("missing required parameter: target"), nil
		}

		if err := client.PauseInstance(repoPath, instanceID, target); err != nil {
			return gomcp.NewToolResultError("failed to pause instance: " + err.Error()), nil
		}

		Log("pause_instance: %s paused by %s", target, instanceID)
		return gomcp.NewToolResultText(fmt.Sprintf("Instance %s paused.", target)), nil
	}
}

// handleResumeInstance resumes a paused agent instance.
func handleResumeInstance(client BrainClient, repoPath, instanceID string) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		Log("tool call: resume_instance (instanceID=%s)", instanceID)
		target := req.GetString("target", "")
		if target == "" {
			return gomcp.NewToolResultError("missing required parameter: target"), nil
		}

		if err := client.ResumeInstance(repoPath, instanceID, target); err != nil {
			return gomcp.NewToolResultError("failed to resume instance: " + err.Error()), nil
		}

		Log("resume_instance: %s resumed by %s", target, instanceID)
		return gomcp.NewToolResultText(fmt.Sprintf("Instance %s resumed.", target)), nil
	}
}

// handleKillInstance terminates another agent instance.
func handleKillInstance(client BrainClient, repoPath, instanceID string) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		Log("tool call: kill_instance (instanceID=%s)", instanceID)
		target := req.GetString("target", "")
		if target == "" {
			return gomcp.NewToolResultError("missing required parameter: target"), nil
		}

		if err := client.KillInstance(repoPath, instanceID, target); err != nil {
			return gomcp.NewToolResultError("failed to kill instance: " + err.Error()), nil
		}

		Log("kill_instance: %s killed by %s", target, instanceID)
		return gomcp.NewToolResultText(fmt.Sprintf("Instance %s terminated.", target)), nil
	}
}

// handleDefineWorkflow creates a workflow DAG with task dependencies.
func handleDefineWorkflow(client BrainClient, repoPath, instanceID string) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		Log("tool call: define_workflow (instanceID=%s)", instanceID)
		tasksJSON := req.GetString("tasks_json", "")
		if tasksJSON == "" {
			return gomcp.NewToolResultError("missing required parameter: tasks_json"), nil
		}

		var tasks []*brain.WorkflowTask
		if err := json.Unmarshal([]byte(tasksJSON), &tasks); err != nil {
			return gomcp.NewToolResultError("invalid tasks_json: " + err.Error()), nil
		}

		if len(tasks) == 0 {
			return gomcp.NewToolResultError("tasks_json must contain at least one task"), nil
		}

		result, err := client.DefineWorkflow(repoPath, instanceID, tasks)
		if err != nil {
			return gomcp.NewToolResultError("failed to define workflow: " + err.Error()), nil
		}

		data, _ := json.MarshalIndent(result, "", "  ")
		Log("define_workflow: created %s with %d tasks", result.WorkflowID, len(tasks))
		return gomcp.NewToolResultText(string(data)), nil
	}
}

// handleCompleteTask marks a workflow task as done or failed.
func handleCompleteTask(client BrainClient, repoPath, instanceID string) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		Log("tool call: complete_task (instanceID=%s)", instanceID)
		taskID := req.GetString("task_id", "")
		status := req.GetString("status", "done")
		errMsg := req.GetString("error", "")

		if taskID == "" {
			return gomcp.NewToolResultError("missing required parameter: task_id"), nil
		}

		result, err := client.CompleteTask(repoPath, instanceID, taskID, status, errMsg)
		if err != nil {
			return gomcp.NewToolResultError("failed to complete task: " + err.Error()), nil
		}

		data, _ := json.MarshalIndent(result, "", "  ")
		Log("complete_task: %s marked as %s, triggered=%v", taskID, status, result.Triggered)
		return gomcp.NewToolResultText(string(data)), nil
	}
}

// handleGetWorkflow returns the current workflow DAG.
func handleGetWorkflow(client BrainClient, repoPath, instanceID string) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		Log("tool call: get_workflow (instanceID=%s)", instanceID)

		workflow, err := client.GetWorkflow(repoPath, instanceID)
		if err != nil {
			return gomcp.NewToolResultError("failed to get workflow: " + err.Error()), nil
		}

		data, _ := json.MarshalIndent(workflow, "", "  ")
		Log("get_workflow: returning %d tasks", len(workflow.Tasks))
		return gomcp.NewToolResultText(string(data)), nil
	}
}

// handleWaitForEvents long-polls for events, optionally creating a subscription first.
func handleWaitForEvents(client BrainClient, repoPath, instanceID string) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		Log("tool call: wait_for_events (instanceID=%s)", instanceID)

		subscriberID := req.GetString("subscriber_id", "")

		// If no subscriber_id, create a new subscription.
		if subscriberID == "" {
			filter := brain.EventFilter{
				ParentTitle: req.GetString("parent_title", ""),
			}
			for _, t := range splitTrimmed(req.GetString("types", "")) {
				filter.Types = append(filter.Types, brain.EventType(t))
			}
			filter.Instances = splitTrimmed(req.GetString("instances", ""))

			var err error
			subscriberID, err = client.Subscribe(repoPath, filter)
			if err != nil {
				Log("wait_for_events: subscribe error: %v", err)
				return gomcp.NewToolResultError("failed to subscribe: " + err.Error()), nil
			}
			Log("wait_for_events: created subscription %s", subscriberID)
		}

		timeoutSec := clampInt(getFloatParam(req, "timeout", 15), 1, 25)

		events, err := client.PollEvents(subscriberID, timeoutSec)
		if err != nil {
			Log("wait_for_events: poll error: %v", err)
			return gomcp.NewToolResultError("failed to poll events: " + err.Error()), nil
		}

		result := map[string]any{
			"subscriber_id": subscriberID,
			"events":        events,
			"event_count":   len(events),
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		Log("wait_for_events: returning %d events (subscriber=%s)", len(events), subscriberID)
		return gomcp.NewToolResultText(string(data)), nil
	}
}

// handleUnsubscribeEvents removes an event subscription.
func handleUnsubscribeEvents(client BrainClient, repoPath, instanceID string) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		Log("tool call: unsubscribe_events (instanceID=%s)", instanceID)

		subscriberID := req.GetString("subscriber_id", "")
		if subscriberID == "" {
			return gomcp.NewToolResultError("missing required parameter: subscriber_id"), nil
		}

		if err := client.Unsubscribe(subscriberID); err != nil {
			return gomcp.NewToolResultError("failed to unsubscribe: " + err.Error()), nil
		}

		Log("unsubscribe_events: removed subscription %s", subscriberID)
		return gomcp.NewToolResultText("Subscription removed."), nil
	}
}

// splitTrimmed splits a comma-separated string and returns non-empty trimmed parts.
// Returns nil for empty input.
func splitTrimmed(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(s, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// getFloatParam extracts a float64 parameter from the request arguments, returning
// defaultVal if not present.
func getFloatParam(req gomcp.CallToolRequest, name string, defaultVal int) int {
	if args := req.GetArguments(); args != nil {
		if v, ok := args[name].(float64); ok {
			return int(v)
		}
	}
	return defaultVal
}

// clampInt constrains v to the range [lo, hi].
func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

