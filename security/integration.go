package security

import (
	"claude-squad/config"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SecurityIntegration handles integration of security with claude-squad
type SecurityIntegration struct {
	securityManager *SecurityManager
	appConfig       *config.Config
	configDir       string
}

// NewSecurityIntegration creates a new security integration
func NewSecurityIntegration(appConfig *config.Config) (*SecurityIntegration, error) {
	configDir, err := config.GetConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get config directory: %w", err)
	}

	securityConfigPath := filepath.Join(configDir, "security.json")
	securityConfig, err := loadSecurityConfig(securityConfigPath)
	if err != nil {
		// Create default config if it doesn't exist
		securityConfig = DefaultSecurityConfig()
		if err := saveSecurityConfig(securityConfig, securityConfigPath); err != nil {
			return nil, fmt.Errorf("failed to save default security config: %w", err)
		}
	}

	securityDir := filepath.Join(configDir, "security")
	securityManager, err := NewSecurityManager(securityConfig, securityDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create security manager: %w", err)
	}

	return &SecurityIntegration{
		securityManager: securityManager,
		appConfig:       appConfig,
		configDir:       configDir,
	}, nil
}

// GetSecurityManager returns the security manager
func (si *SecurityIntegration) GetSecurityManager() *SecurityManager {
	return si.securityManager
}

// CheckSessionPermission checks if a user can access a session
func (si *SecurityIntegration) CheckSessionPermission(userID, sessionOwner, action string) bool {
	if !si.securityManager.IsPermissionEnabled() {
		return true // Allow everything if permissions are disabled
	}

	// Create environment with session ownership info
	environment := map[string]interface{}{
		"resource_owner": sessionOwner,
	}

	// Create anonymous context for permission check
	context := &SecurityContext{
		User: &User{ID: userID},
		IsAuthenticated: userID != "",
		Environment: environment,
	}

	result := si.securityManager.CheckPermission(context, "session", action, "own", environment)
	return result.Granted
}

// CheckGitPermission checks if a user can perform git operations
func (si *SecurityIntegration) CheckGitPermission(userID, action string) bool {
	if !si.securityManager.IsPermissionEnabled() {
		return true
	}

	context := &SecurityContext{
		User: &User{ID: userID},
		IsAuthenticated: userID != "",
		Environment: make(map[string]interface{}),
	}

	result := si.securityManager.CheckPermission(context, "git", action, "repository", nil)
	return result.Granted
}

// CheckConfigPermission checks if a user can access configuration
func (si *SecurityIntegration) CheckConfigPermission(userID, action string) bool {
	if !si.securityManager.IsPermissionEnabled() {
		return true
	}

	context := &SecurityContext{
		User: &User{ID: userID},
		IsAuthenticated: userID != "",
		Environment: make(map[string]interface{}),
	}

	result := si.securityManager.CheckPermission(context, "config", action, "application", nil)
	return result.Granted
}

// LogSessionActivity logs session-related activity
func (si *SecurityIntegration) LogSessionActivity(userID, sessionName, action string, success bool, details map[string]interface{}) {
	var eventType AuditEventType
	switch action {
	case "create":
		eventType = AuditSessionCreated
	case "attach":
		eventType = AuditSessionAttached
	case "detach":
		eventType = AuditSessionDetached
	case "kill":
		eventType = AuditSessionKilled
	case "prompt":
		eventType = AuditPromptSent
	default:
		eventType = AuditEventType("session_" + action)
	}

	if details == nil {
		details = make(map[string]interface{})
	}
	details["session_name"] = sessionName

	si.securityManager.GetAuditLogger().LogEvent(AuditEvent{
		Type:     eventType,
		UserID:   userID,
		Success:  success,
		Severity: SeverityInfo,
		Details:  details,
	})
}

// LogGitActivity logs git-related activity
func (si *SecurityIntegration) LogGitActivity(userID, action string, success bool, details map[string]interface{}) {
	var eventType AuditEventType
	switch action {
	case "commit":
		eventType = AuditGitCommit
	case "push":
		eventType = AuditGitPush
	case "pull":
		eventType = AuditGitPull
	case "branch":
		eventType = AuditGitBranch
	case "checkout":
		eventType = AuditGitCheckout
	default:
		eventType = AuditEventType("git_" + action)
	}

	si.securityManager.GetAuditLogger().LogEvent(AuditEvent{
		Type:     eventType,
		UserID:   userID,
		Success:  success,
		Severity: SeverityInfo,
		Details:  details,
	})
}

// InitializeDefaultUser creates a default admin user if none exists
func (si *SecurityIntegration) InitializeDefaultUser() error {
	if !si.securityManager.IsAuthenticationEnabled() {
		return nil
	}

	// Check if any users exist
	// For now, we'll always try to create the default user
	// In a real implementation, you'd check the user store

	defaultUsername := "admin"
	defaultPassword := "claude-squad-admin" // Should be configurable
	defaultRoles := []string{"admin"}

	_, err := si.securityManager.CreateUser(defaultUsername, defaultPassword, defaultRoles)
	if err != nil {
		// User might already exist, which is fine
		return nil
	}

	fmt.Printf("Created default admin user: %s\n", defaultUsername)
	fmt.Printf("Default password: %s\n", defaultPassword)
	fmt.Println("Please change the default password immediately!")

	return nil
}

// GetSecurityStatus returns the current security status
func (si *SecurityIntegration) GetSecurityStatus() map[string]interface{} {
	return map[string]interface{}{
		"security_enabled":     si.securityManager.IsSecurityEnabled(),
		"authentication":       si.securityManager.IsAuthenticationEnabled(),
		"permissions":          si.securityManager.IsPermissionEnabled(),
		"audit":               si.securityManager.GetConfig().Audit.Enabled,
		"encryption_at_rest":  si.securityManager.GetConfig().General.EncryptionAtRest,
		"security_headers":    si.securityManager.GetConfig().General.SecurityHeaders,
		"rate_limiting":       si.securityManager.GetConfig().General.RateLimiting.Enabled,
		"developer_mode":      si.securityManager.GetConfig().General.DeveloperMode,
	}
}

// Stop stops the security integration
func (si *SecurityIntegration) Stop() error {
	return si.securityManager.Stop()
}

// Helper functions

func loadSecurityConfig(configPath string) (*SecurityConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config SecurityConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse security config: %w", err)
	}

	return &config, nil
}

func saveSecurityConfig(config *SecurityConfig, configPath string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal security config: %w", err)
	}

	return os.WriteFile(configPath, data, 0600)
}

// SecurityWrapper wraps existing functions to add security checks
type SecurityWrapper struct {
	integration *SecurityIntegration
	userID      string
}

// NewSecurityWrapper creates a new security wrapper
func NewSecurityWrapper(integration *SecurityIntegration, userID string) *SecurityWrapper {
	return &SecurityWrapper{
		integration: integration,
		userID:      userID,
	}
}

// WrapSessionCreate wraps session creation with security checks
func (sw *SecurityWrapper) WrapSessionCreate(originalFunc func() error, sessionName string) error {
	// Check permission
	if !sw.integration.CheckSessionPermission(sw.userID, sw.userID, "create") {
		return fmt.Errorf("permission denied: cannot create session")
	}

	// Execute original function
	err := originalFunc()

	// Log activity
	sw.integration.LogSessionActivity(sw.userID, sessionName, "create", err == nil, nil)

	return err
}

// WrapSessionAttach wraps session attachment with security checks
func (sw *SecurityWrapper) WrapSessionAttach(originalFunc func() error, sessionName, sessionOwner string) error {
	// Check permission
	if !sw.integration.CheckSessionPermission(sw.userID, sessionOwner, "attach") {
		return fmt.Errorf("permission denied: cannot attach to session")
	}

	// Execute original function
	err := originalFunc()

	// Log activity
	sw.integration.LogSessionActivity(sw.userID, sessionName, "attach", err == nil, nil)

	return err
}

// WrapGitOperation wraps git operations with security checks
func (sw *SecurityWrapper) WrapGitOperation(originalFunc func() error, action string, details map[string]interface{}) error {
	// Check permission
	if !sw.integration.CheckGitPermission(sw.userID, action) {
		return fmt.Errorf("permission denied: cannot perform git %s", action)
	}

	// Execute original function
	err := originalFunc()

	// Log activity
	sw.integration.LogGitActivity(sw.userID, action, err == nil, details)

	return err
}

// WrapConfigAccess wraps configuration access with security checks
func (sw *SecurityWrapper) WrapConfigAccess(originalFunc func() error, action string) error {
	// Check permission
	if !sw.integration.CheckConfigPermission(sw.userID, action) {
		return fmt.Errorf("permission denied: cannot %s configuration", action)
	}

	// Execute original function
	err := originalFunc()

	// Log activity if it's a write operation
	if action == "write" {
		sw.integration.securityManager.LogSecurityEvent(
			AuditConfigChanged,
			sw.userID,
			"",
			map[string]interface{}{"action": action},
		)
	}

	return err
}