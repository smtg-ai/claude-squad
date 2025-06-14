package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// AuthenticationLevel represents different levels of authentication
type AuthenticationLevel int

const (
	// AuthNone represents no authentication required
	AuthNone AuthenticationLevel = iota
	// AuthBasic represents basic password authentication
	AuthBasic
	// AuthToken represents token-based authentication
	AuthToken
	// AuthMFA represents multi-factor authentication
	AuthMFA
)

// User represents a system user with authentication data
type User struct {
	ID           string                 `json:"id"`
	Username     string                 `json:"username"`
	PasswordHash string                 `json:"password_hash"`
	Salt         string                 `json:"salt"`
	Roles        []string               `json:"roles"`
	Sessions     map[string]*Session    `json:"sessions"`
	CreatedAt    time.Time              `json:"created_at"`
	LastLogin    time.Time              `json:"last_login"`
	MFAEnabled   bool                   `json:"mfa_enabled"`
	MFASecret    string                 `json:"mfa_secret,omitempty"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// Session represents an authenticated user session
type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	Active    bool      `json:"active"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Enabled          bool                `json:"enabled"`
	Level            AuthenticationLevel `json:"level"`
	SessionTimeout   time.Duration       `json:"session_timeout"`
	TokenExpiry      time.Duration       `json:"token_expiry"`
	MaxSessions      int                 `json:"max_sessions"`
	PasswordPolicy   PasswordPolicy      `json:"password_policy"`
	MFARequired      bool                `json:"mfa_required"`
	LockoutPolicy    LockoutPolicy       `json:"lockout_policy"`
	SecureStorage    bool                `json:"secure_storage"`
	EncryptionKeyPath string             `json:"encryption_key_path"`
}

// PasswordPolicy defines password requirements
type PasswordPolicy struct {
	MinLength      int  `json:"min_length"`
	RequireUpper   bool `json:"require_upper"`
	RequireLower   bool `json:"require_lower"`
	RequireNumbers bool `json:"require_numbers"`
	RequireSymbols bool `json:"require_symbols"`
	MaxAge         int  `json:"max_age_days"`
}

// LockoutPolicy defines account lockout rules
type LockoutPolicy struct {
	MaxFailedAttempts int           `json:"max_failed_attempts"`
	LockoutDuration   time.Duration `json:"lockout_duration"`
	ResetAfter        time.Duration `json:"reset_after"`
}

// AuthenticationManager handles user authentication and session management
type AuthenticationManager struct {
	config       *AuthConfig
	users        map[string]*User
	sessions     map[string]*Session
	auditLogger  *AuditLogger
	storageDir   string
	encryptionKey []byte
}

// NewAuthenticationManager creates a new authentication manager
func NewAuthenticationManager(config *AuthConfig, auditLogger *AuditLogger, storageDir string) (*AuthenticationManager, error) {
	manager := &AuthenticationManager{
		config:      config,
		users:       make(map[string]*User),
		sessions:    make(map[string]*Session),
		auditLogger: auditLogger,
		storageDir:  storageDir,
	}

	// Load encryption key if secure storage is enabled
	if config.SecureStorage {
		key, err := manager.loadOrCreateEncryptionKey()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize encryption: %w", err)
		}
		manager.encryptionKey = key
	}

	// Load existing users and sessions
	if err := manager.loadStorage(); err != nil {
		return nil, fmt.Errorf("failed to load authentication storage: %w", err)
	}

	// Start session cleanup routine
	go manager.sessionCleanupRoutine()

	return manager, nil
}

// CreateUser creates a new user with the specified credentials
func (am *AuthenticationManager) CreateUser(username, password string, roles []string) (*User, error) {
	if !am.config.Enabled {
		return nil, fmt.Errorf("authentication is disabled")
	}

	// Validate password policy
	if err := am.validatePassword(password); err != nil {
		return nil, fmt.Errorf("password validation failed: %w", err)
	}

	// Check if user already exists
	if _, exists := am.users[username]; exists {
		return nil, fmt.Errorf("user already exists: %s", username)
	}

	// Generate salt and hash password
	salt, err := generateSalt()
	if err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	passwordHash := hashPassword(password, salt)

	user := &User{
		ID:           generateID(),
		Username:     username,
		PasswordHash: passwordHash,
		Salt:         salt,
		Roles:        roles,
		Sessions:     make(map[string]*Session),
		CreatedAt:    time.Now(),
		MFAEnabled:   am.config.MFARequired,
		Metadata:     make(map[string]interface{}),
	}

	// Generate MFA secret if required
	if am.config.MFARequired {
		user.MFASecret = generateMFASecret()
	}

	am.users[username] = user

	// Save to storage
	if err := am.saveStorage(); err != nil {
		return nil, fmt.Errorf("failed to save user: %w", err)
	}

	// Log user creation
	am.auditLogger.LogEvent(AuditEvent{
		Type:      "user_created",
		UserID:    user.ID,
		Username:  username,
		Timestamp: time.Now(),
		Details:   map[string]interface{}{"roles": roles},
	})

	return user, nil
}

// Authenticate authenticates a user with username and password
func (am *AuthenticationManager) Authenticate(username, password string, metadata map[string]string) (*Session, error) {
	if !am.config.Enabled {
		return nil, fmt.Errorf("authentication is disabled")
	}

	// Get user
	user, exists := am.users[username]
	if !exists {
		am.auditLogger.LogEvent(AuditEvent{
			Type:      "login_failed",
			Username:  username,
			Timestamp: time.Now(),
			Details:   map[string]interface{}{"reason": "user_not_found"},
		})
		return nil, fmt.Errorf("invalid credentials")
	}

	// Verify password
	if !verifyPassword(password, user.Salt, user.PasswordHash) {
		am.auditLogger.LogEvent(AuditEvent{
			Type:      "login_failed",
			UserID:    user.ID,
			Username:  username,
			Timestamp: time.Now(),
			Details:   map[string]interface{}{"reason": "invalid_password"},
		})
		return nil, fmt.Errorf("invalid credentials")
	}

	// Check session limit
	if len(user.Sessions) >= am.config.MaxSessions {
		return nil, fmt.Errorf("maximum sessions exceeded")
	}

	// Create new session
	session := &Session{
		ID:        generateID(),
		UserID:    user.ID,
		Token:     generateSessionToken(),
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(am.config.SessionTimeout),
		IPAddress: metadata["ip_address"],
		UserAgent: metadata["user_agent"],
		Active:    true,
	}

	// Store session
	user.Sessions[session.ID] = session
	am.sessions[session.Token] = session
	user.LastLogin = time.Now()

	// Save to storage
	if err := am.saveStorage(); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	// Log successful login
	am.auditLogger.LogEvent(AuditEvent{
		Type:      "login_success",
		UserID:    user.ID,
		Username:  username,
		SessionID: session.ID,
		Timestamp: time.Now(),
		Details:   map[string]interface{}{"ip_address": session.IPAddress},
	})

	return session, nil
}

// ValidateSession validates a session token and returns the associated user
func (am *AuthenticationManager) ValidateSession(token string) (*User, *Session, error) {
	if !am.config.Enabled {
		return nil, nil, fmt.Errorf("authentication is disabled")
	}

	session, exists := am.sessions[token]
	if !exists {
		return nil, nil, fmt.Errorf("invalid session token")
	}

	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		am.RevokeSession(token)
		return nil, nil, fmt.Errorf("session expired")
	}

	// Check if session is active
	if !session.Active {
		return nil, nil, fmt.Errorf("session is inactive")
	}

	// Get user
	user, exists := am.getUserByID(session.UserID)
	if !exists {
		am.RevokeSession(token)
		return nil, nil, fmt.Errorf("user not found")
	}

	return user, session, nil
}

// RevokeSession revokes a session token
func (am *AuthenticationManager) RevokeSession(token string) error {
	session, exists := am.sessions[token]
	if !exists {
		return fmt.Errorf("session not found")
	}

	// Mark session as inactive
	session.Active = false

	// Remove from sessions map
	delete(am.sessions, token)

	// Remove from user's sessions
	if user, exists := am.getUserByID(session.UserID); exists {
		delete(user.Sessions, session.ID)
	}

	// Save to storage
	if err := am.saveStorage(); err != nil {
		return fmt.Errorf("failed to save session revocation: %w", err)
	}

	// Log session revocation
	am.auditLogger.LogEvent(AuditEvent{
		Type:      "session_revoked",
		UserID:    session.UserID,
		SessionID: session.ID,
		Timestamp: time.Now(),
	})

	return nil
}

// ChangePassword changes a user's password
func (am *AuthenticationManager) ChangePassword(username, oldPassword, newPassword string) error {
	user, exists := am.users[username]
	if !exists {
		return fmt.Errorf("user not found")
	}

	// Verify old password
	if !verifyPassword(oldPassword, user.Salt, user.PasswordHash) {
		return fmt.Errorf("invalid current password")
	}

	// Validate new password
	if err := am.validatePassword(newPassword); err != nil {
		return fmt.Errorf("password validation failed: %w", err)
	}

	// Generate new salt and hash
	salt, err := generateSalt()
	if err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	user.Salt = salt
	user.PasswordHash = hashPassword(newPassword, salt)

	// Save to storage
	if err := am.saveStorage(); err != nil {
		return fmt.Errorf("failed to save password change: %w", err)
	}

	// Log password change
	am.auditLogger.LogEvent(AuditEvent{
		Type:      "password_changed",
		UserID:    user.ID,
		Username:  username,
		Timestamp: time.Now(),
	})

	return nil
}

// GetUserSessions returns all active sessions for a user
func (am *AuthenticationManager) GetUserSessions(username string) ([]*Session, error) {
	user, exists := am.users[username]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}

	sessions := make([]*Session, 0, len(user.Sessions))
	for _, session := range user.Sessions {
		if session.Active && time.Now().Before(session.ExpiresAt) {
			sessions = append(sessions, session)
		}
	}

	return sessions, nil
}

// Helper functions

func (am *AuthenticationManager) getUserByID(userID string) (*User, bool) {
	for _, user := range am.users {
		if user.ID == userID {
			return user, true
		}
	}
	return nil, false
}

func (am *AuthenticationManager) validatePassword(password string) error {
	policy := am.config.PasswordPolicy

	if len(password) < policy.MinLength {
		return fmt.Errorf("password must be at least %d characters long", policy.MinLength)
	}

	if policy.RequireUpper && !containsUpper(password) {
		return fmt.Errorf("password must contain uppercase letters")
	}

	if policy.RequireLower && !containsLower(password) {
		return fmt.Errorf("password must contain lowercase letters")
	}

	if policy.RequireNumbers && !containsDigit(password) {
		return fmt.Errorf("password must contain numbers")
	}

	if policy.RequireSymbols && !containsSymbol(password) {
		return fmt.Errorf("password must contain symbols")
	}

	return nil
}

func (am *AuthenticationManager) sessionCleanupRoutine() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		am.cleanupExpiredSessions()
	}
}

func (am *AuthenticationManager) cleanupExpiredSessions() {
	now := time.Now()
	expiredTokens := make([]string, 0)

	for token, session := range am.sessions {
		if now.After(session.ExpiresAt) || !session.Active {
			expiredTokens = append(expiredTokens, token)
		}
	}

	for _, token := range expiredTokens {
		am.RevokeSession(token)
	}
}

func (am *AuthenticationManager) loadOrCreateEncryptionKey() ([]byte, error) {
	keyPath := am.config.EncryptionKeyPath
	if keyPath == "" {
		keyPath = filepath.Join(am.storageDir, "encryption.key")
	}

	// Try to load existing key
	if data, err := os.ReadFile(keyPath); err == nil {
		return data, nil
	}

	// Generate new key
	key := make([]byte, 32) // 256-bit key
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate encryption key: %w", err)
	}

	// Save key
	if err := os.WriteFile(keyPath, key, 0600); err != nil {
		return nil, fmt.Errorf("failed to save encryption key: %w", err)
	}

	return key, nil
}

func (am *AuthenticationManager) loadStorage() error {
	usersPath := filepath.Join(am.storageDir, "users.json")
	sessionsPath := filepath.Join(am.storageDir, "sessions.json")

	// Load users
	if data, err := os.ReadFile(usersPath); err == nil {
		if err := json.Unmarshal(data, &am.users); err != nil {
			return fmt.Errorf("failed to parse users data: %w", err)
		}
	}

	// Load sessions
	if data, err := os.ReadFile(sessionsPath); err == nil {
		if err := json.Unmarshal(data, &am.sessions); err != nil {
			return fmt.Errorf("failed to parse sessions data: %w", err)
		}
	}

	return nil
}

func (am *AuthenticationManager) saveStorage() error {
	if err := os.MkdirAll(am.storageDir, 0700); err != nil {
		return fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Save users
	usersData, err := json.MarshalIndent(am.users, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal users: %w", err)
	}

	usersPath := filepath.Join(am.storageDir, "users.json")
	if err := os.WriteFile(usersPath, usersData, 0600); err != nil {
		return fmt.Errorf("failed to save users: %w", err)
	}

	// Save sessions
	sessionsData, err := json.MarshalIndent(am.sessions, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal sessions: %w", err)
	}

	sessionsPath := filepath.Join(am.storageDir, "sessions.json")
	if err := os.WriteFile(sessionsPath, sessionsData, 0600); err != nil {
		return fmt.Errorf("failed to save sessions: %w", err)
	}

	return nil
}

// Utility functions

func generateSalt() (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(salt), nil
}

func generateID() string {
	id := make([]byte, 16)
	rand.Read(id)
	return base64.StdEncoding.EncodeToString(id)
}

func generateSessionToken() string {
	token := make([]byte, 32)
	rand.Read(token)
	return base64.StdEncoding.EncodeToString(token)
}

func generateMFASecret() string {
	secret := make([]byte, 20)
	rand.Read(secret)
	return base64.StdEncoding.EncodeToString(secret)
}

func hashPassword(password, salt string) string {
	hasher := sha256.New()
	hasher.Write([]byte(password + salt))
	return base64.StdEncoding.EncodeToString(hasher.Sum(nil))
}

func verifyPassword(password, salt, hash string) bool {
	return hashPassword(password, salt) == hash
}

func containsUpper(s string) bool {
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			return true
		}
	}
	return false
}

func containsLower(s string) bool {
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			return true
		}
	}
	return false
}

func containsDigit(s string) bool {
	for _, r := range s {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

func containsSymbol(s string) bool {
	symbols := "!@#$%^&*()_+-=[]{}|;:,.<>?"
	for _, r := range s {
		for _, symbol := range symbols {
			if r == symbol {
				return true
			}
		}
	}
	return false
}