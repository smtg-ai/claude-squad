package config

import (
	"os"
	"strings"
	"testing"
)

func TestIsClaudeCommand(t *testing.T) {
	tests := []struct {
		name     string
		program  string
		expected bool
	}{
		{
			name:     "simple claude command",
			program:  "claude",
			expected: true,
		},
		{
			name:     "claude with path",
			program:  "/usr/local/bin/claude",
			expected: true,
		},
		{
			name:     "claude with arguments",
			program:  "claude --help",
			expected: true,
		},
		{
			name:     "claude with full path and arguments",
			program:  "/usr/local/bin/claude --version",
			expected: true,
		},
		{
			name:     "aider command",
			program:  "aider",
			expected: false,
		},
		{
			name:     "aider with arguments",
			program:  "aider --model gpt-4",
			expected: false,
		},
		{
			name:     "python command",
			program:  "python",
			expected: false,
		},
		{
			name:     "empty command",
			program:  "",
			expected: false,
		},
		{
			name:     "space only command",
			program:  "   ",
			expected: false,
		},
		{
			name:     "claude-like command",
			program:  "claude-code",
			expected: true,
		},
		{
			name:     "command containing claude",
			program:  "/home/user/claude-wrapper",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isClaudeCommand(tt.program)
			if result != tt.expected {
				t.Errorf("isClaudeCommand(%q) = %v, expected %v", tt.program, result, tt.expected)
			}
		})
	}
}

func TestGenerateMCPConfigFile(t *testing.T) {
	tests := []struct {
		name        string
		mcpServers  map[string]MCPServerConfig
		expectError bool
		expectEmpty bool
	}{
		{
			name:        "empty MCP servers",
			mcpServers:  map[string]MCPServerConfig{},
			expectError: false,
			expectEmpty: true,
		},
		{
			name:        "nil MCP servers",
			mcpServers:  nil,
			expectError: false,
			expectEmpty: true,
		},
		{
			name: "single MCP server",
			mcpServers: map[string]MCPServerConfig{
				"github": {
					Command: "npx",
					Args:    []string{"@modelcontextprotocol/server-github"},
					Env:     map[string]string{"GITHUB_TOKEN": "test-token"},
				},
			},
			expectError: false,
			expectEmpty: false,
		},
		{
			name: "multiple MCP servers",
			mcpServers: map[string]MCPServerConfig{
				"github": {
					Command: "npx",
					Args:    []string{"@modelcontextprotocol/server-github"},
					Env:     map[string]string{"GITHUB_TOKEN": "test-token"},
				},
				"filesystem": {
					Command: "npx",
					Args:    []string{"@modelcontextprotocol/server-filesystem"},
					Env:     map[string]string{},
				},
			},
			expectError: false,
			expectEmpty: false,
		},
		{
			name: "MCP server with no args or env",
			mcpServers: map[string]MCPServerConfig{
				"simple": {
					Command: "simple-mcp-server",
				},
			},
			expectError: false,
			expectEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configFile, err := generateMCPConfigFile(tt.mcpServers)

			if tt.expectError && err == nil {
				t.Errorf("generateMCPConfigFile() expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("generateMCPConfigFile() unexpected error: %v", err)
			}

			if tt.expectEmpty && configFile != "" {
				t.Errorf("generateMCPConfigFile() expected empty config file but got: %s", configFile)
			}
			if !tt.expectEmpty && configFile == "" && err == nil {
				t.Errorf("generateMCPConfigFile() expected config file but got empty result")
			}

			// Cleanup generated file
			if configFile != "" {
				defer CleanupMCPConfigFile(configFile)

				// Verify file exists and contains expected content
				if _, err := os.Stat(configFile); os.IsNotExist(err) {
					t.Errorf("generateMCPConfigFile() created file does not exist: %s", configFile)
				}

				// Read and verify content
				content, err := os.ReadFile(configFile)
				if err != nil {
					t.Errorf("generateMCPConfigFile() failed to read created file: %v", err)
				}

				contentStr := string(content)
				if !strings.Contains(contentStr, "mcpServers") {
					t.Errorf("generateMCPConfigFile() file content missing 'mcpServers' key")
				}
			}
		})
	}
}

func TestGenerateMCPConfigWithRetry(t *testing.T) {
	tests := []struct {
		name        string
		mcpServers  map[string]MCPServerConfig
		maxRetries  int
		expectError bool
	}{
		{
			name: "successful generation",
			mcpServers: map[string]MCPServerConfig{
				"test": {
					Command: "test-command",
				},
			},
			maxRetries:  3,
			expectError: false,
		},
		{
			name:        "empty servers with retries",
			mcpServers:  map[string]MCPServerConfig{},
			maxRetries:  3,
			expectError: false,
		},
		{
			name: "valid servers zero retries",
			mcpServers: map[string]MCPServerConfig{
				"test": {
					Command: "test-command",
				},
			},
			maxRetries:  0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configFile, err := generateMCPConfigWithRetry(tt.mcpServers, tt.maxRetries)

			if tt.expectError && err == nil {
				t.Errorf("generateMCPConfigWithRetry() expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("generateMCPConfigWithRetry() unexpected error: %v", err)
			}

			// Cleanup generated file
			if configFile != "" {
				defer CleanupMCPConfigFile(configFile)
			}
		})
	}
}

func TestModifyCommandWithMCP(t *testing.T) {
	tests := []struct {
		name       string
		program    string
		mcpServers map[string]MCPServerConfig
		expected   string
		contains   string
	}{
		{
			name:    "non-claude command unchanged",
			program: "aider --model gpt-4",
			mcpServers: map[string]MCPServerConfig{
				"test": {Command: "test-mcp"},
			},
			expected: "aider --model gpt-4",
		},
		{
			name:       "claude command no MCP servers",
			program:    "claude",
			mcpServers: map[string]MCPServerConfig{},
			expected:   "claude",
		},
		{
			name:    "claude command with MCP servers",
			program: "claude",
			mcpServers: map[string]MCPServerConfig{
				"github": {
					Command: "npx",
					Args:    []string{"@modelcontextprotocol/server-github"},
				},
			},
			contains: "--mcp-config",
		},
		{
			name:    "claude with args and MCP servers",
			program: "claude --verbose",
			mcpServers: map[string]MCPServerConfig{
				"filesystem": {
					Command: "filesystem-mcp",
				},
			},
			contains: "--mcp-config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				MCPServers: tt.mcpServers,
			}

			result := ModifyCommandWithMCP(tt.program, config)

			if tt.expected != "" {
				if result != tt.expected {
					t.Errorf("ModifyCommandWithMCP(%q) = %q, expected %q", tt.program, result, tt.expected)
				}
			}

			if tt.contains != "" {
				if !strings.Contains(result, tt.contains) {
					t.Errorf("ModifyCommandWithMCP(%q) = %q, expected to contain %q", tt.program, result, tt.contains)
				}

				// For MCP-modified commands, verify the file exists and cleanup
				parts := strings.Fields(result)
				for i, part := range parts {
					if part == "--mcp-config" && i+1 < len(parts) {
						configFile := parts[i+1]
						defer CleanupMCPConfigFile(configFile)

						if _, err := os.Stat(configFile); os.IsNotExist(err) {
							t.Errorf("ModifyCommandWithMCP() MCP config file does not exist: %s", configFile)
						}
						break
					}
				}
			}
		})
	}
}

func TestCleanupMCPConfigFile(t *testing.T) {
	// Create a temporary file that looks like an MCP config file
	tempFile, err := os.CreateTemp("", "claude-mcp-config-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()

	// Verify file exists
	if _, err := os.Stat(tempFile.Name()); os.IsNotExist(err) {
		t.Fatalf("Temp file should exist: %s", tempFile.Name())
	}

	// Cleanup the file
	CleanupMCPConfigFile(tempFile.Name())

	// Verify file is removed
	if _, err := os.Stat(tempFile.Name()); !os.IsNotExist(err) {
		t.Errorf("CleanupMCPConfigFile() failed to remove file: %s", tempFile.Name())
	}
}

func TestCleanupMCPConfigFileNonMCP(t *testing.T) {
	// Create a temporary file that doesn't look like an MCP config file
	tempFile, err := os.CreateTemp("", "normal-file-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()

	// Verify file exists
	if _, err := os.Stat(tempFile.Name()); os.IsNotExist(err) {
		t.Fatalf("Temp file should exist: %s", tempFile.Name())
	}

	// Try to cleanup the file (should not remove it)
	CleanupMCPConfigFile(tempFile.Name())

	// Verify file still exists (not removed)
	if _, err := os.Stat(tempFile.Name()); os.IsNotExist(err) {
		t.Errorf("CleanupMCPConfigFile() should not remove non-MCP files: %s", tempFile.Name())
	}

	// Manual cleanup
	os.Remove(tempFile.Name())
}
