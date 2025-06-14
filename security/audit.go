package security

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// AuditEventType represents different types of audit events
type AuditEventType string

const (
	// Authentication events
	AuditLogin          AuditEventType = "login"
	AuditLoginFailed    AuditEventType = "login_failed"
	AuditLogout         AuditEventType = "logout"
	AuditSessionExpired AuditEventType = "session_expired"

	// Permission events
	AuditPermissionGranted AuditEventType = "permission_granted"
	AuditPermissionDenied  AuditEventType = "permission_denied"

	// System events
	AuditSystemStart    AuditEventType = "system_start"
	AuditSystemStop     AuditEventType = "system_stop"
	AuditConfigChanged  AuditEventType = "config_changed"
	AuditUserCreated    AuditEventType = "user_created"
	AuditUserDeleted    AuditEventType = "user_deleted"
	AuditRoleAssigned   AuditEventType = "role_assigned"
	AuditRoleRevoked    AuditEventType = "role_revoked"
	AuditPasswordChange AuditEventType = "password_changed"

	// Session events
	AuditSessionCreated   AuditEventType = "session_created"
	AuditSessionAttached  AuditEventType = "session_attached"
	AuditSessionDetached  AuditEventType = "session_detached"
	AuditSessionKilled    AuditEventType = "session_killed"
	AuditPromptSent      AuditEventType = "prompt_sent"

	// Git events
	AuditGitCommit    AuditEventType = "git_commit"
	AuditGitPush      AuditEventType = "git_push"
	AuditGitPull      AuditEventType = "git_pull"
	AuditGitBranch    AuditEventType = "git_branch"
	AuditGitCheckout  AuditEventType = "git_checkout"

	// Security events
	AuditSecurityViolation AuditEventType = "security_violation"
	AuditSuspiciousActivity AuditEventType = "suspicious_activity"
	AuditBruteForceAttempt  AuditEventType = "brute_force_attempt"
)

// AuditEvent represents a single audit log entry
type AuditEvent struct {
	ID          string                 `json:"id"`
	Type        AuditEventType         `json:"type"`
	Timestamp   time.Time              `json:"timestamp"`
	UserID      string                 `json:"user_id,omitempty"`
	Username    string                 `json:"username,omitempty"`
	SessionID   string                 `json:"session_id,omitempty"`
	IPAddress   string                 `json:"ip_address,omitempty"`
	UserAgent   string                 `json:"user_agent,omitempty"`
	Resource    string                 `json:"resource,omitempty"`
	Action      string                 `json:"action,omitempty"`
	Success     bool                   `json:"success"`
	ErrorMsg    string                 `json:"error_msg,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Severity    AuditSeverity          `json:"severity"`
	Source      string                 `json:"source"`
	Environment map[string]interface{} `json:"environment,omitempty"`
}

// AuditSeverity represents the severity level of an audit event
type AuditSeverity string

const (
	SeverityInfo     AuditSeverity = "info"
	SeverityWarning  AuditSeverity = "warning"
	SeverityError    AuditSeverity = "error"
	SeverityCritical AuditSeverity = "critical"
)

// AuditFilter represents filters for querying audit logs
type AuditFilter struct {
	UserID     string         `json:"user_id,omitempty"`
	Username   string         `json:"username,omitempty"`
	Type       AuditEventType `json:"type,omitempty"`
	Severity   AuditSeverity  `json:"severity,omitempty"`
	StartTime  time.Time      `json:"start_time,omitempty"`
	EndTime    time.Time      `json:"end_time,omitempty"`
	Success    *bool          `json:"success,omitempty"`
	IPAddress  string         `json:"ip_address,omitempty"`
	Resource   string         `json:"resource,omitempty"`
	Action     string         `json:"action,omitempty"`
	Limit      int            `json:"limit,omitempty"`
	Offset     int            `json:"offset,omitempty"`
}

// AuditConfig represents audit logging configuration
type AuditConfig struct {
	Enabled           bool              `json:"enabled"`
	LogFile           string            `json:"log_file"`
	MaxFileSize       int64             `json:"max_file_size"`
	MaxFiles          int               `json:"max_files"`
	BufferSize        int               `json:"buffer_size"`
	FlushInterval     time.Duration     `json:"flush_interval"`
	MinSeverity       AuditSeverity     `json:"min_severity"`
	IncludeStackTrace bool              `json:"include_stack_trace"`
	AsyncLogging      bool              `json:"async_logging"`
	Retention         AuditRetention    `json:"retention"`
	Alerting          AuditAlerting     `json:"alerting"`
	Encryption        EncryptionConfig  `json:"encryption"`
}

// AuditRetention represents audit log retention policy
type AuditRetention struct {
	Enabled       bool          `json:"enabled"`
	MaxAge        time.Duration `json:"max_age"`
	MaxSize       int64         `json:"max_size"`
	CompressOld   bool          `json:"compress_old"`
	ArchiveOld    bool          `json:"archive_old"`
	ArchivePath   string        `json:"archive_path"`
}

// AuditAlerting represents audit alerting configuration
type AuditAlerting struct {
	Enabled       bool                     `json:"enabled"`
	Rules         []AuditAlertRule         `json:"rules"`
	Destinations  []AuditAlertDestination  `json:"destinations"`
	RateLimit     AuditAlertRateLimit      `json:"rate_limit"`
}

// AuditAlertRule represents a rule for triggering alerts
type AuditAlertRule struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Conditions  map[string]interface{} `json:"conditions"`
	Threshold   int                    `json:"threshold"`
	TimeWindow  time.Duration          `json:"time_window"`
	Severity    AuditSeverity          `json:"severity"`
	Enabled     bool                   `json:"enabled"`
}

// AuditAlertDestination represents where to send alerts
type AuditAlertDestination struct {
	Type   string                 `json:"type"` // email, webhook, syslog
	Config map[string]interface{} `json:"config"`
}

// AuditAlertRateLimit represents rate limiting for alerts
type AuditAlertRateLimit struct {
	Enabled     bool          `json:"enabled"`
	MaxAlerts   int           `json:"max_alerts"`
	TimeWindow  time.Duration `json:"time_window"`
}

// EncryptionConfig represents encryption configuration for audit logs
type EncryptionConfig struct {
	Enabled   bool   `json:"enabled"`
	Algorithm string `json:"algorithm"`
	KeyPath   string `json:"key_path"`
}

// AuditLogger handles audit logging functionality
type AuditLogger struct {
	config      *AuditConfig
	logFile     *os.File
	buffer      chan *AuditEvent
	storageDir  string
	encryptionKey []byte
	mutex       sync.RWMutex
	stats       *AuditStats
	alertRules  map[string]*AuditAlertRule
	stopChan    chan struct{}
	running     bool
}

// AuditStats represents audit logging statistics
type AuditStats struct {
	TotalEvents    int64                    `json:"total_events"`
	EventsByType   map[AuditEventType]int64 `json:"events_by_type"`
	EventsBySeverity map[AuditSeverity]int64 `json:"events_by_severity"`
	EventsPerHour  map[string]int64         `json:"events_per_hour"`
	LastEvent      time.Time                `json:"last_event"`
	AlertsTriggered int64                   `json:"alerts_triggered"`
	ErrorCount     int64                    `json:"error_count"`
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(config *AuditConfig, storageDir string) (*AuditLogger, error) {
	logger := &AuditLogger{
		config:     config,
		storageDir: storageDir,
		buffer:     make(chan *AuditEvent, config.BufferSize),
		stats:      &AuditStats{
			EventsByType:     make(map[AuditEventType]int64),
			EventsBySeverity: make(map[AuditSeverity]int64),
			EventsPerHour:    make(map[string]int64),
		},
		alertRules: make(map[string]*AuditAlertRule),
		stopChan:   make(chan struct{}),
	}

	if !config.Enabled {
		return logger, nil
	}

	// Create storage directory
	if err := os.MkdirAll(storageDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create audit storage directory: %w", err)
	}

	// Open log file
	logPath := filepath.Join(storageDir, config.LogFile)
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log file: %w", err)
	}
	logger.logFile = file

	// Load encryption key if enabled
	if config.Encryption.Enabled {
		key, err := logger.loadEncryptionKey()
		if err != nil {
			return nil, fmt.Errorf("failed to load encryption key: %w", err)
		}
		logger.encryptionKey = key
	}

	// Load alert rules
	for _, rule := range config.Alerting.Rules {
		logger.alertRules[rule.ID] = &rule
	}

	// Start background processing
	if config.AsyncLogging {
		go logger.processingLoop()
	}

	logger.running = true

	// Log system start
	logger.LogEvent(AuditEvent{
		Type:      AuditSystemStart,
		Timestamp: time.Now(),
		Success:   true,
		Severity:  SeverityInfo,
		Source:    "audit_logger",
		Details: map[string]interface{}{
			"config": config,
		},
	})

	return logger, nil
}

// LogEvent logs an audit event
func (al *AuditLogger) LogEvent(event AuditEvent) {
	if !al.config.Enabled {
		return
	}

	// Set defaults
	if event.ID == "" {
		event.ID = generateID()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if event.Severity == "" {
		event.Severity = SeverityInfo
	}
	if event.Source == "" {
		event.Source = "claude-squad"
	}

	// Check minimum severity
	if al.severityLevel(event.Severity) < al.severityLevel(al.config.MinSeverity) {
		return
	}

	// Update statistics
	al.updateStats(&event)

	// Check alert rules
	al.checkAlertRules(&event)

	if al.config.AsyncLogging {
		// Send to buffer for async processing
		select {
		case al.buffer <- &event:
		default:
			// Buffer full, log synchronously
			al.writeEvent(&event)
		}
	} else {
		// Log synchronously
		al.writeEvent(&event)
	}
}

// writeEvent writes an event to the log file
func (al *AuditLogger) writeEvent(event *AuditEvent) {
	al.mutex.Lock()
	defer al.mutex.Unlock()

	data, err := json.Marshal(event)
	if err != nil {
		al.stats.ErrorCount++
		return
	}

	// Add newline
	data = append(data, '\n')

	// Encrypt if enabled
	if al.config.Encryption.Enabled && al.encryptionKey != nil {
		data = al.encryptData(data)
	}

	// Write to file
	if al.logFile != nil {
		if _, err := al.logFile.Write(data); err != nil {
			al.stats.ErrorCount++
			return
		}

		// Flush immediately for critical events
		if event.Severity == SeverityCritical {
			al.logFile.Sync()
		}
	}

	// Check file rotation
	if al.needsRotation() {
		al.rotateLogFile()
	}
}

// processingLoop handles async event processing
func (al *AuditLogger) processingLoop() {
	ticker := time.NewTicker(al.config.FlushInterval)
	defer ticker.Stop()

	events := make([]*AuditEvent, 0, al.config.BufferSize)

	for {
		select {
		case event := <-al.buffer:
			events = append(events, event)
			
			// Flush if buffer is full or event is critical
			if len(events) >= al.config.BufferSize || event.Severity == SeverityCritical {
				al.flushEvents(events)
				events = events[:0]
			}

		case <-ticker.C:
			// Periodic flush
			if len(events) > 0 {
				al.flushEvents(events)
				events = events[:0]
			}

		case <-al.stopChan:
			// Flush remaining events before stopping
			if len(events) > 0 {
				al.flushEvents(events)
			}
			return
		}
	}
}

// flushEvents writes multiple events to the log file
func (al *AuditLogger) flushEvents(events []*AuditEvent) {
	for _, event := range events {
		al.writeEvent(event)
	}

	// Force sync to disk
	if al.logFile != nil {
		al.logFile.Sync()
	}
}

// QueryEvents queries audit events based on filters
func (al *AuditLogger) QueryEvents(filter AuditFilter) ([]*AuditEvent, error) {
	if !al.config.Enabled {
		return nil, fmt.Errorf("audit logging is disabled")
	}

	// Read and parse log file
	logPath := filepath.Join(al.storageDir, al.config.LogFile)
	data, err := os.ReadFile(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read audit log: %w", err)
	}

	// Decrypt if enabled
	if al.config.Encryption.Enabled && al.encryptionKey != nil {
		data = al.decryptData(data)
	}

	var events []*AuditEvent
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var event AuditEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue // Skip malformed events
		}

		if al.matchesFilter(&event, filter) {
			events = append(events, &event)
		}
	}

	// Apply limit and offset
	if filter.Offset > 0 && filter.Offset < len(events) {
		events = events[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(events) {
		events = events[:filter.Limit]
	}

	return events, nil
}

// matchesFilter checks if an event matches the given filter
func (al *AuditLogger) matchesFilter(event *AuditEvent, filter AuditFilter) bool {
	if filter.UserID != "" && event.UserID != filter.UserID {
		return false
	}
	if filter.Username != "" && event.Username != filter.Username {
		return false
	}
	if filter.Type != "" && event.Type != filter.Type {
		return false
	}
	if filter.Severity != "" && event.Severity != filter.Severity {
		return false
	}
	if !filter.StartTime.IsZero() && event.Timestamp.Before(filter.StartTime) {
		return false
	}
	if !filter.EndTime.IsZero() && event.Timestamp.After(filter.EndTime) {
		return false
	}
	if filter.Success != nil && event.Success != *filter.Success {
		return false
	}
	if filter.IPAddress != "" && event.IPAddress != filter.IPAddress {
		return false
	}
	if filter.Resource != "" && event.Resource != filter.Resource {
		return false
	}
	if filter.Action != "" && event.Action != filter.Action {
		return false
	}
	return true
}

// GetStats returns audit logging statistics
func (al *AuditLogger) GetStats() *AuditStats {
	al.mutex.RLock()
	defer al.mutex.RUnlock()
	
	// Create a copy to avoid race conditions
	stats := &AuditStats{
		TotalEvents:      al.stats.TotalEvents,
		EventsByType:     make(map[AuditEventType]int64),
		EventsBySeverity: make(map[AuditSeverity]int64),
		EventsPerHour:    make(map[string]int64),
		LastEvent:        al.stats.LastEvent,
		AlertsTriggered:  al.stats.AlertsTriggered,
		ErrorCount:       al.stats.ErrorCount,
	}
	
	for k, v := range al.stats.EventsByType {
		stats.EventsByType[k] = v
	}
	for k, v := range al.stats.EventsBySeverity {
		stats.EventsBySeverity[k] = v
	}
	for k, v := range al.stats.EventsPerHour {
		stats.EventsPerHour[k] = v
	}
	
	return stats
}

// Stop stops the audit logger
func (al *AuditLogger) Stop() error {
	if !al.running {
		return nil
	}

	// Log system stop
	al.LogEvent(AuditEvent{
		Type:      AuditSystemStop,
		Timestamp: time.Now(),
		Success:   true,
		Severity:  SeverityInfo,
		Source:    "audit_logger",
	})

	// Stop processing loop
	if al.config.AsyncLogging {
		close(al.stopChan)
	}

	// Close log file
	if al.logFile != nil {
		al.logFile.Close()
	}

	al.running = false
	return nil
}

// Helper functions

func (al *AuditLogger) updateStats(event *AuditEvent) {
	al.mutex.Lock()
	defer al.mutex.Unlock()

	al.stats.TotalEvents++
	al.stats.EventsByType[event.Type]++
	al.stats.EventsBySeverity[event.Severity]++
	al.stats.LastEvent = event.Timestamp

	// Update hourly stats
	hourKey := event.Timestamp.Format("2006-01-02-15")
	al.stats.EventsPerHour[hourKey]++
}

func (al *AuditLogger) checkAlertRules(event *AuditEvent) {
	if !al.config.Alerting.Enabled {
		return
	}

	for _, rule := range al.alertRules {
		if !rule.Enabled {
			continue
		}

		if al.eventMatchesRule(event, rule) {
			al.triggerAlert(rule, event)
		}
	}
}

func (al *AuditLogger) eventMatchesRule(event *AuditEvent, rule *AuditAlertRule) bool {
	// Simple condition matching for now
	for key, expectedValue := range rule.Conditions {
		var actualValue interface{}
		
		switch key {
		case "type":
			actualValue = event.Type
		case "severity":
			actualValue = event.Severity
		case "success":
			actualValue = event.Success
		case "user_id":
			actualValue = event.UserID
		case "resource":
			actualValue = event.Resource
		case "action":
			actualValue = event.Action
		default:
			if event.Details != nil {
				actualValue = event.Details[key]
			}
		}

		if actualValue != expectedValue {
			return false
		}
	}
	return true
}

func (al *AuditLogger) triggerAlert(rule *AuditAlertRule, event *AuditEvent) {
	al.stats.AlertsTriggered++
	
	// Log the alert
	al.LogEvent(AuditEvent{
		Type:      AuditSuspiciousActivity,
		Timestamp: time.Now(),
		Success:   true,
		Severity:  rule.Severity,
		Source:    "audit_alerting",
		Details: map[string]interface{}{
			"rule_id":     rule.ID,
			"rule_name":   rule.Name,
			"trigger_event": event,
		},
	})
}

func (al *AuditLogger) severityLevel(severity AuditSeverity) int {
	switch severity {
	case SeverityInfo:
		return 1
	case SeverityWarning:
		return 2
	case SeverityError:
		return 3
	case SeverityCritical:
		return 4
	default:
		return 0
	}
}

func (al *AuditLogger) needsRotation() bool {
	if al.logFile == nil {
		return false
	}

	stat, err := al.logFile.Stat()
	if err != nil {
		return false
	}

	return stat.Size() > al.config.MaxFileSize
}

func (al *AuditLogger) rotateLogFile() {
	// Close current file
	if al.logFile != nil {
		al.logFile.Close()
	}

	// Rotate existing files
	logPath := filepath.Join(al.storageDir, al.config.LogFile)
	for i := al.config.MaxFiles - 1; i > 0; i-- {
		oldPath := fmt.Sprintf("%s.%d", logPath, i)
		newPath := fmt.Sprintf("%s.%d", logPath, i+1)
		os.Rename(oldPath, newPath)
	}

	// Move current log to .1
	os.Rename(logPath, logPath+".1")

	// Create new log file
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		al.stats.ErrorCount++
		return
	}
	al.logFile = file
}

func (al *AuditLogger) loadEncryptionKey() ([]byte, error) {
	keyPath := al.config.Encryption.KeyPath
	if keyPath == "" {
		keyPath = filepath.Join(al.storageDir, "audit_encryption.key")
	}

	return os.ReadFile(keyPath)
}

func (al *AuditLogger) encryptData(data []byte) []byte {
	// Simple XOR encryption for demonstration
	// In production, use proper encryption like AES
	encrypted := make([]byte, len(data))
	keyLen := len(al.encryptionKey)
	
	for i, b := range data {
		encrypted[i] = b ^ al.encryptionKey[i%keyLen]
	}
	
	return encrypted
}

func (al *AuditLogger) decryptData(data []byte) []byte {
	// XOR decryption (same as encryption for XOR)
	return al.encryptData(data)
}

// Convenience functions for common audit events

func (al *AuditLogger) LogLogin(userID, username, sessionID, ipAddress string, success bool, errorMsg string) {
	eventType := AuditLogin
	if !success {
		eventType = AuditLoginFailed
	}

	al.LogEvent(AuditEvent{
		Type:      eventType,
		UserID:    userID,
		Username:  username,
		SessionID: sessionID,
		IPAddress: ipAddress,
		Success:   success,
		ErrorMsg:  errorMsg,
		Severity:  SeverityInfo,
		Details: map[string]interface{}{
			"login_attempt": true,
		},
	})
}

func (al *AuditLogger) LogPermissionCheck(userID string, resource, action, scope string, granted bool, reason string) {
	eventType := AuditPermissionGranted
	if !granted {
		eventType = AuditPermissionDenied
	}

	al.LogEvent(AuditEvent{
		Type:     eventType,
		UserID:   userID,
		Resource: resource,
		Action:   action,
		Success:  granted,
		Severity: SeverityInfo,
		Details: map[string]interface{}{
			"scope":  scope,
			"reason": reason,
		},
	})
}

func (al *AuditLogger) LogSecurityViolation(userID, username, description string, severity AuditSeverity, details map[string]interface{}) {
	al.LogEvent(AuditEvent{
		Type:     AuditSecurityViolation,
		UserID:   userID,
		Username: username,
		Success:  false,
		Severity: severity,
		Details:  details,
		ErrorMsg: description,
	})
}