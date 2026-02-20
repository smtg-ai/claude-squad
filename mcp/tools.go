package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

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

// sharedContextEntry represents a single entry in shared_context.json.
type sharedContextEntry struct {
	InstanceID string `json:"instance_id"`
	Type       string `json:"type"`
	Content    string `json:"content"`
	Timestamp  string `json:"timestamp,omitempty"`
}

// handleListInstances returns all instances from state.json.
func handleListInstances(reader *StateReader) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		instances, err := reader.ReadInstances()
		if err != nil {
			return gomcp.NewToolResultError("failed to read instances: " + err.Error()), nil
		}

		if len(instances) == 0 {
			return gomcp.NewToolResultText("No Hivemind instances found."), nil
		}

		views := make([]instanceView, len(instances))
		for i, inst := range instances {
			views[i] = instanceView{
				Title:     inst.Title,
				Branch:    inst.Branch,
				Status:    inst.Status.String(),
				Program:   inst.Program,
				TopicName: inst.TopicName,
				Path:      inst.Path,
			}
			views[i].DiffStats.Added = inst.DiffStats.Added
			views[i].DiffStats.Removed = inst.DiffStats.Removed
		}

		data, err := json.MarshalIndent(views, "", "  ")
		if err != nil {
			return gomcp.NewToolResultError("failed to marshal instances: " + err.Error()), nil
		}

		return gomcp.NewToolResultText(string(data)), nil
	}
}

// handleCheckFileActivity checks if other agents are modifying specific files.
// This is a placeholder; real file tracking comes in a later phase.
func handleCheckFileActivity() mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		filesArg := req.GetString("files", "")
		if filesArg == "" {
			return gomcp.NewToolResultError("missing required parameter: files"), nil
		}

		files := strings.Split(filesArg, ",")
		for i := range files {
			files[i] = strings.TrimSpace(files[i])
		}

		result := struct {
			Files     []string `json:"files_checked"`
			Conflicts []string `json:"conflicts"`
			Message   string   `json:"message"`
		}{
			Files:     files,
			Conflicts: []string{},
			Message:   "No conflicts detected. File activity tracking is not yet implemented; this will be enhanced in a future update.",
		}

		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return gomcp.NewToolResultError("failed to marshal result: " + err.Error()), nil
		}

		return gomcp.NewToolResultText(string(data)), nil
	}
}

// handleGetSharedContext returns entries from shared_context.json.
func handleGetSharedContext(hivemindDir string) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		contextPath := filepath.Join(hivemindDir, "shared_context.json")
		data, err := os.ReadFile(contextPath)
		if err != nil {
			if os.IsNotExist(err) {
				return gomcp.NewToolResultText("[]"), nil
			}
			return gomcp.NewToolResultError("failed to read shared context: " + err.Error()), nil
		}

		var entries []sharedContextEntry
		if err := json.Unmarshal(data, &entries); err != nil {
			return gomcp.NewToolResultError("failed to parse shared context: " + err.Error()), nil
		}

		out, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return gomcp.NewToolResultError("failed to marshal shared context: " + err.Error()), nil
		}

		return gomcp.NewToolResultText(string(out)), nil
	}
}
