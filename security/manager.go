package security

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SecurityManager is the central security coordinator for claude-squad
type SecurityManager struct {
	authManager       *AuthenticationManager
	permissionManager *PermissionManager
	auditLogger       *AuditLogger
	config           *SecurityConfig
	storageDir       string
	initialized      bool
}

// SecurityConfig represents the overall security configuration
type SecurityConfig struct {
	Authentication AuthConfig        `json:"authentication"`
	Permissions    PermissionConfig  `json:"permissions"`
	Audit          AuditConfig       `json:"audit"`
	General        GeneralConfig     `json:"general"`
}

// GeneralConfig represents general security settings
type GeneralConfig struct {
	Enabled                bool          `json:"enabled"`
	EnforceHTTPS          bool          `json:"enforce_https"`
	SessionSecurity       SessionConfig `json:"session_security"`
	RateLimiting          RateConfig    `json:"rate_limiting"`
	IPWhitelist           []string      `json:"ip_whitelist"`
	IPBlacklist           []string      `json:"ip_blacklist"`
	SecurityHeaders       bool          `json:"security_headers"`
	ContentSecurityPolicy string        `json:"content_security_policy"`
	EncryptionAtRest      bool          `json:"encryption_at_rest"`
	RequireEncryption     bool          `json:"require_encryption"`
	TrustedProxies        []string      `json:"trusted_proxies"`
	DeveloperMode         bool          `json:"developer_mode"`
}

// SessionConfig represents session security configuration
type SessionConfig struct {
	SecureCookies    bool          `json:"secure_cookies"`
	HTTPOnlyCookies  bool          `json:"http_only_cookies"`
	SameSiteCookies  string        `json:"same_site_cookies"`
	SessionTimeout   time.Duration `json:"session_timeout"`
	IdleTimeout      time.Duration `json:"idle_timeout"`
	RegenerateID     bool          `json:"regenerate_id"`
	BindToIP         bool          `json:"bind_to_ip"`
	MultipleLogins   bool          `json:"multiple_logins"`
}

// RateConfig represents rate limiting configuration
type RateConfig struct {
	Enabled        bool          `json:"enabled"`
	LoginAttempts  int           `json:"login_attempts"`
	TimeWindow     time.Duration `json:"time_window"`
	BlockDuration  time.Duration `json:"block_duration"`
	GlobalLimit    int           `json:"global_limit"`
	PerUserLimit   int           `json:"per_user_limit"`
	PerIPLimit     int           `json:"per_ip_limit"`
}

// SecurityContext represents the current security context
type SecurityContext struct {
	User          *User
	Session       *Session
	Permissions   []*Permission
	IPAddress     string
	UserAgent     string
	IsAuthenticated bool
	IsAdmin       bool
	Environment   map[string]interface{}
}

// NewSecurityManager creates a new security manager
func NewSecurityManager(config *SecurityConfig, storageDir string) (*SecurityManager, error) {
	// Create storage directory
	if err := os.MkdirAll(storageDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create security storage directory: %w", err)
	}

	// Initialize audit logger first (other components depend on it)
	auditLogger, err := NewAuditLogger(&config.Audit, filepath.Join(storageDir, "audit"))
	if err != nil {
		return nil, fmt.Errorf("failed to create audit logger: %w", err)
	}

	// Initialize permission manager
	permissionManager, err := NewPermissionManager(&config.Permissions, auditLogger, filepath.Join(storageDir, "permissions"))
	if err != nil {
		return nil, fmt.Errorf("failed to create permission manager: %w", err)
	}

	// Initialize authentication manager
	authManager, err := NewAuthenticationManager(&config.Authentication, auditLogger, filepath.Join(storageDir, "auth"))
	if err != nil {
		return nil, fmt.Errorf("failed to create authentication manager: %w", err)
	}

	sm := &SecurityManager{
		authManager:       authManager,
		permissionManager: permissionManager,
		auditLogger:       auditLogger,
		config:           config,
		storageDir:       storageDir,
		initialized:      true,
	}

	// Log security system initialization
	auditLogger.LogEvent(AuditEvent{
		Type:      AuditSystemStart,
		Timestamp: time.Now(),
		Success:   true,
		Severity:  SeverityInfo,
		Source:    "security_manager",
		Details: map[string]interface{}{
			"auth_enabled":        config.Authentication.Enabled,
			"permissions_enabled": config.Permissions.Enabled,
			"audit_enabled":       config.Audit.Enabled,
		},
	})

	return sm, nil
}

// Authenticate authenticates a user and returns a security context
func (sm *SecurityManager) Authenticate(username, password string, metadata map[string]string) (*SecurityContext, error) {
	if !sm.config.Authentication.Enabled {
		// Create anonymous context when auth is disabled
		return &SecurityContext{
			IsAuthenticated: false,
			IPAddress:      metadata["ip_address"],
			UserAgent:      metadata["user_agent"],
			Environment:    make(map[string]interface{}),
		}, nil
	}

	// Perform authentication
	session, err := sm.authManager.Authenticate(username, password, metadata)
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Get user
	user, _, err := sm.authManager.ValidateSession(session.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to validate session: %w", err)
	}

	// Get user permissions
	permissions, err := sm.permissionManager.GetUserPermissions(user.Roles)
	if err != nil {
		return nil, fmt.Errorf("failed to get user permissions: %w", err)
	}

	// Create security context
	context := &SecurityContext{
		User:            user,
		Session:         session,
		Permissions:     permissions,
		IPAddress:       session.IPAddress,
		UserAgent:       session.UserAgent,
		IsAuthenticated: true,
		IsAdmin:         sm.permissionManager.IsAdmin(user.Roles),
		Environment:     make(map[string]interface{}),
	}

	return context, nil
}

// ValidateSession validates a session token and returns security context
func (sm *SecurityManager) ValidateSession(token string) (*SecurityContext, error) {
	if !sm.config.Authentication.Enabled {
		return &SecurityContext{
			IsAuthenticated: false,
			Environment:     make(map[string]interface{}),
		}, nil
	}

	// Validate session
	user, session, err := sm.authManager.ValidateSession(token)
	if err != nil {
		return nil, fmt.Errorf("session validation failed: %w", err)
	}

	// Get user permissions
	permissions, err := sm.permissionManager.GetUserPermissions(user.Roles)
	if err != nil {
		return nil, fmt.Errorf("failed to get user permissions: %w", err)
	}

	// Create security context
	context := &SecurityContext{
		User:            user,
		Session:         session,
		Permissions:     permissions,
		IPAddress:       session.IPAddress,
		UserAgent:       session.UserAgent,
		IsAuthenticated: true,
		IsAdmin:         sm.permissionManager.IsAdmin(user.Roles),
		Environment:     make(map[string]interface{}),
	}

	return context, nil
}

// CheckPermission checks if a user has permission to perform an action
func (sm *SecurityManager) CheckPermission(context *SecurityContext, resource, action, scope string, environment map[string]interface{}) *PermissionResult {
	if !sm.config.Permissions.Enabled {
		return &PermissionResult{
			Granted:     true,
			Reason:      "Permission system disabled",
			EvaluatedAt: time.Now(),
		}
	}

	// Prepare permission context
	permContext := &PermissionContext{
		Resource:    resource,
		Action:      action,
		Scope:       scope,
		Environment: environment,
		Timestamp:   time.Now(),
	}

	if context.IsAuthenticated {
		permContext.UserID = context.User.ID
		permContext.UserRoles = context.User.Roles
	}

	// Check permission
	result := sm.permissionManager.CheckPermission(permContext)

	// Log permission check
	sm.auditLogger.LogPermissionCheck(
		permContext.UserID,
		resource,
		action,
		scope,
		result.Granted,
		result.Reason,
	)

	return result
}

// RequirePermission checks permission and panics if denied
func (sm *SecurityManager) RequirePermission(context *SecurityContext, resource, action, scope string, environment map[string]interface{}) {
	result := sm.CheckPermission(context, resource, action, scope, environment)
	if !result.Granted {
		panic(fmt.Sprintf("Permission denied: %s", result.Reason))
	}
}

// CreateUser creates a new user
func (sm *SecurityManager) CreateUser(username, password string, roles []string) (*User, error) {
	user, err := sm.authManager.CreateUser(username, password, roles)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// Logout revokes a user session
func (sm *SecurityManager) Logout(token string) error {
	if !sm.config.Authentication.Enabled {
		return nil
	}

	return sm.authManager.RevokeSession(token)
}

// ChangePassword changes a user's password
func (sm *SecurityManager) ChangePassword(username, oldPassword, newPassword string) error {
	return sm.authManager.ChangePassword(username, oldPassword, newPassword)
}

// GetSecurityContext creates a security context for a given user
func (sm *SecurityManager) GetSecurityContext(userID string) (*SecurityContext, error) {
	if !sm.config.Authentication.Enabled {
		return &SecurityContext{
			IsAuthenticated: false,
			Environment:     make(map[string]interface{}),
		}, nil
	}

	// This is a simplified version - in practice, you'd need to get user from storage
	// For now, we'll return an error
	return nil, fmt.Errorf("get security context by user ID not implemented")
}

// IsSecurityEnabled returns whether security is enabled
func (sm *SecurityManager) IsSecurityEnabled() bool {
	return sm.config.General.Enabled
}

// IsAuthenticationEnabled returns whether authentication is enabled
func (sm *SecurityManager) IsAuthenticationEnabled() bool {
	return sm.config.Authentication.Enabled
}

// IsPermissionEnabled returns whether permissions are enabled
func (sm *SecurityManager) IsPermissionEnabled() bool {
	return sm.config.Permissions.Enabled
}

// GetAuditLogger returns the audit logger
func (sm *SecurityManager) GetAuditLogger() *AuditLogger {
	return sm.auditLogger
}

// GetPermissionManager returns the permission manager
func (sm *SecurityManager) GetPermissionManager() *PermissionManager {
	return sm.permissionManager
}

// GetAuthenticationManager returns the authentication manager
func (sm *SecurityManager) GetAuthenticationManager() *AuthenticationManager {
	return sm.authManager
}

// GetConfig returns the security configuration
func (sm *SecurityManager) GetConfig() *SecurityConfig {
	return sm.config
}

// LogSecurityEvent logs a security-related event
func (sm *SecurityManager) LogSecurityEvent(eventType AuditEventType, userID, username string, details map[string]interface{}) {
	sm.auditLogger.LogEvent(AuditEvent{
		Type:      eventType,
		UserID:    userID,
		Username:  username,
		Timestamp: time.Now(),
		Success:   true,
		Severity:  SeverityInfo,
		Details:   details,
	})
}

// GetSecurityStats returns security-related statistics
func (sm *SecurityManager) GetSecurityStats() map[string]interface{} {
	auditStats := sm.auditLogger.GetStats()
	
	return map[string]interface{}{
		"audit_stats": auditStats,
		"system_status": map[string]interface{}{
			"auth_enabled":        sm.config.Authentication.Enabled,
			"permissions_enabled": sm.config.Permissions.Enabled,
			"audit_enabled":       sm.config.Audit.Enabled,
			"security_enabled":    sm.config.General.Enabled,
		},
		"timestamp": time.Now(),
	}
}

// ValidateIPAddress validates if an IP address is allowed
func (sm *SecurityManager) ValidateIPAddress(ipAddress string) bool {
	if !sm.config.General.Enabled {
		return true
	}

	// Check blacklist first
	for _, blacklistedIP := range sm.config.General.IPBlacklist {
		if ipAddress == blacklistedIP {
			return false
		}
	}

	// If whitelist is empty, allow all (except blacklisted)
	if len(sm.config.General.IPWhitelist) == 0 {
		return true
	}

	// Check whitelist
	for _, whitelistedIP := range sm.config.General.IPWhitelist {
		if ipAddress == whitelistedIP {
			return true
		}
	}

	return false
}

// Stop stops the security manager and all its components
func (sm *SecurityManager) Stop() error {
	if !sm.initialized {
		return nil
	}

	// Log security system shutdown
	sm.auditLogger.LogEvent(AuditEvent{
		Type:      AuditSystemStop,
		Timestamp: time.Now(),
		Success:   true,
		Severity:  SeverityInfo,
		Source:    "security_manager",
	})

	// Stop audit logger
	if err := sm.auditLogger.Stop(); err != nil {
		return fmt.Errorf("failed to stop audit logger: %w", err)
	}

	sm.initialized = false
	return nil
}

// DefaultSecurityConfig returns a default security configuration
func DefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		Authentication: AuthConfig{
			Enabled:        false, // Disabled by default for compatibility
			Level:          AuthBasic,
			SessionTimeout: 24 * time.Hour,
			TokenExpiry:    1 * time.Hour,
			MaxSessions:    5,
			PasswordPolicy: PasswordPolicy{
				MinLength:      8,
				RequireUpper:   false,
				RequireLower:   false,
				RequireNumbers: false,
				RequireSymbols: false,
				MaxAge:         90,
			},
			MFARequired: false,
			LockoutPolicy: LockoutPolicy{
				MaxFailedAttempts: 5,
				LockoutDuration:   15 * time.Minute,
				ResetAfter:        1 * time.Hour,
			},
			SecureStorage: false,
		},
		Permissions: PermissionConfig{
			Enabled:           false, // Disabled by default for compatibility
			DefaultRole:       "user",
			AdminRole:         "admin",
			GuestRole:         "guest",
			CacheTimeout:      5 * time.Minute,
			StrictMode:        false,
			AuditAllChecks:    false,
			InheritanceDepth:  5,
			PermissionTimeout: 30 * time.Second,
		},
		Audit: AuditConfig{
			Enabled:           true, // Enabled by default for security
			LogFile:           "audit.log",
			MaxFileSize:       100 * 1024 * 1024, // 100MB
			MaxFiles:          10,
			BufferSize:        1000,
			FlushInterval:     5 * time.Second,
			MinSeverity:       SeverityInfo,
			IncludeStackTrace: false,
			AsyncLogging:      true,
			Retention: AuditRetention{
				Enabled:     true,
				MaxAge:      90 * 24 * time.Hour, // 90 days
				MaxSize:     1024 * 1024 * 1024,  // 1GB
				CompressOld: true,
				ArchiveOld:  false,
			},
			Alerting: AuditAlerting{
				Enabled: false,
				Rules:   []AuditAlertRule{},
				RateLimit: AuditAlertRateLimit{
					Enabled:    true,
					MaxAlerts:  10,
					TimeWindow: time.Hour,
				},
			},
			Encryption: EncryptionConfig{
				Enabled:   false,
				Algorithm: "AES-256",
			},
		},
		General: GeneralConfig{
			Enabled:               false, // Disabled by default for compatibility
			EnforceHTTPS:          false,
			SecurityHeaders:       false,
			EncryptionAtRest:      false,
			RequireEncryption:     false,
			DeveloperMode:         true, // Enabled by default for development
			SessionSecurity: SessionConfig{
				SecureCookies:   false,
				HTTPOnlyCookies: true,
				SameSiteCookies: "Strict",
				SessionTimeout:  24 * time.Hour,
				IdleTimeout:     2 * time.Hour,
				RegenerateID:    true,
				BindToIP:        false,
				MultipleLogins:  true,
			},
			RateLimiting: RateConfig{
				Enabled:       false,
				LoginAttempts: 5,
				TimeWindow:    15 * time.Minute,
				BlockDuration: 15 * time.Minute,
				GlobalLimit:   1000,
				PerUserLimit:  100,
				PerIPLimit:    50,
			},
		},
	}
}

// ProductionSecurityConfig returns a production-ready security configuration
func ProductionSecurityConfig() *SecurityConfig {
	config := DefaultSecurityConfig()
	
	// Enable all security features for production
	config.General.Enabled = true
	config.Authentication.Enabled = true
	config.Permissions.Enabled = true
	config.General.DeveloperMode = false
	
	// Strengthen authentication
	config.Authentication.Level = AuthMFA
	config.Authentication.MFARequired = true
	config.Authentication.SecureStorage = true
	config.Authentication.PasswordPolicy = PasswordPolicy{
		MinLength:      12,
		RequireUpper:   true,
		RequireLower:   true,
		RequireNumbers: true,
		RequireSymbols: true,
		MaxAge:         60,
	}
	
	// Enable strict permissions
	config.Permissions.StrictMode = true
	config.Permissions.AuditAllChecks = true
	
	// Enable security features
	config.General.EnforceHTTPS = true
	config.General.SecurityHeaders = true
	config.General.EncryptionAtRest = true
	config.General.RequireEncryption = true
	config.General.SessionSecurity.SecureCookies = true
	config.General.SessionSecurity.BindToIP = true
	config.General.SessionSecurity.MultipleLogins = false
	config.General.RateLimiting.Enabled = true
	
	// Enable audit encryption
	config.Audit.Encryption.Enabled = true
	config.Audit.MinSeverity = SeverityWarning
	
	return config
}

// SecurityMiddleware can be used to integrate security into HTTP handlers
type SecurityMiddleware struct {
	securityManager *SecurityManager
}

// NewSecurityMiddleware creates a new security middleware
func NewSecurityMiddleware(securityManager *SecurityManager) *SecurityMiddleware {
	return &SecurityMiddleware{
		securityManager: securityManager,
	}
}