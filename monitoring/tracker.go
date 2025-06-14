package monitoring

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// UsageEventType represents different types of usage events
type UsageEventType string

const (
	// Session events
	EventSessionCreated   UsageEventType = "session_created"
	EventSessionAttached  UsageEventType = "session_attached"
	EventSessionDetached  UsageEventType = "session_detached"
	EventSessionKilled    UsageEventType = "session_killed"
	
	// Command events
	EventCommandExecuted  UsageEventType = "command_executed"
	EventPromptSent      UsageEventType = "prompt_sent"
	EventResponseReceived UsageEventType = "response_received"
	
	// Git events
	EventGitCommit       UsageEventType = "git_commit"
	EventGitPush         UsageEventType = "git_push"
	EventGitPull         UsageEventType = "git_pull"
	EventGitBranch       UsageEventType = "git_branch"
	
	// System events
	EventSystemStart     UsageEventType = "system_start"
	EventSystemStop      UsageEventType = "system_stop"
	EventError          UsageEventType = "error"
	EventPerformance    UsageEventType = "performance"
)

// UsageEvent represents a single usage tracking event
type UsageEvent struct {
	ID          string                 `json:"id"`
	Type        UsageEventType         `json:"type"`
	Timestamp   time.Time              `json:"timestamp"`
	UserID      string                 `json:"user_id,omitempty"`
	SessionID   string                 `json:"session_id,omitempty"`
	SessionName string                 `json:"session_name,omitempty"`
	Command     string                 `json:"command,omitempty"`
	Program     string                 `json:"program,omitempty"`
	Duration    time.Duration          `json:"duration,omitempty"`
	Success     bool                   `json:"success"`
	ErrorMsg    string                 `json:"error_msg,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Source      string                 `json:"source"`
	Repository  string                 `json:"repository,omitempty"`
	Branch      string                 `json:"branch,omitempty"`
}

// UsageStats represents aggregated usage statistics
type UsageStats struct {
	TotalEvents      int64                        `json:"total_events"`
	EventsByType     map[UsageEventType]int64     `json:"events_by_type"`
	SessionStats     SessionStats                 `json:"session_stats"`
	CommandStats     CommandStats                 `json:"command_stats"`
	GitStats         GitStats                     `json:"git_stats"`
	PerformanceStats PerformanceStats             `json:"performance_stats"`
	ErrorStats       ErrorStats                   `json:"error_stats"`
	TimeRange        TimeRange                    `json:"time_range"`
	LastUpdated      time.Time                    `json:"last_updated"`
}

// SessionStats represents session-related statistics
type SessionStats struct {
	TotalSessions       int64         `json:"total_sessions"`
	ActiveSessions      int64         `json:"active_sessions"`
	AverageSessionTime  time.Duration `json:"average_session_time"`
	SessionsByProgram   map[string]int64 `json:"sessions_by_program"`
	MostUsedPrograms    []ProgramUsage   `json:"most_used_programs"`
}

// CommandStats represents command execution statistics
type CommandStats struct {
	TotalCommands       int64              `json:"total_commands"`
	CommandsPerSession  float64            `json:"commands_per_session"`
	AverageCommandTime  time.Duration      `json:"average_command_time"`
	MostUsedCommands    []CommandUsage     `json:"most_used_commands"`
	CommandsByProgram   map[string]int64   `json:"commands_by_program"`
}

// GitStats represents git operation statistics
type GitStats struct {
	TotalCommits       int64            `json:"total_commits"`
	TotalPushes        int64            `json:"total_pushes"`
	TotalPulls         int64            `json:"total_pulls"`
	BranchesCreated    int64            `json:"branches_created"`
	RepositoryActivity map[string]int64 `json:"repository_activity"`
	MostActiveRepos    []RepoActivity   `json:"most_active_repos"`
}

// PerformanceStats represents system performance metrics
type PerformanceStats struct {
	AverageResponseTime time.Duration     `json:"average_response_time"`
	P95ResponseTime     time.Duration     `json:"p95_response_time"`
	P99ResponseTime     time.Duration     `json:"p99_response_time"`
	ThroughputPerHour   float64           `json:"throughput_per_hour"`
	PeakUsageHours      []string          `json:"peak_usage_hours"`
	ResourceUsage       ResourceUsage     `json:"resource_usage"`
}

// ErrorStats represents error tracking statistics
type ErrorStats struct {
	TotalErrors        int64              `json:"total_errors"`
	ErrorRate          float64            `json:"error_rate"`
	ErrorsByType       map[string]int64   `json:"errors_by_type"`
	MostCommonErrors   []ErrorFrequency   `json:"most_common_errors"`
	ErrorTrends        []ErrorTrend       `json:"error_trends"`
}

// Supporting types
type ProgramUsage struct {
	Program string `json:"program"`
	Count   int64  `json:"count"`
	Percentage float64 `json:"percentage"`
}

type CommandUsage struct {
	Command string `json:"command"`
	Count   int64  `json:"count"`
	AverageTime time.Duration `json:"average_time"`
}

type RepoActivity struct {
	Repository string `json:"repository"`
	Events     int64  `json:"events"`
	LastActivity time.Time `json:"last_activity"`
}

type ResourceUsage struct {
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	DiskUsage   float64 `json:"disk_usage"`
}

type ErrorFrequency struct {
	Error string `json:"error"`
	Count int64  `json:"count"`
	LastSeen time.Time `json:"last_seen"`
}

type ErrorTrend struct {
	Hour  string `json:"hour"`
	Count int64  `json:"count"`
}

type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// UsageFilter represents filters for querying usage data
type UsageFilter struct {
	UserID     string         `json:"user_id,omitempty"`
	SessionID  string         `json:"session_id,omitempty"`
	Type       UsageEventType `json:"type,omitempty"`
	Program    string         `json:"program,omitempty"`
	StartTime  time.Time      `json:"start_time,omitempty"`
	EndTime    time.Time      `json:"end_time,omitempty"`
	Success    *bool          `json:"success,omitempty"`
	Repository string         `json:"repository,omitempty"`
	Limit      int            `json:"limit,omitempty"`
	Offset     int            `json:"offset,omitempty"`
}

// TrackerConfig represents configuration for usage tracking
type TrackerConfig struct {
	Enabled           bool          `json:"enabled"`
	StorageDir        string        `json:"storage_dir"`
	LogFile           string        `json:"log_file"`
	BufferSize        int           `json:"buffer_size"`
	FlushInterval     time.Duration `json:"flush_interval"`
	RetentionDays     int           `json:"retention_days"`
	StatsInterval     time.Duration `json:"stats_interval"`
	AsyncLogging      bool          `json:"async_logging"`
	CompressOldLogs   bool          `json:"compress_old_logs"`
	EnablePerformance bool          `json:"enable_performance"`
	EnableDetailed    bool          `json:"enable_detailed"`
}

// UsageTracker handles usage tracking and statistics
type UsageTracker struct {
	config      *TrackerConfig
	logFile     *os.File
	buffer      chan *UsageEvent
	storageDir  string
	mutex       sync.RWMutex
	stats       *UsageStats
	stopChan    chan struct{}
	running     bool
	sessions    map[string]*SessionTracker
}

// SessionTracker tracks individual session metrics
type SessionTracker struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Program      string                 `json:"program"`
	StartTime    time.Time              `json:"start_time"`
	LastActivity time.Time              `json:"last_activity"`
	CommandCount int64                  `json:"command_count"`
	TotalTime    time.Duration          `json:"total_time"`
	Repository   string                 `json:"repository"`
	Branch       string                 `json:"branch"`
	Metadata     map[string]interface{} `json:"metadata"`
	Active       bool                   `json:"active"`
}

// NewUsageTracker creates a new usage tracker
func NewUsageTracker(config *TrackerConfig) (*UsageTracker, error) {
	tracker := &UsageTracker{
		config:     config,
		storageDir: config.StorageDir,
		buffer:     make(chan *UsageEvent, config.BufferSize),
		stats:      &UsageStats{
			EventsByType:  make(map[UsageEventType]int64),
			SessionStats:  SessionStats{SessionsByProgram: make(map[string]int64)},
			CommandStats:  CommandStats{CommandsByProgram: make(map[string]int64)},
			GitStats:      GitStats{RepositoryActivity: make(map[string]int64)},
			ErrorStats:    ErrorStats{ErrorsByType: make(map[string]int64)},
			TimeRange:     TimeRange{Start: time.Now(), End: time.Now()},
		},
		sessions: make(map[string]*SessionTracker),
		stopChan: make(chan struct{}),
	}

	if !config.Enabled {
		return tracker, nil
	}

	// Create storage directory
	if err := os.MkdirAll(config.StorageDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Open log file
	logPath := filepath.Join(config.StorageDir, config.LogFile)
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	tracker.logFile = file

	// Load existing stats
	if err := tracker.loadStats(); err != nil {
		// Log error but continue
		fmt.Printf("Warning: failed to load existing stats: %v\n", err)
	}

	// Start background processing
	if config.AsyncLogging {
		go tracker.processingLoop()
	}

	// Start stats collection
	go tracker.statsLoop()

	tracker.running = true

	// Log system start
	tracker.TrackEvent(UsageEvent{
		Type:      EventSystemStart,
		Timestamp: time.Now(),
		Success:   true,
		Source:    "usage_tracker",
		Metadata: map[string]interface{}{
			"config": config,
		},
	})

	return tracker, nil
}

// TrackEvent tracks a usage event
func (ut *UsageTracker) TrackEvent(event UsageEvent) {
	if !ut.config.Enabled {
		return
	}

	// Set defaults
	if event.ID == "" {
		event.ID = generateEventID()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if event.Source == "" {
		event.Source = "claude-squad"
	}

	// Update session tracker if applicable
	if event.SessionID != "" {
		ut.updateSessionTracker(&event)
	}

	if ut.config.AsyncLogging {
		// Send to buffer for async processing
		select {
		case ut.buffer <- &event:
		default:
			// Buffer full, log synchronously
			ut.writeEvent(&event)
		}
	} else {
		// Log synchronously
		ut.writeEvent(&event)
	}

	// Update real-time stats
	ut.updateStats(&event)
}

// writeEvent writes an event to the log file
func (ut *UsageTracker) writeEvent(event *UsageEvent) {
	ut.mutex.Lock()
	defer ut.mutex.Unlock()

	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	// Add newline
	data = append(data, '\n')

	// Write to file
	if ut.logFile != nil {
		ut.logFile.Write(data)
	}
}

// processingLoop handles async event processing
func (ut *UsageTracker) processingLoop() {
	ticker := time.NewTicker(ut.config.FlushInterval)
	defer ticker.Stop()

	events := make([]*UsageEvent, 0, ut.config.BufferSize)

	for {
		select {
		case event := <-ut.buffer:
			events = append(events, event)
			
			// Flush if buffer is full
			if len(events) >= ut.config.BufferSize {
				ut.flushEvents(events)
				events = events[:0]
			}

		case <-ticker.C:
			// Periodic flush
			if len(events) > 0 {
				ut.flushEvents(events)
				events = events[:0]
			}

		case <-ut.stopChan:
			// Flush remaining events before stopping
			if len(events) > 0 {
				ut.flushEvents(events)
			}
			return
		}
	}
}

// flushEvents writes multiple events to the log file
func (ut *UsageTracker) flushEvents(events []*UsageEvent) {
	for _, event := range events {
		ut.writeEvent(event)
	}

	// Force sync to disk
	if ut.logFile != nil {
		ut.logFile.Sync()
	}
}

// statsLoop periodically updates statistics
func (ut *UsageTracker) statsLoop() {
	ticker := time.NewTicker(ut.config.StatsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ut.generateStats()
			ut.saveStats()
		case <-ut.stopChan:
			return
		}
	}
}

// updateSessionTracker updates session-specific tracking
func (ut *UsageTracker) updateSessionTracker(event *UsageEvent) {
	ut.mutex.Lock()
	defer ut.mutex.Unlock()

	sessionID := event.SessionID
	session, exists := ut.sessions[sessionID]

	if !exists {
		session = &SessionTracker{
			ID:           sessionID,
			Name:         event.SessionName,
			Program:      event.Program,
			StartTime:    event.Timestamp,
			LastActivity: event.Timestamp,
			Repository:   event.Repository,
			Branch:       event.Branch,
			Metadata:     make(map[string]interface{}),
			Active:       true,
		}
		ut.sessions[sessionID] = session
	}

	// Update session data
	session.LastActivity = event.Timestamp
	if event.Type == EventCommandExecuted || event.Type == EventPromptSent {
		session.CommandCount++
	}
	if event.Type == EventSessionKilled {
		session.Active = false
		session.TotalTime = event.Timestamp.Sub(session.StartTime)
	}
}

// updateStats updates real-time statistics
func (ut *UsageTracker) updateStats(event *UsageEvent) {
	ut.mutex.Lock()
	defer ut.mutex.Unlock()

	ut.stats.TotalEvents++
	ut.stats.EventsByType[event.Type]++
	ut.stats.LastUpdated = time.Now()

	// Update time range
	if event.Timestamp.Before(ut.stats.TimeRange.Start) {
		ut.stats.TimeRange.Start = event.Timestamp
	}
	if event.Timestamp.After(ut.stats.TimeRange.End) {
		ut.stats.TimeRange.End = event.Timestamp
	}

	// Update specific stats based on event type
	switch event.Type {
	case EventSessionCreated:
		ut.stats.SessionStats.TotalSessions++
		if event.Program != "" {
			ut.stats.SessionStats.SessionsByProgram[event.Program]++
		}
	case EventCommandExecuted, EventPromptSent:
		ut.stats.CommandStats.TotalCommands++
		if event.Program != "" {
			ut.stats.CommandStats.CommandsByProgram[event.Program]++
		}
	case EventGitCommit:
		ut.stats.GitStats.TotalCommits++
		if event.Repository != "" {
			ut.stats.GitStats.RepositoryActivity[event.Repository]++
		}
	case EventGitPush:
		ut.stats.GitStats.TotalPushes++
	case EventGitPull:
		ut.stats.GitStats.TotalPulls++
	case EventGitBranch:
		ut.stats.GitStats.BranchesCreated++
	case EventError:
		ut.stats.ErrorStats.TotalErrors++
		if event.ErrorMsg != "" {
			ut.stats.ErrorStats.ErrorsByType[event.ErrorMsg]++
		}
	}
}

// GetStats returns current usage statistics
func (ut *UsageTracker) GetStats() *UsageStats {
	ut.mutex.RLock()
	defer ut.mutex.RUnlock()

	// Create a deep copy to avoid race conditions
	stats := &UsageStats{
		TotalEvents:      ut.stats.TotalEvents,
		EventsByType:     make(map[UsageEventType]int64),
		SessionStats:     ut.stats.SessionStats,
		CommandStats:     ut.stats.CommandStats,
		GitStats:         ut.stats.GitStats,
		PerformanceStats: ut.stats.PerformanceStats,
		ErrorStats:       ut.stats.ErrorStats,
		TimeRange:        ut.stats.TimeRange,
		LastUpdated:      ut.stats.LastUpdated,
	}

	for k, v := range ut.stats.EventsByType {
		stats.EventsByType[k] = v
	}

	return stats
}

// GetSessions returns active session trackers
func (ut *UsageTracker) GetSessions() map[string]*SessionTracker {
	ut.mutex.RLock()
	defer ut.mutex.RUnlock()

	sessions := make(map[string]*SessionTracker)
	for k, v := range ut.sessions {
		sessions[k] = v
	}
	return sessions
}

// QueryEvents queries usage events based on filters
func (ut *UsageTracker) QueryEvents(filter UsageFilter) ([]*UsageEvent, error) {
	if !ut.config.Enabled {
		return nil, fmt.Errorf("usage tracking is disabled")
	}

	// Read and parse log file
	logPath := filepath.Join(ut.storageDir, ut.config.LogFile)
	data, err := os.ReadFile(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read usage log: %w", err)
	}

	var events []*UsageEvent
	lines := []string{}
	for _, line := range []string{string(data)} {
		lines = append(lines, line)
	}

	for _, line := range lines {
		if line == "" {
			continue
		}

		var event UsageEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue // Skip malformed events
		}

		if ut.matchesFilter(&event, filter) {
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
func (ut *UsageTracker) matchesFilter(event *UsageEvent, filter UsageFilter) bool {
	if filter.UserID != "" && event.UserID != filter.UserID {
		return false
	}
	if filter.SessionID != "" && event.SessionID != filter.SessionID {
		return false
	}
	if filter.Type != "" && event.Type != filter.Type {
		return false
	}
	if filter.Program != "" && event.Program != filter.Program {
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
	if filter.Repository != "" && event.Repository != filter.Repository {
		return false
	}
	return true
}

// generateStats generates comprehensive statistics
func (ut *UsageTracker) generateStats() {
	ut.mutex.Lock()
	defer ut.mutex.Unlock()

	// Calculate active sessions
	activeCount := int64(0)
	totalSessionTime := time.Duration(0)
	sessionCount := int64(0)

	for _, session := range ut.sessions {
		if session.Active {
			activeCount++
		}
		if !session.TotalTime.Nanoseconds() != 0 {
			totalSessionTime += session.TotalTime
			sessionCount++
		}
	}

	ut.stats.SessionStats.ActiveSessions = activeCount
	if sessionCount > 0 {
		ut.stats.SessionStats.AverageSessionTime = totalSessionTime / time.Duration(sessionCount)
	}

	// Calculate commands per session
	if ut.stats.SessionStats.TotalSessions > 0 {
		ut.stats.CommandStats.CommandsPerSession = float64(ut.stats.CommandStats.TotalCommands) / float64(ut.stats.SessionStats.TotalSessions)
	}

	// Calculate error rate
	if ut.stats.TotalEvents > 0 {
		ut.stats.ErrorStats.ErrorRate = float64(ut.stats.ErrorStats.TotalErrors) / float64(ut.stats.TotalEvents)
	}
}

// saveStats saves statistics to disk
func (ut *UsageTracker) saveStats() {
	statsPath := filepath.Join(ut.storageDir, "usage_stats.json")
	data, err := json.MarshalIndent(ut.stats, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(statsPath, data, 0644)
}

// loadStats loads statistics from disk
func (ut *UsageTracker) loadStats() error {
	statsPath := filepath.Join(ut.storageDir, "usage_stats.json")
	data, err := os.ReadFile(statsPath)
	if err != nil {
		return err // File might not exist yet
	}

	return json.Unmarshal(data, ut.stats)
}

// Stop stops the usage tracker
func (ut *UsageTracker) Stop() error {
	if !ut.running {
		return nil
	}

	// Log system stop
	ut.TrackEvent(UsageEvent{
		Type:      EventSystemStop,
		Timestamp: time.Now(),
		Success:   true,
		Source:    "usage_tracker",
	})

	// Stop processing loops
	close(ut.stopChan)

	// Close log file
	if ut.logFile != nil {
		ut.logFile.Close()
	}

	// Save final stats
	ut.generateStats()
	ut.saveStats()

	ut.running = false
	return nil
}

// Helper functions

func generateEventID() string {
	return fmt.Sprintf("evt_%d", time.Now().UnixNano())
}

// DefaultTrackerConfig returns default tracker configuration
func DefaultTrackerConfig() *TrackerConfig {
	return &TrackerConfig{
		Enabled:           true,
		StorageDir:        "~/.claude-squad/monitoring",
		LogFile:           "usage.log",
		BufferSize:        1000,
		FlushInterval:     time.Second * 30,
		RetentionDays:     30,
		StatsInterval:     time.Minute * 5,
		AsyncLogging:      true,
		CompressOldLogs:   true,
		EnablePerformance: true,
		EnableDetailed:    true,
	}
}

// ProductionTrackerConfig returns production-optimized configuration
func ProductionTrackerConfig() *TrackerConfig {
	config := DefaultTrackerConfig()
	config.BufferSize = 5000
	config.FlushInterval = time.Second * 10
	config.StatsInterval = time.Minute * 1
	config.RetentionDays = 90
	return config
}