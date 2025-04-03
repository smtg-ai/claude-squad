package config

import (
	"claude-squad/log"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const ConfigFileName = "config.json"

// Global config instance
var ConfigInstance *Config

// MCPServerConfig represents the configuration for an MCP server
type MCPServerConfig struct {
	// Command is the command to execute to start the MCP server
	Command string `json:"command"`
	// Args are the arguments to pass to the command
	Args []string `json:"args"`
	// Env is a map of environment variables to set for the MCP server
	Env map[string]string `json:"env,omitempty"`
}

// Config represents the application configuration
type Config struct {
	// DefaultProgram is the default program to run in new instances
	DefaultProgram string `json:"default_program"`
	// AutoYes is a flag to automatically accept all prompts.
	AutoYes bool `json:"auto_yes"`
	// DaemonPollInterval is the interval (ms) at which the daemon polls sessions for autoyes mode.
	DaemonPollInterval int `json:"daemon_poll_interval"`
	// MCPServers is a map of MCP server configurations.
	MCPServers map[string]MCPServerConfig `json:"mcp_servers,omitempty"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		DefaultProgram:     "claude",
		AutoYes:            false,
		DaemonPollInterval: 1000,
		MCPServers:         map[string]MCPServerConfig{},
	}
}

// GetConfigDir returns the path to the application's configuration directory
func GetConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get config home directory: %w", err)
	}
	return filepath.Join(homeDir, ".claude-squad"), nil
}

// LoadConfig loads the configuration from disk. If it cannot be done, we return the default configuration.
// This should only be called once (in main).
func LoadConfig() *Config {
	ConfigInstance = loadConfigFromDisk()
	return ConfigInstance
}

// loadConfigFromDisk is the internal function that actually loads the config from disk
func loadConfigFromDisk() *Config {
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
			if saveErr := SaveConfig(); saveErr != nil {
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

	ConfigInstance = &config
	return ConfigInstance
}

// ReloadConfig forces a reload of the config from disk
func ReloadConfig() *Config {
	ConfigInstance = loadConfigFromDisk()
	return ConfigInstance
}

// SaveConfig saves the configuration to disk
func SaveConfig() error {
	configDir, err := GetConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := filepath.Join(configDir, ConfigFileName)
	data, err := json.MarshalIndent(ConfigInstance, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(configPath, data, 0644)
}
