package monitoring

import (
	"claude-squad/config"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MonitoringIntegration provides a unified monitoring interface for claude-squad
type MonitoringIntegration struct {
	config           *MonitoringConfig
	tracker          *UsageTracker
	metrics          *MetricsCollector
	dashboard        *Dashboard
	reporter         *ReportGenerator
	mutex            sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc
	running          bool
	sessionTracker   map[string]*SessionContext
	performanceTimer *PerformanceTimer
}

// MonitoringConfig represents comprehensive monitoring configuration
type MonitoringConfig struct {
	Enabled          bool           `json:"enabled"`
	StorageDir       string         `json:"storage_dir"`
	Usage            TrackerConfig  `json:"usage"`
	Metrics          MetricsConfig  `json:"metrics"`
	Dashboard        DashboardConfig `json:"dashboard"`
	Reporting        ReportConfig   `json:"reporting"`
	Integration      IntegrationConfig `json:"integration"`
}

// IntegrationConfig represents integration-specific settings
type IntegrationConfig struct {
	TrackSessions     bool `json:"track_sessions"`
	TrackCommands     bool `json:"track_commands"`
	TrackGitOps       bool `json:"track_git_ops"`
	TrackPerformance  bool `json:"track_performance"`
	TrackErrors       bool `json:"track_errors"`
	TrackSecurity     bool `json:"track_security"`
	AutoStartStop     bool `json:"auto_start_stop"`
	RealTimeUpdates   bool `json:"real_time_updates"`
}

// SessionContext tracks context for active sessions
type SessionContext struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Program      string                 `json:"program"`
	StartTime    time.Time              `json:"start_time"`
	LastActivity time.Time              `json:"last_activity"`
	CommandCount int64                  `json:"command_count"`
	Repository   string                 `json:"repository"`
	Branch       string                 `json:"branch"`
	UserID       string                 `json:"user_id"`
	Metadata     map[string]interface{} `json:"metadata"`
	Performance  PerformanceData        `json:"performance"`
}

// PerformanceData tracks performance metrics for sessions
type PerformanceData struct {
	CommandTimes    []time.Duration `json:"command_times"`
	ResponseTimes   []time.Duration `json:"response_times"`
	AverageTime     time.Duration   `json:"average_time"`
	TotalTime       time.Duration   `json:"total_time"`
	ErrorCount      int64           `json:"error_count"`
	SuccessCount    int64           `json:"success_count"`
}

// PerformanceTimer tracks operation timing
type PerformanceTimer struct {
	operations map[string]time.Time
	mutex      sync.RWMutex
}

// MonitoringStats provides comprehensive monitoring statistics
type MonitoringStats struct {
	Overview         OverviewStats      `json:"overview"`
	SessionStats     SessionStats       `json:"session_stats"`
	PerformanceStats PerformanceStats   `json:"performance_stats"`
	UsageStats       *UsageStats        `json:"usage_stats"`
	SystemMetrics    *SystemMetrics     `json:"system_metrics"`
	Trends           TrendAnalysis      `json:"trends"`
	Health           HealthOverview     `json:"health"`
	LastUpdated      time.Time          `json:"last_updated"`
}

// OverviewStats provides high-level statistics
type OverviewStats struct {
	TotalSessions       int64         `json:"total_sessions"`
	ActiveSessions      int64         `json:"active_sessions"`
	TotalCommands       int64         `json:"total_commands"`
	TotalGitOperations  int64         `json:"total_git_operations"`
	SystemUptime        time.Duration `json:"system_uptime"`
	AverageSessionTime  time.Duration `json:"average_session_time"`
	CommandsPerSession  float64       `json:"commands_per_session"`
	ErrorRate           float64       `json:"error_rate"`
	TopPrograms         []ProgramUsage `json:"top_programs"`
	TopRepositories     []RepoActivity `json:"top_repositories"`
}

// TrendAnalysis provides trend analysis data
type TrendAnalysis struct {
	UsageGrowth        TrendData `json:"usage_growth"`
	PerformanceTrend   TrendData `json:"performance_trend"`
	ErrorTrend         TrendData `json:"error_trend"`
	PopularityTrends   map[string]TrendData `json:"popularity_trends"`
	PredictedGrowth    float64   `json:"predicted_growth"`
	RecommendedActions []string  `json:"recommended_actions"`
}

// TrendData represents trending information
type TrendData struct {
	Direction    TrendDirection `json:"direction"`
	Percentage   float64        `json:"percentage"`
	Confidence   float64        `json:"confidence"`
	DataPoints   []float64      `json:"data_points"`
	TimeRange    string         `json:"time_range"`
}

// HealthOverview provides system health overview
type HealthOverview struct {
	OverallStatus    HealthStatus             `json:"overall_status"`
	ComponentStatus  map[string]HealthStatus  `json:"component_status"`
	ActiveAlerts     []ActiveAlert            `json:"active_alerts"`
	HealthScore      float64                  `json:"health_score"`
	Recommendations  []string                 `json:"recommendations"`
	LastHealthCheck  time.Time                `json:"last_health_check"`
}

// NewMonitoringIntegration creates a new monitoring integration
func NewMonitoringIntegration(appConfig *config.Config) (*MonitoringIntegration, error) {
	// Load monitoring configuration
	monitoringConfig, err := LoadMonitoringConfig(appConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load monitoring config: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	integration := &MonitoringIntegration{
		config:           monitoringConfig,
		ctx:              ctx,
		cancel:           cancel,
		sessionTracker:   make(map[string]*SessionContext),
		performanceTimer: NewPerformanceTimer(),
	}

	if !monitoringConfig.Enabled {
		return integration, nil
	}

	// Create storage directory
	if err := os.MkdirAll(monitoringConfig.StorageDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create monitoring storage directory: %w", err)
	}

	// Initialize usage tracker
	integration.tracker, err = NewUsageTracker(&monitoringConfig.Usage)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize usage tracker: %w", err)
	}

	// Initialize metrics collector
	integration.metrics, err = NewMetricsCollector(&monitoringConfig.Metrics, integration.tracker)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize metrics collector: %w", err)
	}

	// Initialize dashboard
	integration.dashboard, err = NewDashboard(&monitoringConfig.Dashboard, integration)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize dashboard: %w", err)
	}

	// Initialize reporter
	integration.reporter, err = NewReportGenerator(&monitoringConfig.Reporting, integration)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize report generator: %w", err)
	}

	integration.running = true
	return integration, nil
}

// Session tracking methods

// TrackSessionCreated tracks session creation
func (mi *MonitoringIntegration) TrackSessionCreated(sessionID, sessionName, program, repository, userID string) {
	if !mi.config.Enabled || !mi.config.Integration.TrackSessions {
		return
	}

	mi.mutex.Lock()
	defer mi.mutex.Unlock()

	// Create session context
	sessionCtx := &SessionContext{
		ID:           sessionID,
		Name:         sessionName,
		Program:      program,
		StartTime:    time.Now(),
		LastActivity: time.Now(),
		Repository:   repository,
		UserID:       userID,
		Metadata:     make(map[string]interface{}),
		Performance:  PerformanceData{
			CommandTimes:  make([]time.Duration, 0),
			ResponseTimes: make([]time.Duration, 0),
		},
	}

	mi.sessionTracker[sessionID] = sessionCtx

	// Track usage event
	mi.tracker.TrackEvent(UsageEvent{
		Type:        EventSessionCreated,
		Timestamp:   time.Now(),
		UserID:      userID,
		SessionID:   sessionID,
		SessionName: sessionName,
		Program:     program,
		Repository:  repository,
		Success:     true,
		Source:      "monitoring_integration",
		Metadata: map[string]interface{}{
			"session_created": true,
		},
	})
}

// TrackSessionAttached tracks session attachment
func (mi *MonitoringIntegration) TrackSessionAttached(sessionID, userID string) {
	if !mi.config.Enabled || !mi.config.Integration.TrackSessions {
		return
	}

	mi.updateSessionActivity(sessionID)

	mi.tracker.TrackEvent(UsageEvent{
		Type:      EventSessionAttached,
		Timestamp: time.Now(),
		UserID:    userID,
		SessionID: sessionID,
		Success:   true,
		Source:    "monitoring_integration",
	})
}

// TrackSessionKilled tracks session termination
func (mi *MonitoringIntegration) TrackSessionKilled(sessionID, userID string) {
	if !mi.config.Enabled || !mi.config.Integration.TrackSessions {
		return
	}

	mi.mutex.Lock()
	sessionCtx, exists := mi.sessionTracker[sessionID]
	if exists {
		sessionCtx.Performance.TotalTime = time.Since(sessionCtx.StartTime)
		delete(mi.sessionTracker, sessionID)
	}
	mi.mutex.Unlock()

	duration := time.Duration(0)
	if exists {
		duration = sessionCtx.Performance.TotalTime
	}

	mi.tracker.TrackEvent(UsageEvent{
		Type:      EventSessionKilled,
		Timestamp: time.Now(),
		UserID:    userID,
		SessionID: sessionID,
		Duration:  duration,
		Success:   true,
		Source:    "monitoring_integration",
		Metadata: map[string]interface{}{
			"session_duration": duration.String(),
		},
	})
}

// Command tracking methods

// TrackCommandExecuted tracks command execution
func (mi *MonitoringIntegration) TrackCommandExecuted(sessionID, userID, command string, duration time.Duration, success bool, errorMsg string) {
	if !mi.config.Enabled || !mi.config.Integration.TrackCommands {
		return
	}

	mi.updateSessionActivity(sessionID)
	mi.updateSessionPerformance(sessionID, duration, success)

	mi.tracker.TrackEvent(UsageEvent{
		Type:      EventCommandExecuted,
		Timestamp: time.Now(),
		UserID:    userID,
		SessionID: sessionID,
		Command:   command,
		Duration:  duration,
		Success:   success,
		ErrorMsg:  errorMsg,
		Source:    "monitoring_integration",
		Metadata: map[string]interface{}{
			"command_executed": true,
			"execution_time":   duration.String(),
		},
	})
}

// TrackPromptSent tracks prompt sending
func (mi *MonitoringIntegration) TrackPromptSent(sessionID, userID, prompt string) {
	if !mi.config.Enabled || !mi.config.Integration.TrackCommands {
		return
	}

	mi.updateSessionActivity(sessionID)

	mi.tracker.TrackEvent(UsageEvent{
		Type:      EventPromptSent,
		Timestamp: time.Now(),
		UserID:    userID,
		SessionID: sessionID,
		Success:   true,
		Source:    "monitoring_integration",
		Metadata: map[string]interface{}{
			"prompt_length": len(prompt),
		},
	})
}

// Git tracking methods

// TrackGitCommit tracks git commits
func (mi *MonitoringIntegration) TrackGitCommit(sessionID, userID, repository, branch, commitHash string, success bool) {
	if !mi.config.Enabled || !mi.config.Integration.TrackGitOps {
		return
	}

	mi.updateSessionActivity(sessionID)

	mi.tracker.TrackEvent(UsageEvent{
		Type:       EventGitCommit,
		Timestamp:  time.Now(),
		UserID:     userID,
		SessionID:  sessionID,
		Repository: repository,
		Branch:     branch,
		Success:    success,
		Source:     "monitoring_integration",
		Metadata: map[string]interface{}{
			"commit_hash": commitHash,
		},
	})
}

// TrackGitPush tracks git pushes
func (mi *MonitoringIntegration) TrackGitPush(sessionID, userID, repository, branch string, success bool, errorMsg string) {
	if !mi.config.Enabled || !mi.config.Integration.TrackGitOps {
		return
	}

	mi.updateSessionActivity(sessionID)

	mi.tracker.TrackEvent(UsageEvent{
		Type:       EventGitPush,
		Timestamp:  time.Now(),
		UserID:     userID,
		SessionID:  sessionID,
		Repository: repository,
		Branch:     branch,
		Success:    success,
		ErrorMsg:   errorMsg,
		Source:     "monitoring_integration",
	})
}

// Performance tracking methods

// StartOperation starts timing an operation
func (mi *MonitoringIntegration) StartOperation(operationID string) {
	if !mi.config.Enabled || !mi.config.Integration.TrackPerformance {
		return
	}

	mi.performanceTimer.Start(operationID)
}

// EndOperation ends timing an operation and tracks the result
func (mi *MonitoringIntegration) EndOperation(operationID, sessionID, userID string, success bool) time.Duration {
	if !mi.config.Enabled || !mi.config.Integration.TrackPerformance {
		return 0
	}

	duration := mi.performanceTimer.End(operationID)

	mi.tracker.TrackEvent(UsageEvent{
		Type:      EventPerformance,
		Timestamp: time.Now(),
		UserID:    userID,
		SessionID: sessionID,
		Duration:  duration,
		Success:   success,
		Source:    "monitoring_integration",
		Metadata: map[string]interface{}{
			"operation_id":   operationID,
			"operation_time": duration.String(),
		},
	})

	return duration
}

// Error tracking methods

// TrackError tracks errors
func (mi *MonitoringIntegration) TrackError(sessionID, userID, errorType, errorMessage string, severity string) {
	if !mi.config.Enabled || !mi.config.Integration.TrackErrors {
		return
	}

	mi.updateSessionPerformance(sessionID, 0, false)

	mi.tracker.TrackEvent(UsageEvent{
		Type:      EventError,
		Timestamp: time.Now(),
		UserID:    userID,
		SessionID: sessionID,
		Success:   false,
		ErrorMsg:  errorMessage,
		Source:    "monitoring_integration",
		Metadata: map[string]interface{}{
			"error_type": errorType,
			"severity":   severity,
		},
	})
}

// Stats and reporting methods

// GetStats returns comprehensive monitoring statistics
func (mi *MonitoringIntegration) GetStats() *MonitoringStats {
	if !mi.config.Enabled {
		return &MonitoringStats{LastUpdated: time.Now()}
	}

	usageStats := mi.tracker.GetStats()
	systemMetrics := mi.metrics.GetMetrics()

	overview := mi.calculateOverviewStats(usageStats)
	trends := mi.analyzeTrends(usageStats, systemMetrics)
	health := mi.assessHealth(systemMetrics)

	return &MonitoringStats{
		Overview:         overview,
		SessionStats:     usageStats.SessionStats,
		PerformanceStats: systemMetrics.Performance,
		UsageStats:       usageStats,
		SystemMetrics:    systemMetrics,
		Trends:           trends,
		Health:           health,
		LastUpdated:      time.Now(),
	}
}

// GetDashboard returns dashboard data
func (mi *MonitoringIntegration) GetDashboard() *DashboardData {
	if !mi.config.Enabled || mi.dashboard == nil {
		return &DashboardData{}
	}

	return mi.dashboard.GetData()
}

// GenerateReport generates a monitoring report
func (mi *MonitoringIntegration) GenerateReport(reportType string, timeRange TimeRange) (*Report, error) {
	if !mi.config.Enabled || mi.reporter == nil {
		return nil, fmt.Errorf("monitoring is disabled")
	}

	return mi.reporter.GenerateReport(reportType, timeRange)
}

// System control methods

// Start starts the monitoring system
func (mi *MonitoringIntegration) Start() error {
	if !mi.config.Enabled {
		return nil
	}

	if mi.running {
		return nil
	}

	// Start components that need explicit starting
	if mi.dashboard != nil {
		if err := mi.dashboard.Start(); err != nil {
			return fmt.Errorf("failed to start dashboard: %w", err)
		}
	}

	mi.running = true

	// Track system start
	mi.tracker.TrackEvent(UsageEvent{
		Type:      EventSystemStart,
		Timestamp: time.Now(),
		Success:   true,
		Source:    "monitoring_integration",
	})

	return nil
}

// Stop stops the monitoring system
func (mi *MonitoringIntegration) Stop() error {
	if !mi.running {
		return nil
	}

	// Track system stop
	if mi.config.Enabled {
		mi.tracker.TrackEvent(UsageEvent{
			Type:      EventSystemStop,
			Timestamp: time.Now(),
			Success:   true,
			Source:    "monitoring_integration",
		})
	}

	// Stop components
	if mi.tracker != nil {
		mi.tracker.Stop()
	}
	if mi.metrics != nil {
		mi.metrics.Stop()
	}
	if mi.dashboard != nil {
		mi.dashboard.Stop()
	}

	mi.cancel()
	mi.running = false
	return nil
}

// IsEnabled returns whether monitoring is enabled
func (mi *MonitoringIntegration) IsEnabled() bool {
	return mi.config.Enabled
}

// Private helper methods

func (mi *MonitoringIntegration) updateSessionActivity(sessionID string) {
	mi.mutex.Lock()
	defer mi.mutex.Unlock()

	if session, exists := mi.sessionTracker[sessionID]; exists {
		session.LastActivity = time.Now()
		session.CommandCount++
	}
}

func (mi *MonitoringIntegration) updateSessionPerformance(sessionID string, duration time.Duration, success bool) {
	mi.mutex.Lock()
	defer mi.mutex.Unlock()

	if session, exists := mi.sessionTracker[sessionID]; exists {
		if duration > 0 {
			session.Performance.CommandTimes = append(session.Performance.CommandTimes, duration)
		}
		
		if success {
			session.Performance.SuccessCount++
		} else {
			session.Performance.ErrorCount++
		}

		// Calculate average time
		if len(session.Performance.CommandTimes) > 0 {
			total := time.Duration(0)
			for _, d := range session.Performance.CommandTimes {
				total += d
			}
			session.Performance.AverageTime = total / time.Duration(len(session.Performance.CommandTimes))
		}
	}
}

func (mi *MonitoringIntegration) calculateOverviewStats(usageStats *UsageStats) OverviewStats {
	mi.mutex.RLock()
	activeCount := int64(len(mi.sessionTracker))
	mi.mutex.RUnlock()

	// Create top programs list
	topPrograms := make([]ProgramUsage, 0, len(usageStats.SessionStats.SessionsByProgram))
	total := usageStats.SessionStats.TotalSessions
	
	for program, count := range usageStats.SessionStats.SessionsByProgram {
		percentage := float64(count) / float64(total) * 100
		topPrograms = append(topPrograms, ProgramUsage{
			Program:    program,
			Count:      count,
			Percentage: percentage,
		})
	}

	// Sort by count
	for i := 0; i < len(topPrograms)-1; i++ {
		for j := i + 1; j < len(topPrograms); j++ {
			if topPrograms[j].Count > topPrograms[i].Count {
				topPrograms[i], topPrograms[j] = topPrograms[j], topPrograms[i]
			}
		}
	}

	// Limit to top 5
	if len(topPrograms) > 5 {
		topPrograms = topPrograms[:5]
	}

	return OverviewStats{
		TotalSessions:      usageStats.SessionStats.TotalSessions,
		ActiveSessions:     activeCount,
		TotalCommands:      usageStats.CommandStats.TotalCommands,
		TotalGitOperations: usageStats.GitStats.TotalCommits + usageStats.GitStats.TotalPushes + usageStats.GitStats.TotalPulls,
		AverageSessionTime: usageStats.SessionStats.AverageSessionTime,
		CommandsPerSession: usageStats.CommandStats.CommandsPerSession,
		ErrorRate:          usageStats.ErrorStats.ErrorRate,
		TopPrograms:        topPrograms,
		TopRepositories:    []RepoActivity{}, // Would be calculated from repository activity
	}
}

func (mi *MonitoringIntegration) analyzeTrends(usageStats *UsageStats, systemMetrics *SystemMetrics) TrendAnalysis {
	// Simple trend analysis - in production this would be more sophisticated
	return TrendAnalysis{
		UsageGrowth: TrendData{
			Direction:  TrendStable,
			Percentage: 0,
			Confidence: 0.5,
			TimeRange:  "24h",
		},
		PerformanceTrend: TrendData{
			Direction:  TrendStable,
			Percentage: 0,
			Confidence: 0.5,
			TimeRange:  "24h",
		},
		ErrorTrend: TrendData{
			Direction:  TrendStable,
			Percentage: 0,
			Confidence: 0.5,
			TimeRange:  "24h",
		},
		PopularityTrends:   make(map[string]TrendData),
		PredictedGrowth:    0,
		RecommendedActions: []string{},
	}
}

func (mi *MonitoringIntegration) assessHealth(systemMetrics *SystemMetrics) HealthOverview {
	return HealthOverview{
		OverallStatus:   systemMetrics.Health.OverallHealth,
		ComponentStatus: systemMetrics.Health.ComponentHealth,
		ActiveAlerts:    systemMetrics.Health.Alerts,
		HealthScore:     systemMetrics.Health.PerformanceScore,
		Recommendations: []string{},
		LastHealthCheck: time.Now(),
	}
}

// NewPerformanceTimer creates a new performance timer
func NewPerformanceTimer() *PerformanceTimer {
	return &PerformanceTimer{
		operations: make(map[string]time.Time),
	}
}

// Start starts timing an operation
func (pt *PerformanceTimer) Start(operationID string) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()
	pt.operations[operationID] = time.Now()
}

// End ends timing an operation and returns the duration
func (pt *PerformanceTimer) End(operationID string) time.Duration {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()
	
	if startTime, exists := pt.operations[operationID]; exists {
		duration := time.Since(startTime)
		delete(pt.operations, operationID)
		return duration
	}
	return 0
}

// LoadMonitoringConfig loads monitoring configuration
func LoadMonitoringConfig(appConfig *config.Config) (*MonitoringConfig, error) {
	// Use configuration from app config if available, otherwise use defaults
	enabled := true
	trackSessions := true
	trackCommands := true
	trackGitOps := true
	trackPerformance := true
	trackErrors := true
	dashboardPort := 8080
	dashboardEnabled := true
	
	// Override with app config if monitoring section exists
	if appConfig != nil {
		enabled = appConfig.Monitoring.Enabled
		trackSessions = appConfig.Monitoring.TrackSessions
		trackCommands = appConfig.Monitoring.TrackCommands
		trackGitOps = appConfig.Monitoring.TrackGitOps
		trackPerformance = appConfig.Monitoring.TrackPerformance
		trackErrors = appConfig.Monitoring.TrackErrors
		if appConfig.Monitoring.DashboardPort > 0 {
			dashboardPort = appConfig.Monitoring.DashboardPort
		}
		dashboardEnabled = appConfig.Monitoring.DashboardEnabled
	}

	// Create configuration
	monitoringConfig := &MonitoringConfig{
		Enabled:    enabled,
		StorageDir: "~/.claude-squad/monitoring",
		Usage:      *DefaultTrackerConfig(),
		Metrics:    *DefaultMetricsConfig(),
		Dashboard:  *DefaultDashboardConfig(),
		Reporting:  *DefaultReportConfig(),
		Integration: IntegrationConfig{
			TrackSessions:    trackSessions,
			TrackCommands:    trackCommands,
			TrackGitOps:      trackGitOps,
			TrackPerformance: trackPerformance,
			TrackErrors:      trackErrors,
			TrackSecurity:    false,
			AutoStartStop:    true,
			RealTimeUpdates:  true,
		},
	}
	
	// Update dashboard config
	monitoringConfig.Dashboard.Enabled = dashboardEnabled
	monitoringConfig.Dashboard.Port = dashboardPort

	// Expand storage directory path
	if monitoringConfig.StorageDir[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		monitoringConfig.StorageDir = filepath.Join(homeDir, monitoringConfig.StorageDir[2:])
	}

	// Update nested storage directories
	monitoringConfig.Usage.StorageDir = monitoringConfig.StorageDir
	monitoringConfig.Metrics.StorageDir = filepath.Join(monitoringConfig.StorageDir, "metrics")
	monitoringConfig.Reporting.StorageDir = filepath.Join(monitoringConfig.StorageDir, "reports")

	return monitoringConfig, nil
}