package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// mcpConfig represents the .mcp.json file format that Claude Code auto-discovers.
type mcpConfig struct {
	MCPServers map[string]mcpServerEntry `json:"mcpServers"`
}

type mcpServerEntry struct {
	Type    string            `json:"type"`
	Command string            `json:"command"`
	Env     map[string]string `json:"env"`
}

// writeMCPConfig writes a .mcp.json file into the worktree directory so that
// Claude Code can discover the Hivemind MCP server. If the hivemind-mcp binary
// is not found (e.g. dev mode), this silently returns nil — MCP is a
// progressive enhancement.
func writeMCPConfig(worktreePath, instanceTitle string) error {
	execPath, err := os.Executable()
	if err != nil {
		return nil // can't determine binary location; skip silently
	}

	mcpBinary := filepath.Join(filepath.Dir(execPath), "hivemind-mcp")
	if _, err := os.Stat(mcpBinary); err != nil {
		// Try $GOPATH/bin for go install users
		if gopath := os.Getenv("GOPATH"); gopath != "" {
			mcpBinary = filepath.Join(gopath, "bin", "hivemind-mcp")
		} else if home, hErr := os.UserHomeDir(); hErr == nil {
			mcpBinary = filepath.Join(home, "go", "bin", "hivemind-mcp")
		}
		if _, err := os.Stat(mcpBinary); err != nil {
			return nil // not found anywhere; skip silently
		}
	}

	cfg := mcpConfig{
		MCPServers: map[string]mcpServerEntry{
			"hivemind": {
				Type:    "stdio",
				Command: mcpBinary,
				Env: map[string]string{
					"HIVEMIND_INSTANCE_ID": instanceTitle,
				},
			},
		},
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(worktreePath, ".mcp.json"), data, 0600)
}

// isClaudeProgram returns true if the program string refers to Claude Code.
// This duplicates the unexported function in session/tmux to avoid exporting it.
func isClaudeProgram(program string) bool {
	return strings.HasSuffix(program, "claude")
}
