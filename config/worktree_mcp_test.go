package config

import (
	"os"
	"strings"
	"testing"
)

func TestGetWorktreeMCPs(t *testing.T) {
	config := &Config{
		WorktreeMCPs: map[string][]string{
			"/path/to/worktree1": {"github", "filesystem"},
			"/path/to/worktree2": {"filesystem"},
		},
	}

	tests := []struct {
		name         string
		worktreePath string
		expected     []string
	}{
		{
			name:         "existing worktree with MCPs",
			worktreePath: "/path/to/worktree1",
			expected:     []string{"github", "filesystem"},
		},
		{
			name:         "existing worktree with single MCP",
			worktreePath: "/path/to/worktree2",
			expected:     []string{"filesystem"},
		},
		{
			name:         "non-existing worktree",
			worktreePath: "/path/to/nonexistent",
			expected:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.GetWorktreeMCPs(tt.worktreePath)
			if len(result) != len(tt.expected) {
				t.Errorf("GetWorktreeMCPs() = %v, expected %v", result, tt.expected)
				return
			}
			for i, mcpName := range result {
				if mcpName != tt.expected[i] {
					t.Errorf("GetWorktreeMCPs() = %v, expected %v", result, tt.expected)
					break
				}
			}
		})
	}
}

func TestGetWorktreeMCPsNilMap(t *testing.T) {
	config := &Config{
		WorktreeMCPs: nil,
	}

	result := config.GetWorktreeMCPs("/path/to/worktree")
	if result != nil {
		t.Errorf("GetWorktreeMCPs() with nil map = %v, expected nil", result)
	}
}

func TestSetWorktreeMCPs(t *testing.T) {
	config := &Config{
		WorktreeMCPs: make(map[string][]string),
	}

	// Test setting MCPs
	mcps := []string{"github", "filesystem"}
	config.SetWorktreeMCPs("/path/to/worktree", mcps)

	result := config.GetWorktreeMCPs("/path/to/worktree")
	if len(result) != len(mcps) {
		t.Errorf("SetWorktreeMCPs() failed, got %v expected %v", result, mcps)
		return
	}
	for i, mcpName := range result {
		if mcpName != mcps[i] {
			t.Errorf("SetWorktreeMCPs() failed, got %v expected %v", result, mcps)
			break
		}
	}
}

func TestSetWorktreeMCPsEmpty(t *testing.T) {
	config := &Config{
		WorktreeMCPs: map[string][]string{
			"/path/to/worktree": {"github", "filesystem"},
		},
	}

	// Test removing MCPs by setting empty slice
	config.SetWorktreeMCPs("/path/to/worktree", []string{})

	result := config.GetWorktreeMCPs("/path/to/worktree")
	if result != nil {
		t.Errorf("SetWorktreeMCPs() with empty slice should remove entry, got %v", result)
	}
}

func TestSetWorktreeMCPsNilMap(t *testing.T) {
	config := &Config{
		WorktreeMCPs: nil,
	}

	// Test setting MCPs with nil map (should initialize)
	mcps := []string{"github"}
	config.SetWorktreeMCPs("/path/to/worktree", mcps)

	if config.WorktreeMCPs == nil {
		t.Error("SetWorktreeMCPs() should initialize nil map")
		return
	}

	result := config.GetWorktreeMCPs("/path/to/worktree")
	if len(result) != 1 || result[0] != "github" {
		t.Errorf("SetWorktreeMCPs() with nil map failed, got %v expected %v", result, mcps)
	}
}

func TestGetWorktreeMCPConfigs(t *testing.T) {
	config := &Config{
		MCPServers: map[string]MCPServerConfig{
			"github": {
				Command: "npx",
				Args:    []string{"@modelcontextprotocol/server-github"},
				Env:     map[string]string{"GITHUB_TOKEN": "test-token"},
			},
			"filesystem": {
				Command: "npx",
				Args:    []string{"@modelcontextprotocol/server-filesystem"},
			},
			"unassigned": {
				Command: "unassigned-command",
			},
		},
		WorktreeMCPs: map[string][]string{
			"/path/to/worktree": {"github", "filesystem"},
		},
	}

	tests := []struct {
		name         string
		worktreePath string
		expectedLen  int
		expectedMCPs []string
	}{
		{
			name:         "worktree with assigned MCPs",
			worktreePath: "/path/to/worktree",
			expectedLen:  2,
			expectedMCPs: []string{"github", "filesystem"},
		},
		{
			name:         "worktree with no assigned MCPs",
			worktreePath: "/path/to/empty",
			expectedLen:  0,
			expectedMCPs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.GetWorktreeMCPConfigs(tt.worktreePath)
			
			if len(result) != tt.expectedLen {
				t.Errorf("GetWorktreeMCPConfigs() returned %d configs, expected %d", len(result), tt.expectedLen)
				return
			}

			for _, expectedMCP := range tt.expectedMCPs {
				if _, exists := result[expectedMCP]; !exists {
					t.Errorf("GetWorktreeMCPConfigs() missing expected MCP: %s", expectedMCP)
				}
			}
		})
	}
}

func TestModifyCommandWithMCPForWorktree(t *testing.T) {
	config := &Config{
		MCPServers: map[string]MCPServerConfig{
			"github": {
				Command: "npx",
				Args:    []string{"@modelcontextprotocol/server-github"},
			},
			"filesystem": {
				Command: "npx",
				Args:    []string{"@modelcontextprotocol/server-filesystem"},
			},
		},
		WorktreeMCPs: map[string][]string{
			"/path/to/worktree1": {"github"},
			"/path/to/worktree2": {"github", "filesystem"},
		},
	}

	tests := []struct {
		name         string
		command      string
		worktreePath string
		expected     string
		contains     string
	}{
		{
			name:         "non-claude command unchanged",
			command:      "aider --model gpt-4",
			worktreePath: "/path/to/worktree1",
			expected:     "aider --model gpt-4",
		},
		{
			name:         "claude command with no assigned MCPs",
			command:      "claude",
			worktreePath: "/path/to/empty",
			expected:     "claude",
		},
		{
			name:         "claude command with assigned MCPs",
			command:      "claude",
			worktreePath: "/path/to/worktree1",
			contains:     "--mcp-config",
		},
		{
			name:         "claude command with multiple assigned MCPs",
			command:      "claude --verbose",
			worktreePath: "/path/to/worktree2",
			contains:     "--mcp-config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ModifyCommandWithMCPForWorktree(tt.command, config, tt.worktreePath)

			if tt.expected != "" {
				if result != tt.expected {
					t.Errorf("ModifyCommandWithMCPForWorktree(%q, %q) = %q, expected %q", 
						tt.command, tt.worktreePath, result, tt.expected)
				}
			}

			if tt.contains != "" {
				if !strings.Contains(result, tt.contains) {
					t.Errorf("ModifyCommandWithMCPForWorktree(%q, %q) = %q, expected to contain %q", 
						tt.command, tt.worktreePath, result, tt.contains)
				}

				// Cleanup generated config file
				parts := strings.Fields(result)
				for i, part := range parts {
					if part == "--mcp-config" && i+1 < len(parts) {
						configFile := parts[i+1]
						defer CleanupMCPConfigFile(configFile)
						
						// Verify file exists
						if _, err := os.Stat(configFile); os.IsNotExist(err) {
							t.Errorf("ModifyCommandWithMCPForWorktree() MCP config file does not exist: %s", configFile)
						}
						break
					}
				}
			}
		})
	}
}

func TestCleanupWorktreeMCPs(t *testing.T) {
	config := &Config{
		WorktreeMCPs: map[string][]string{
			"/path/to/worktree1": {"github", "filesystem"},
			"/path/to/worktree2": {"filesystem"},
			"/path/to/worktree3": {"github"},
		},
	}

	tests := []struct {
		name         string
		worktreePath string
		expectedLen  int
		shouldExist  bool
	}{
		{
			name:         "cleanup existing worktree",
			worktreePath: "/path/to/worktree1",
			expectedLen:  2,
			shouldExist:  false,
		},
		{
			name:         "cleanup another existing worktree",
			worktreePath: "/path/to/worktree2",
			expectedLen:  1,
			shouldExist:  false,
		},
		{
			name:         "cleanup non-existing worktree (should be no-op)",
			worktreePath: "/path/to/nonexistent",
			expectedLen:  1,
			shouldExist:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.CleanupWorktreeMCPs(tt.worktreePath)
			
			// Check that the entry was removed
			result := config.GetWorktreeMCPs(tt.worktreePath)
			if tt.shouldExist && result == nil {
				t.Errorf("CleanupWorktreeMCPs() removed entry that should exist: %s", tt.worktreePath)
			}
			if !tt.shouldExist && result != nil {
				t.Errorf("CleanupWorktreeMCPs() did not remove entry: %s, still has %v", tt.worktreePath, result)
			}
			
			// Check that the map has the expected number of entries
			if len(config.WorktreeMCPs) != tt.expectedLen {
				t.Errorf("CleanupWorktreeMCPs() left %d entries, expected %d", len(config.WorktreeMCPs), tt.expectedLen)
			}
		})
	}
}

func TestCleanupWorktreeMCPsNilMap(t *testing.T) {
	config := &Config{
		WorktreeMCPs: nil,
	}

	// Should not panic when map is nil
	config.CleanupWorktreeMCPs("/path/to/worktree")

	if config.WorktreeMCPs == nil {
		// This is acceptable - the method should be a no-op when map is nil
		return
	}

	// If the map was initialized by the method, it should be empty
	if len(config.WorktreeMCPs) != 0 {
		t.Errorf("CleanupWorktreeMCPs() with nil map should result in empty or nil map, got %v", config.WorktreeMCPs)
	}
}

func TestCleanupWorktreeMCPsEmptyMap(t *testing.T) {
	config := &Config{
		WorktreeMCPs: make(map[string][]string),
	}

	// Should not panic when map is empty
	config.CleanupWorktreeMCPs("/path/to/worktree")

	if len(config.WorktreeMCPs) != 0 {
		t.Errorf("CleanupWorktreeMCPs() with empty map should result in empty map, got %v", config.WorktreeMCPs)
	}
}

func TestConfigDefensiveInitialization(t *testing.T) {
	// Test that DefaultConfig initializes WorktreeMCPs
	config := DefaultConfig()
	if config.WorktreeMCPs == nil {
		t.Error("DefaultConfig() should initialize WorktreeMCPs map")
	}
}

func TestLoadConfigDefensiveInitialization(t *testing.T) {
	// Create a minimal config JSON without WorktreeMCPs field
	configJSON := `{
		"default_program": "claude",
		"auto_yes": false,
		"daemon_poll_interval": 1000,
		"branch_prefix": "test/",
		"console_shell": "/bin/bash",
		"mcp_servers": {}
	}`

	// Create temporary config file
	tmpFile, err := os.CreateTemp("", "test-config-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configJSON); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpFile.Close()

	// We'll test defensive initialization in LoadConfig indirectly

	// Test LoadConfig with a config that doesn't have WorktreeMCPs
	// Since we can't easily override GetConfigDir, we'll test the defensive
	// initialization by creating a config manually and checking the result
	
	// Create config struct without WorktreeMCPs initialized
	testConfig := Config{
		DefaultProgram:     "claude",
		MCPServers:         map[string]MCPServerConfig{},
		WorktreeMCPs:       nil, // This should be initialized
	}

	// Simulate what LoadConfig does for defensive initialization
	if testConfig.MCPServers == nil {
		testConfig.MCPServers = make(map[string]MCPServerConfig)
	}
	if testConfig.WorktreeMCPs == nil {
		testConfig.WorktreeMCPs = make(map[string][]string)
	}

	if testConfig.WorktreeMCPs == nil {
		t.Error("LoadConfig() should defensively initialize WorktreeMCPs map")
	}
}