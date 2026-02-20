package session

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ByteMirror/hivemind/log"
)

// mcpServerNameRe strips characters that aren't valid in MCP server names.
var mcpServerNameRe = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// mcpServerName returns a unique, sanitized MCP server name for an instance.
// Each instance needs its own name so that multiple agents in a shared worktree
// don't overwrite each other's MCP registration and environment variables.
func mcpServerName(instanceTitle string) string {
	sanitized := mcpServerNameRe.ReplaceAllString(instanceTitle, "-")
	return "hivemind-" + sanitized
}

// registerMCPServer registers the Hivemind MCP server with Claude Code for
// the given worktree directory. It uses `claude mcp add` with local scope,
// which stores the config in ~/.claude.json per-project and does NOT require
// the approval prompt that project-scoped .mcp.json files trigger.
//
// Each instance gets a unique server name (hivemind-<title>) so that multiple
// agents in a shared worktree each get their own HIVEMIND_INSTANCE_ID.
//
// If the hivemind-mcp binary or claude CLI is not found, this silently
// returns nil — MCP is a progressive enhancement.
func registerMCPServer(worktreePath, repoPath, instanceTitle string) error {
	log.InfoLog.Printf("MCP config: registering for instance=%q worktree=%s repo=%s", instanceTitle, worktreePath, repoPath)

	mcpBinary, err := findMCPBinary()
	if err != nil {
		log.InfoLog.Printf("MCP config: skipped (%v)", err)
		return nil
	}

	serverName := mcpServerName(instanceTitle)
	log.InfoLog.Printf("MCP config: using binary %s (server name: %s)", mcpBinary, serverName)

	// Use `claude mcp add` to register the MCP server with local scope (default).
	// Local scope is stored in ~/.claude.json and doesn't require user approval,
	// unlike project-scoped .mcp.json which prompts for confirmation.
	// Note: -e flag must come after the server name, before -- separator.
	cmd := exec.Command("claude", "mcp", "add",
		serverName,
		"-e", fmt.Sprintf("HIVEMIND_INSTANCE_ID=%s", instanceTitle),
		"-e", fmt.Sprintf("HIVEMIND_REPO_PATH=%s", repoPath),
		"-e", "HIVEMIND_TIER=3",
		"--",
		mcpBinary,
	)
	cmd.Dir = worktreePath

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.WarningLog.Printf("MCP config: claude mcp add failed: %v (output: %s)", err, strings.TrimSpace(string(output)))
		// Fall back to writing .mcp.json directly
		return writeMCPConfigFile(worktreePath, repoPath, instanceTitle, mcpBinary)
	}

	log.InfoLog.Printf("MCP config: registered via claude mcp add (local scope)")
	return nil
}

// writeMCPConfigFile writes a .mcp.json file into the worktree directory as a
// fallback when `claude mcp add` is unavailable. Uses a unique server name
// per instance so multiple agents in a shared worktree don't collide.
func writeMCPConfigFile(worktreePath, repoPath, instanceTitle, mcpBinary string) error {
	log.InfoLog.Printf("MCP config: falling back to .mcp.json for instance=%q", instanceTitle)

	serverName := mcpServerName(instanceTitle)

	// Write the JSON manually to avoid importing encoding/json for a simple template
	content := fmt.Sprintf(`{
  "mcpServers": {
    %q: {
      "command": %q,
      "env": {
        "HIVEMIND_INSTANCE_ID": %q,
        "HIVEMIND_REPO_PATH": %q,
        "HIVEMIND_TIER": "3"
      }
    }
  }
}
`, serverName, mcpBinary, instanceTitle, repoPath)

	mcpPath := filepath.Join(worktreePath, ".mcp.json")
	if err := os.WriteFile(mcpPath, []byte(content), 0600); err != nil {
		return err
	}
	log.InfoLog.Printf("MCP config: wrote %s", mcpPath)
	return nil
}

// findMCPBinary locates the hivemind-mcp binary. It checks:
// 1. Next to the current executable
// 2. $GOPATH/bin
// 3. ~/go/bin
func findMCPBinary() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("can't determine executable path: %w", err)
	}

	mcpBinary := filepath.Join(filepath.Dir(execPath), "hivemind-mcp")
	if _, err := os.Stat(mcpBinary); err == nil {
		return mcpBinary, nil
	}

	// Try $GOPATH/bin
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		mcpBinary = filepath.Join(gopath, "bin", "hivemind-mcp")
	} else if home, hErr := os.UserHomeDir(); hErr == nil {
		mcpBinary = filepath.Join(home, "go", "bin", "hivemind-mcp")
	}
	if _, err := os.Stat(mcpBinary); err == nil {
		return mcpBinary, nil
	}

	return "", fmt.Errorf("hivemind-mcp not found")
}

// isClaudeProgram returns true if the program string refers to Claude Code.
// This duplicates the unexported function in session/tmux to avoid exporting it.
func isClaudeProgram(program string) bool {
	return strings.HasSuffix(program, "claude")
}
