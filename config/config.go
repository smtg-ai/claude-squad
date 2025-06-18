package config

import (
	"claude-squad/log"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	ConfigFileName = "config.json"
	defaultProgram = "claude"
)

// MCPServerConfig represents the configuration for an MCP server
type MCPServerConfig struct {
	Type    string            `json:"type"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
}

// GetConfigDir returns the path to the application's configuration directory
func GetConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get config home directory: %w", err)
	}
	return filepath.Join(homeDir, ".claude-squad"), nil
}

// Config represents the application configuration
type Config struct {
	// DefaultProgram is the default program to run in new instances
	DefaultProgram string `json:"default_program"`
	// AutoYes is a flag to automatically accept all prompts.
	AutoYes bool `json:"auto_yes"`
	// DaemonPollInterval is the interval (ms) at which the daemon polls sessions for autoyes mode.
	DaemonPollInterval int `json:"daemon_poll_interval"`
	// BranchPrefix is the prefix used for git branches created by the application.
	BranchPrefix string `json:"branch_prefix"`
	// ConsoleShell is the shell command to use in the console tab.
	ConsoleShell string `json:"console_shell"`
	// MCPServers is a map of MCP server configurations
	MCPServers map[string]MCPServerConfig `json:"mcp_servers,omitempty"`
	// WorktreeMCPs maps worktree paths to assigned MCP server names
	WorktreeMCPs map[string][]string `json:"worktree_mcps,omitempty"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	program, err := GetClaudeCommand()
	if err != nil {
		log.ErrorLog.Printf("failed to get claude command: %v", err)
		program = defaultProgram
	}

	// Get default shell from environment or fallback to bash
	defaultShell := os.Getenv("SHELL")
	if defaultShell == "" {
		defaultShell = "/bin/bash"
	}

	return &Config{
		DefaultProgram:     program,
		AutoYes:            false,
		DaemonPollInterval: 1000,
		BranchPrefix: func() string {
			user, err := user.Current()
			if err != nil || user == nil || user.Username == "" {
				log.ErrorLog.Printf("failed to get current user: %v", err)
				return "session/"
			}
			return fmt.Sprintf("%s/", strings.ToLower(user.Username))
		}(),
		ConsoleShell: defaultShell,
		MCPServers:   make(map[string]MCPServerConfig),
		WorktreeMCPs: make(map[string][]string),
	}
}

// GetClaudeCommand attempts to find the "claude" command in the user's shell
// It checks in the following order:
// 1. Shell alias resolution: using "which" command
// 2. PATH lookup
//
// If both fail, it returns an error.
func GetClaudeCommand() (string, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash" // Default to bash if SHELL is not set
	}

	// Force the shell to load the user's profile and then run the command
	// For zsh, source .zshrc; for bash, source .bashrc
	var shellCmd string
	if strings.Contains(shell, "zsh") {
		shellCmd = "source ~/.zshrc 2>/dev/null || true; which claude"
	} else if strings.Contains(shell, "bash") {
		shellCmd = "source ~/.bashrc 2>/dev/null || true; which claude"
	} else {
		shellCmd = "which claude"
	}

	cmd := exec.Command(shell, "-c", shellCmd)
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		path := strings.TrimSpace(string(output))
		if path != "" {
			// Check if the output is an alias definition and extract the actual path
			// Handle formats like "claude: aliased to /path/to/claude" or other shell-specific formats
			aliasRegex := regexp.MustCompile(`(?:aliased to|->|=)\s*([^\s]+)`)
			matches := aliasRegex.FindStringSubmatch(path)
			if len(matches) > 1 {
				path = matches[1]
			}
			return path, nil
		}
	}

	// Otherwise, try to find in PATH directly
	claudePath, err := exec.LookPath("claude")
	if err == nil {
		return claudePath, nil
	}

	return "", fmt.Errorf("claude command not found in aliases or PATH")
}

func LoadConfig() *Config {
	configDir, err := GetConfigDir()
	if err != nil {
		log.ErrorLog.Printf("failed to get config directory: %v", err)
		return DefaultConfig()
	}

	configPath := filepath.Join(configDir, ConfigFileName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create and save default config if file doesn't exist
			defaultCfg := DefaultConfig()
			if saveErr := saveConfig(defaultCfg); saveErr != nil {
				log.WarningLog.Printf("failed to save default config: %v", saveErr)
			}
			return defaultCfg
		}

		log.WarningLog.Printf("failed to get config file: %v", err)
		return DefaultConfig()
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		log.ErrorLog.Printf("failed to parse config file: %v", err)
		return DefaultConfig()
	}

	// DEFENSIVE INITIALIZATION: Ensure MCPServers map is never nil
	if config.MCPServers == nil {
		config.MCPServers = make(map[string]MCPServerConfig)
	}
	if config.WorktreeMCPs == nil {
		config.WorktreeMCPs = make(map[string][]string)
	}

	// Normalize MCP server configurations to ensure they have proper format
	for name, serverConfig := range config.MCPServers {
		config.MCPServers[name] = normalizeMCPServerConfig(serverConfig)
	}

	// Clean up stale worktree MCP assignments
	config.CleanupStaleWorktreeMCPs()

	return &config
}

// saveConfig saves the configuration to disk
func saveConfig(config *Config) error {
	configDir, err := GetConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := filepath.Join(configDir, ConfigFileName)
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(configPath, data, 0644)
}

// SaveConfig exports the saveConfig function for use by other packages
func SaveConfig(config *Config) error {
	return saveConfig(config)
}

// isClaudeCommand checks if the given program command is a Claude command
func isClaudeCommand(program string) bool {
	if program == "" {
		return false
	}

	// Normalize the program string for comparison
	normalized := strings.ToLower(strings.TrimSpace(program))

	// Extract the base command name from a path or command with arguments
	parts := strings.Fields(normalized)
	if len(parts) == 0 {
		return false
	}

	baseCommand := filepath.Base(parts[0])

	// Check if the base command contains "claude"
	return strings.Contains(baseCommand, "claude")
}

// parseCommandString splits a command string into command and args for proper MCP format
func parseCommandString(commandStr string) (string, []string) {
	parts := strings.Fields(commandStr)
	if len(parts) == 0 {
		return "", []string{}
	}
	if len(parts) == 1 {
		return parts[0], []string{}
	}
	return parts[0], parts[1:]
}

// normalizeMCPServerConfig ensures MCP server config has all required fields in correct format
func normalizeMCPServerConfig(config MCPServerConfig) MCPServerConfig {
	// Ensure type is set (default to stdio for Claude Code compatibility)
	if config.Type == "" {
		config.Type = "stdio"
	}

	// Handle legacy format where command might contain arguments
	// If Args is nil or empty but Command contains spaces, split the command
	if (config.Args == nil || len(config.Args) == 0) && strings.Contains(config.Command, " ") {
		command, args := parseCommandString(config.Command)
		config.Command = command
		config.Args = args
	}

	// Ensure Args is never nil (Claude Code expects an array, not null)
	if config.Args == nil {
		config.Args = []string{}
	}

	// Ensure Env is never nil
	if config.Env == nil {
		config.Env = make(map[string]string)
	}

	return config
}

// generateMCPConfigFile creates a temporary MCP configuration file
func generateMCPConfigFile(mcpServers map[string]MCPServerConfig) (string, error) {
	// Handle empty or nil servers by returning empty string (no config file needed)
	if len(mcpServers) == 0 {
		return "", nil
	}

	// Normalize all MCP server configurations
	normalizedServers := make(map[string]MCPServerConfig)
	for name, config := range mcpServers {
		normalizedServers[name] = normalizeMCPServerConfig(config)
	}

	// Create MCP configuration structure
	mcpConfig := map[string]interface{}{
		"mcpServers": normalizedServers,
	}

	// Marshal to JSON
	configData, err := json.MarshalIndent(mcpConfig, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal MCP config: %w", err)
	}

	// Create temporary file with MCP-specific naming pattern
	tmpFile, err := ioutil.TempFile("", "claude-mcp-config-*.json")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmpFile.Close()

	// Write configuration to file
	if _, err := tmpFile.Write(configData); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to write MCP config: %w", err)
	}

	return tmpFile.Name(), nil
}

// generateMCPConfigWithRetry attempts to generate MCP config file with retry logic
func generateMCPConfigWithRetry(mcpServers map[string]MCPServerConfig, maxRetries int) (string, error) {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		configFile, err := generateMCPConfigFile(mcpServers)
		if err == nil {
			return configFile, nil
		}

		lastErr = err
		if log.WarningLog != nil {
			log.WarningLog.Printf("MCP config generation attempt %d failed: %v", attempt+1, err)
		}

		// Exponential backoff delay
		if attempt < maxRetries-1 {
			delay := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			time.Sleep(delay)
		}
	}

	return "", fmt.Errorf("failed to generate MCP config after %d attempts: %w", maxRetries, lastErr)
}

// GetWorktreeMCPs returns the MCP server names assigned to a specific worktree path
func (c *Config) GetWorktreeMCPs(worktreePath string) []string {
	if c.WorktreeMCPs == nil {
		return nil
	}
	return c.WorktreeMCPs[worktreePath]
}

// SetWorktreeMCPs assigns MCP server names to a specific worktree path
func (c *Config) SetWorktreeMCPs(worktreePath string, mcpNames []string) {
	if c.WorktreeMCPs == nil {
		c.WorktreeMCPs = make(map[string][]string)
	}
	if len(mcpNames) == 0 {
		delete(c.WorktreeMCPs, worktreePath)
	} else {
		c.WorktreeMCPs[worktreePath] = mcpNames
	}
}

// GetWorktreeMCPConfigs returns the MCP configurations for a specific worktree
func (c *Config) GetWorktreeMCPConfigs(worktreePath string) map[string]MCPServerConfig {
	assignedMCPs := c.GetWorktreeMCPs(worktreePath)
	if len(assignedMCPs) == 0 {
		return nil
	}

	configs := make(map[string]MCPServerConfig)
	for _, mcpName := range assignedMCPs {
		if mcpConfig, exists := c.MCPServers[mcpName]; exists {
			configs[mcpName] = mcpConfig
		}
	}
	return configs
}

// ModifyCommandWithMCPForWorktree modifies a command to include MCP configuration for a specific worktree
func ModifyCommandWithMCPForWorktree(originalCommand string, config *Config, worktreePath string) string {
	if config == nil || !isClaudeCommand(originalCommand) {
		return originalCommand
	}

	// Clean any existing --mcp-config flags from the command
	cleanCommand := cleanMCPConfigFromCommand(originalCommand)

	// Get worktree-specific MCP configurations
	worktreeMCPs := config.GetWorktreeMCPConfigs(worktreePath)
	if len(worktreeMCPs) == 0 {
		return cleanCommand // Return clean command without MCPs
	}

	configFile, err := generateMCPConfigWithRetry(worktreeMCPs, 3)
	if err != nil {
		if log.ErrorLog != nil {
			log.ErrorLog.Printf("MCP config failed for worktree %s, running Claude without MCPs: %v", worktreePath, err)
		}
		return cleanCommand // Graceful fallback without MCPs
	}

	return cleanCommand + " --mcp-config " + configFile
}

// ModifyCommandWithMCP modifies a command to include MCP configuration if it's a Claude command
// DEPRECATED: Use ModifyCommandWithMCPForWorktree for worktree-specific MCP assignment
func ModifyCommandWithMCP(originalCommand string, config *Config) string {
	if config == nil || !isClaudeCommand(originalCommand) || len(config.MCPServers) == 0 {
		return originalCommand
	}

	configFile, err := generateMCPConfigWithRetry(config.MCPServers, 3)
	if err != nil {
		if log.ErrorLog != nil {
			log.ErrorLog.Printf("MCP config failed, running Claude without MCPs: %v", err)
		}
		return originalCommand // Graceful fallback
	}

	return originalCommand + " --mcp-config " + configFile
}

// CleanupWorktreeMCPs removes MCP assignments for a specific worktree path
// This should be called when a worktree is deleted to avoid stale configuration
func (c *Config) CleanupWorktreeMCPs(worktreePath string) {
	if c.WorktreeMCPs == nil {
		return
	}
	delete(c.WorktreeMCPs, worktreePath)
}

// CleanupStaleWorktreeMCPs removes MCP assignments for worktrees that no longer exist
func (c *Config) CleanupStaleWorktreeMCPs() {
	if c.WorktreeMCPs == nil {
		return
	}

	stalePaths := make([]string, 0)
	for worktreePath := range c.WorktreeMCPs {
		// Check if the worktree path still exists
		if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
			stalePaths = append(stalePaths, worktreePath)
		}
	}

	// Remove stale assignments
	for _, stalePath := range stalePaths {
		if log.InfoLog != nil {
			log.InfoLog.Printf("Removing stale worktree MCP assignment: %s", stalePath)
		}
		delete(c.WorktreeMCPs, stalePath)
	}

	// Save config if we cleaned up any stale entries
	if len(stalePaths) > 0 {
		if err := SaveConfig(c); err != nil {
			if log.ErrorLog != nil {
				log.ErrorLog.Printf("Failed to save config after cleanup: %v", err)
			}
		}
	}
}

// cleanMCPConfigFromCommand removes all --mcp-config flags and their arguments from a command
func cleanMCPConfigFromCommand(command string) string {
	// Split command into parts
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return command
	}

	var cleanParts []string
	i := 0
	for i < len(parts) {
		if parts[i] == "--mcp-config" {
			// Skip this flag and its argument (next part)
			i += 2 // Skip both "--mcp-config" and the config file path
		} else {
			cleanParts = append(cleanParts, parts[i])
			i++
		}
	}

	return strings.Join(cleanParts, " ")
}

// CleanupMCPConfigFile removes the temporary MCP configuration file
// Only removes files that follow the MCP config naming pattern for safety
func CleanupMCPConfigFile(configFile string) error {
	if configFile == "" {
		return nil
	}

	// Safety check: only remove files that look like MCP config files
	baseName := filepath.Base(configFile)
	if !strings.Contains(baseName, "claude-mcp-config-") || !strings.HasSuffix(baseName, ".json") {
		// Not an MCP config file, don't remove it
		return nil
	}

	err := os.Remove(configFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to cleanup MCP config file: %w", err)
	}

	return nil
}
