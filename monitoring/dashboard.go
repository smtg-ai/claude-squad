package monitoring

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"sync"
	"time"
)

// Dashboard provides a web-based monitoring dashboard
type Dashboard struct {
	config     *DashboardConfig
	integration *MonitoringIntegration
	server     *http.Server
	data       *DashboardData
	mutex      sync.RWMutex
	running    bool
}

// DashboardConfig represents dashboard configuration
type DashboardConfig struct {
	Enabled        bool          `json:"enabled"`
	Port           int           `json:"port"`
	Host           string        `json:"host"`
	RefreshInterval time.Duration `json:"refresh_interval"`
	EnableAuth     bool          `json:"enable_auth"`
	Theme          string        `json:"theme"` // light, dark, auto
	Layout         string        `json:"layout"` // compact, detailed, custom
	EnableRealtime bool          `json:"enable_realtime"`
	MaxDataPoints  int           `json:"max_data_points"`
	CustomWidgets  []WidgetConfig `json:"custom_widgets"`
}

// DashboardData represents the complete dashboard data structure
type DashboardData struct {
	Overview     OverviewWidget     `json:"overview"`
	Sessions     SessionWidget      `json:"sessions"`
	Performance  PerformanceWidget  `json:"performance"`
	Usage        UsageWidget        `json:"usage"`
	System       SystemWidget       `json:"system"`
	Git          GitWidget          `json:"git"`
	Errors       ErrorWidget        `json:"errors"`
	Health       HealthWidget       `json:"health"`
	Trends       TrendWidget        `json:"trends"`
	Alerts       AlertWidget        `json:"alerts"`
	Charts       ChartData          `json:"charts"`
	LastUpdated  time.Time          `json:"last_updated"`
}

// Widget interfaces and implementations
type Widget interface {
	Update(integration *MonitoringIntegration)
	GetData() interface{}
}

// OverviewWidget displays high-level statistics
type OverviewWidget struct {
	TotalSessions      int64         `json:"total_sessions"`
	ActiveSessions     int64         `json:"active_sessions"`
	TotalCommands      int64         `json:"total_commands"`
	CommandsToday      int64         `json:"commands_today"`
	AverageSessionTime time.Duration `json:"average_session_time"`
	SystemUptime       time.Duration `json:"system_uptime"`
	ErrorRate          float64       `json:"error_rate"`
	SuccessRate        float64       `json:"success_rate"`
	TopProgram         string        `json:"top_program"`
	MostActiveRepo     string        `json:"most_active_repo"`
}

// SessionWidget displays session information
type SessionWidget struct {
	ActiveSessions     []ActiveSessionInfo `json:"active_sessions"`
	RecentSessions     []SessionSummary    `json:"recent_sessions"`
	SessionsByProgram  map[string]int64    `json:"sessions_by_program"`
	SessionDistribution []TimeDistribution `json:"session_distribution"`
	AverageMetrics     SessionMetrics      `json:"average_metrics"`
}

// PerformanceWidget displays performance metrics
type PerformanceWidget struct {
	CPU            CPUInfo         `json:"cpu"`
	Memory         MemoryInfo      `json:"memory"`
	Goroutines     GoroutineInfo   `json:"goroutines"`
	GC             GCInfo          `json:"gc"`
	ResponseTimes  ResponseInfo    `json:"response_times"`
	Throughput     ThroughputInfo  `json:"throughput"`
	PerformanceScore float64       `json:"performance_score"`
	Alerts         []string        `json:"alerts"`
}

// UsageWidget displays usage patterns
type UsageWidget struct {
	HourlyUsage       []UsageDataPoint  `json:"hourly_usage"`
	DailyUsage        []UsageDataPoint  `json:"daily_usage"`
	ProgramUsage      []ProgramUsageInfo `json:"program_usage"`
	CommandFrequency  []CommandInfo     `json:"command_frequency"`
	UsageTrends       TrendInfo         `json:"usage_trends"`
	PeakHours         []string          `json:"peak_hours"`
}

// SystemWidget displays system information
type SystemWidget struct {
	OS               string            `json:"os"`
	Architecture     string            `json:"architecture"`
	GoVersion        string            `json:"go_version"`
	CPUCount         int               `json:"cpu_count"`
	ProcessID        int               `json:"process_id"`
	WorkingDirectory string            `json:"working_directory"`
	ConfigPath       string            `json:"config_path"`
	Uptime           time.Duration     `json:"uptime"`
	Environment      map[string]string `json:"environment"`
}

// GitWidget displays git operation statistics
type GitWidget struct {
	TotalCommits       int64                    `json:"total_commits"`
	TotalPushes        int64                    `json:"total_pushes"`
	TotalPulls         int64                    `json:"total_pulls"`
	BranchesCreated    int64                    `json:"branches_created"`
	RepositoryActivity []RepositoryActivity     `json:"repository_activity"`
	RecentOperations   []GitOperation           `json:"recent_operations"`
	CommitFrequency    []TimeDistribution       `json:"commit_frequency"`
}

// ErrorWidget displays error information
type ErrorWidget struct {
	TotalErrors        int64              `json:"total_errors"`
	ErrorRate          float64            `json:"error_rate"`
	ErrorsByType       []ErrorTypeInfo    `json:"errors_by_type"`
	RecentErrors       []ErrorInfo        `json:"recent_errors"`
	ErrorTrends        []TimeDistribution `json:"error_trends"`
	TopErrors          []ErrorFrequency   `json:"top_errors"`
}

// HealthWidget displays system health
type HealthWidget struct {
	OverallStatus     HealthStatus              `json:"overall_status"`
	HealthScore       float64                   `json:"health_score"`
	ComponentHealth   []ComponentHealthInfo     `json:"component_health"`
	ActiveAlerts      []AlertInfo               `json:"active_alerts"`
	HealthHistory     []HealthHistoryPoint      `json:"health_history"`
	Recommendations   []string                  `json:"recommendations"`
}

// TrendWidget displays trending information
type TrendWidget struct {
	UsageTrend         TrendInfo    `json:"usage_trend"`
	PerformanceTrend   TrendInfo    `json:"performance_trend"`
	ErrorTrend         TrendInfo    `json:"error_trend"`
	PopularityTrends   []TrendInfo  `json:"popularity_trends"`
	Predictions        []Prediction `json:"predictions"`
	GrowthMetrics      GrowthInfo   `json:"growth_metrics"`
}

// AlertWidget displays alerts and notifications
type AlertWidget struct {
	ActiveAlerts      []AlertInfo      `json:"active_alerts"`
	RecentAlerts      []AlertInfo      `json:"recent_alerts"`
	AlertsByType      []AlertTypeInfo  `json:"alerts_by_type"`
	AlertTrends       []TimeDistribution `json:"alert_trends"`
	AlertSettings     AlertSettings    `json:"alert_settings"`
}

// Supporting data structures
type ActiveSessionInfo struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Program      string        `json:"program"`
	StartTime    time.Time     `json:"start_time"`
	Duration     time.Duration `json:"duration"`
	CommandCount int64         `json:"command_count"`
	Repository   string        `json:"repository"`
	Status       string        `json:"status"`
}

type SessionSummary struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Program      string        `json:"program"`
	Duration     time.Duration `json:"duration"`
	CommandCount int64         `json:"command_count"`
	Success      bool          `json:"success"`
	EndTime      time.Time     `json:"end_time"`
}

type SessionMetrics struct {
	AverageDuration    time.Duration `json:"average_duration"`
	AverageCommands    float64       `json:"average_commands"`
	SuccessRate        float64       `json:"success_rate"`
	CommandsPerMinute  float64       `json:"commands_per_minute"`
}

type TimeDistribution struct {
	Time  time.Time `json:"time"`
	Count int64     `json:"count"`
	Value float64   `json:"value"`
}

type UsageDataPoint struct {
	Time     time.Time `json:"time"`
	Sessions int64     `json:"sessions"`
	Commands int64     `json:"commands"`
	Errors   int64     `json:"errors"`
}

type ProgramUsageInfo struct {
	Program     string  `json:"program"`
	Sessions    int64   `json:"sessions"`
	Commands    int64   `json:"commands"`
	Percentage  float64 `json:"percentage"`
	Trend       string  `json:"trend"`
}

type CommandInfo struct {
	Command   string  `json:"command"`
	Count     int64   `json:"count"`
	AvgTime   time.Duration `json:"avg_time"`
	Success   float64 `json:"success_rate"`
}

type CPUInfo struct {
	Usage      float64 `json:"usage"`
	LoadAvg    float64 `json:"load_avg"`
	Cores      int     `json:"cores"`
	Status     string  `json:"status"`
}

type MemoryInfo struct {
	Used       uint64  `json:"used"`
	Total      uint64  `json:"total"`
	Percentage float64 `json:"percentage"`
	Status     string  `json:"status"`
}

type GoroutineInfo struct {
	Count   int    `json:"count"`
	Status  string `json:"status"`
	Trend   string `json:"trend"`
}

type GCInfo struct {
	Collections   uint32        `json:"collections"`
	LastPause     time.Duration `json:"last_pause"`
	TotalPause    time.Duration `json:"total_pause"`
	Status        string        `json:"status"`
}

type ResponseInfo struct {
	Average time.Duration `json:"average"`
	P95     time.Duration `json:"p95"`
	P99     time.Duration `json:"p99"`
	Status  string        `json:"status"`
}

type ThroughputInfo struct {
	RequestsPerSecond float64 `json:"requests_per_second"`
	CommandsPerMinute float64 `json:"commands_per_minute"`
	Trend             string  `json:"trend"`
}

type RepositoryActivity struct {
	Name       string    `json:"name"`
	Commits    int64     `json:"commits"`
	Pushes     int64     `json:"pushes"`
	LastCommit time.Time `json:"last_commit"`
	Active     bool      `json:"active"`
}

type GitOperation struct {
	Type       string    `json:"type"`
	Repository string    `json:"repository"`
	Branch     string    `json:"branch"`
	Time       time.Time `json:"time"`
	Success    bool      `json:"success"`
	User       string    `json:"user"`
}

type ErrorTypeInfo struct {
	Type    string `json:"type"`
	Count   int64  `json:"count"`
	Percent float64 `json:"percent"`
	Trend   string `json:"trend"`
}

type ErrorInfo struct {
	Time     time.Time `json:"time"`
	Type     string    `json:"type"`
	Message  string    `json:"message"`
	Session  string    `json:"session"`
	Severity string    `json:"severity"`
}

type ComponentHealthInfo struct {
	Component string       `json:"component"`
	Status    HealthStatus `json:"status"`
	Score     float64      `json:"score"`
	Message   string       `json:"message"`
}

type AlertInfo struct {
	ID        string       `json:"id"`
	Type      string       `json:"type"`
	Message   string       `json:"message"`
	Severity  HealthStatus `json:"severity"`
	Time      time.Time    `json:"time"`
	Acknowledged bool      `json:"acknowledged"`
}

type AlertTypeInfo struct {
	Type  string `json:"type"`
	Count int64  `json:"count"`
	Last  time.Time `json:"last"`
}

type AlertSettings struct {
	Enabled     bool                  `json:"enabled"`
	Thresholds  map[string]float64    `json:"thresholds"`
	Recipients  []string              `json:"recipients"`
}

type HealthHistoryPoint struct {
	Time   time.Time `json:"time"`
	Score  float64   `json:"score"`
	Status HealthStatus `json:"status"`
}

type TrendInfo struct {
	Direction  TrendDirection `json:"direction"`
	Percentage float64        `json:"percentage"`
	Period     string         `json:"period"`
	Confidence float64        `json:"confidence"`
}

type Prediction struct {
	Metric     string    `json:"metric"`
	Value      float64   `json:"value"`
	Time       time.Time `json:"time"`
	Confidence float64   `json:"confidence"`
}

type GrowthInfo struct {
	Daily   float64 `json:"daily"`
	Weekly  float64 `json:"weekly"`
	Monthly float64 `json:"monthly"`
	Trend   string  `json:"trend"`
}

type ChartData struct {
	UsageChart       ChartDataset   `json:"usage_chart"`
	PerformanceChart ChartDataset   `json:"performance_chart"`
	ErrorChart       ChartDataset   `json:"error_chart"`
	SessionChart     ChartDataset   `json:"session_chart"`
	GitChart         ChartDataset   `json:"git_chart"`
}

type ChartDataset struct {
	Labels   []string             `json:"labels"`
	Datasets []ChartDatasetSeries `json:"datasets"`
}

type ChartDatasetSeries struct {
	Label string    `json:"label"`
	Data  []float64 `json:"data"`
	Color string    `json:"color"`
}

type WidgetConfig struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Title    string                 `json:"title"`
	Position Position               `json:"position"`
	Size     Size                   `json:"size"`
	Config   map[string]interface{} `json:"config"`
}

type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type Size struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// NewDashboard creates a new monitoring dashboard
func NewDashboard(config *DashboardConfig, integration *MonitoringIntegration) (*Dashboard, error) {
	dashboard := &Dashboard{
		config:      config,
		integration: integration,
		data:        &DashboardData{},
	}

	if !config.Enabled {
		return dashboard, nil
	}

	return dashboard, nil
}

// Start starts the dashboard web server
func (d *Dashboard) Start() error {
	if !d.config.Enabled {
		return nil
	}

	// Create HTTP server
	mux := http.NewServeMux()
	
	// API endpoints
	mux.HandleFunc("/api/dashboard", d.handleDashboardAPI)
	mux.HandleFunc("/api/stats", d.handleStatsAPI)
	mux.HandleFunc("/api/health", d.handleHealthAPI)
	mux.HandleFunc("/api/sessions", d.handleSessionsAPI)
	mux.HandleFunc("/api/metrics", d.handleMetricsAPI)
	
	// Static files (dashboard UI)
	mux.HandleFunc("/", d.handleDashboardUI)

	d.server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", d.config.Host, d.config.Port),
		Handler: mux,
	}

	// Start data refresh loop
	go d.refreshLoop()

	// Start server in goroutine
	go func() {
		if err := d.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Dashboard server error: %v\n", err)
		}
	}()

	d.running = true
	return nil
}

// Stop stops the dashboard
func (d *Dashboard) Stop() error {
	if !d.running {
		return nil
	}

	if d.server != nil {
		d.server.Close()
	}

	d.running = false
	return nil
}

// GetData returns current dashboard data
func (d *Dashboard) GetData() *DashboardData {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	// Return a copy
	data := *d.data
	return &data
}

// refreshLoop periodically refreshes dashboard data
func (d *Dashboard) refreshLoop() {
	ticker := time.NewTicker(d.config.RefreshInterval)
	defer ticker.Stop()

	// Initial refresh
	d.refreshData()

	for range ticker.C {
		d.refreshData()
	}
}

// refreshData updates all dashboard data
func (d *Dashboard) refreshData() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	stats := d.integration.GetStats()
	sessions := d.integration.GetSessions()

	// Update overview widget
	d.data.Overview = d.buildOverviewWidget(stats)
	
	// Update session widget
	d.data.Sessions = d.buildSessionWidget(stats, sessions)
	
	// Update performance widget
	d.data.Performance = d.buildPerformanceWidget(stats.SystemMetrics)
	
	// Update usage widget
	d.data.Usage = d.buildUsageWidget(stats.UsageStats)
	
	// Update system widget
	d.data.System = d.buildSystemWidget(stats.SystemMetrics)
	
	// Update git widget
	d.data.Git = d.buildGitWidget(stats.UsageStats)
	
	// Update error widget
	d.data.Errors = d.buildErrorWidget(stats.UsageStats)
	
	// Update health widget
	d.data.Health = d.buildHealthWidget(stats.Health, stats.SystemMetrics)
	
	// Update trends widget
	d.data.Trends = d.buildTrendWidget(stats.Trends)
	
	// Update alerts widget
	d.data.Alerts = d.buildAlertWidget(stats.SystemMetrics.Health.Alerts)
	
	// Update charts
	d.data.Charts = d.buildChartData(stats)

	d.data.LastUpdated = time.Now()
}

// HTTP handlers

func (d *Dashboard) handleDashboardAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	data := d.GetData()
	json.NewEncoder(w).Encode(data)
}

func (d *Dashboard) handleStatsAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	stats := d.integration.GetStats()
	json.NewEncoder(w).Encode(stats)
}

func (d *Dashboard) handleHealthAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	health := d.integration.GetStats().Health
	json.NewEncoder(w).Encode(health)
}

func (d *Dashboard) handleSessionsAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	sessions := d.integration.GetSessions()
	json.NewEncoder(w).Encode(sessions)
}

func (d *Dashboard) handleMetricsAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	metrics := d.integration.metrics.GetMetrics()
	json.NewEncoder(w).Encode(metrics)
}

func (d *Dashboard) handleDashboardUI(w http.ResponseWriter, r *http.Request) {
	// Serve static HTML dashboard
	html := d.generateDashboardHTML()
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// Widget builders

func (d *Dashboard) buildOverviewWidget(stats *MonitoringStats) OverviewWidget {
	topProgram := ""
	mostActiveRepo := ""
	
	if len(stats.Overview.TopPrograms) > 0 {
		topProgram = stats.Overview.TopPrograms[0].Program
	}
	if len(stats.Overview.TopRepositories) > 0 {
		mostActiveRepo = stats.Overview.TopRepositories[0].Repository
	}

	return OverviewWidget{
		TotalSessions:      stats.Overview.TotalSessions,
		ActiveSessions:     stats.Overview.ActiveSessions,
		TotalCommands:      stats.Overview.TotalCommands,
		CommandsToday:      0, // Would need time-based filtering
		AverageSessionTime: stats.Overview.AverageSessionTime,
		SystemUptime:       stats.Overview.SystemUptime,
		ErrorRate:          stats.Overview.ErrorRate,
		SuccessRate:        1.0 - stats.Overview.ErrorRate,
		TopProgram:         topProgram,
		MostActiveRepo:     mostActiveRepo,
	}
}

func (d *Dashboard) buildSessionWidget(stats *MonitoringStats, sessions map[string]*SessionContext) SessionWidget {
	activeSessions := make([]ActiveSessionInfo, 0, len(sessions))
	
	for _, session := range sessions {
		activeSessions = append(activeSessions, ActiveSessionInfo{
			ID:           session.ID,
			Name:         session.Name,
			Program:      session.Program,
			StartTime:    session.StartTime,
			Duration:     time.Since(session.StartTime),
			CommandCount: session.CommandCount,
			Repository:   session.Repository,
			Status:       "active",
		})
	}

	return SessionWidget{
		ActiveSessions:      activeSessions,
		RecentSessions:      []SessionSummary{}, // Would need historical data
		SessionsByProgram:   stats.SessionStats.SessionsByProgram,
		SessionDistribution: []TimeDistribution{},
		AverageMetrics:      SessionMetrics{
			AverageDuration:   stats.SessionStats.AverageSessionTime,
			AverageCommands:   stats.CommandStats.CommandsPerSession,
			SuccessRate:       1.0 - stats.Overview.ErrorRate,
			CommandsPerMinute: 0, // Would need calculation
		},
	}
}

func (d *Dashboard) buildPerformanceWidget(metrics *SystemMetrics) PerformanceWidget {
	cpuStatus := "good"
	if metrics.Performance.CPU.UsagePercent > 80 {
		cpuStatus = "warning"
	}
	if metrics.Performance.CPU.UsagePercent > 95 {
		cpuStatus = "critical"
	}

	memStatus := "good"
	if metrics.Performance.Memory.MemoryPercent > 80 {
		memStatus = "warning"
	}
	if metrics.Performance.Memory.MemoryPercent > 95 {
		memStatus = "critical"
	}

	return PerformanceWidget{
		CPU: CPUInfo{
			Usage:   metrics.Performance.CPU.UsagePercent,
			LoadAvg: metrics.Performance.CPU.LoadAverage,
			Cores:   metrics.System.CPUCount,
			Status:  cpuStatus,
		},
		Memory: MemoryInfo{
			Used:       metrics.Performance.Memory.Allocated,
			Total:      metrics.Performance.Memory.System,
			Percentage: metrics.Performance.Memory.MemoryPercent,
			Status:     memStatus,
		},
		Goroutines: GoroutineInfo{
			Count:  metrics.Performance.Goroutines.Count,
			Status: "good",
			Trend:  "stable",
		},
		GC: GCInfo{
			Collections: metrics.Performance.GC.NumGC,
			LastPause:   metrics.Performance.GC.LastPause,
			TotalPause:  metrics.Performance.GC.PauseTotal,
			Status:      "good",
		},
		ResponseTimes: ResponseInfo{
			Average: metrics.Performance.ResponseTimes.Average,
			P95:     metrics.Performance.ResponseTimes.P95,
			P99:     metrics.Performance.ResponseTimes.P99,
			Status:  "good",
		},
		PerformanceScore: metrics.Health.PerformanceScore,
		Alerts:           []string{},
	}
}

func (d *Dashboard) buildUsageWidget(stats *UsageStats) UsageWidget {
	programUsage := make([]ProgramUsageInfo, 0, len(stats.SessionStats.SessionsByProgram))
	total := stats.SessionStats.TotalSessions
	
	for program, count := range stats.SessionStats.SessionsByProgram {
		percentage := float64(count) / float64(total) * 100
		programUsage = append(programUsage, ProgramUsageInfo{
			Program:    program,
			Sessions:   count,
			Commands:   0, // Would need detailed tracking
			Percentage: percentage,
			Trend:      "stable",
		})
	}

	return UsageWidget{
		HourlyUsage:       []UsageDataPoint{},
		DailyUsage:        []UsageDataPoint{},
		ProgramUsage:      programUsage,
		CommandFrequency:  []CommandInfo{},
		UsageTrends:       TrendInfo{Direction: TrendStable},
		PeakHours:         []string{},
	}
}

func (d *Dashboard) buildSystemWidget(metrics *SystemMetrics) SystemWidget {
	return SystemWidget{
		OS:               metrics.System.OS,
		Architecture:     metrics.System.Architecture,
		GoVersion:        metrics.System.GoVersion,
		CPUCount:         metrics.System.CPUCount,
		ProcessID:        metrics.System.ProcessID,
		WorkingDirectory: metrics.System.WorkingDirectory,
		Uptime:           metrics.System.Uptime,
		Environment:      make(map[string]string),
	}
}

func (d *Dashboard) buildGitWidget(stats *UsageStats) GitWidget {
	repositories := make([]RepositoryActivity, 0, len(stats.GitStats.RepositoryActivity))
	
	for repo, activity := range stats.GitStats.RepositoryActivity {
		repositories = append(repositories, RepositoryActivity{
			Name:    repo,
			Commits: activity,
			Active:  true,
		})
	}

	return GitWidget{
		TotalCommits:       stats.GitStats.TotalCommits,
		TotalPushes:        stats.GitStats.TotalPushes,
		TotalPulls:         stats.GitStats.TotalPulls,
		BranchesCreated:    stats.GitStats.BranchesCreated,
		RepositoryActivity: repositories,
		RecentOperations:   []GitOperation{},
		CommitFrequency:    []TimeDistribution{},
	}
}

func (d *Dashboard) buildErrorWidget(stats *UsageStats) ErrorWidget {
	errorTypes := make([]ErrorTypeInfo, 0, len(stats.ErrorStats.ErrorsByType))
	total := stats.ErrorStats.TotalErrors
	
	for errorType, count := range stats.ErrorStats.ErrorsByType {
		percent := float64(count) / float64(total) * 100
		errorTypes = append(errorTypes, ErrorTypeInfo{
			Type:    errorType,
			Count:   count,
			Percent: percent,
			Trend:   "stable",
		})
	}

	return ErrorWidget{
		TotalErrors:  stats.ErrorStats.TotalErrors,
		ErrorRate:    stats.ErrorStats.ErrorRate,
		ErrorsByType: errorTypes,
		RecentErrors: []ErrorInfo{},
		ErrorTrends:  []TimeDistribution{},
		TopErrors:    []ErrorFrequency{},
	}
}

func (d *Dashboard) buildHealthWidget(health HealthOverview, metrics *SystemMetrics) HealthWidget {
	components := make([]ComponentHealthInfo, 0, len(health.ComponentStatus))
	
	for component, status := range health.ComponentStatus {
		score := 100.0
		if status == HealthWarning {
			score = 70.0
		} else if status == HealthCritical {
			score = 30.0
		}

		components = append(components, ComponentHealthInfo{
			Component: component,
			Status:    status,
			Score:     score,
			Message:   string(status),
		})
	}

	alerts := make([]AlertInfo, len(health.ActiveAlerts))
	for i, alert := range health.ActiveAlerts {
		alerts[i] = AlertInfo{
			ID:       alert.ID,
			Type:     alert.Type,
			Message:  alert.Message,
			Severity: alert.Severity,
			Time:     alert.Timestamp,
			Acknowledged: alert.Acknowledged,
		}
	}

	return HealthWidget{
		OverallStatus:   health.OverallStatus,
		HealthScore:     health.HealthScore,
		ComponentHealth: components,
		ActiveAlerts:    alerts,
		HealthHistory:   []HealthHistoryPoint{},
		Recommendations: health.Recommendations,
	}
}

func (d *Dashboard) buildTrendWidget(trends TrendAnalysis) TrendWidget {
	return TrendWidget{
		UsageTrend: TrendInfo{
			Direction:  trends.UsageGrowth.Direction,
			Percentage: trends.UsageGrowth.Percentage,
			Period:     trends.UsageGrowth.TimeRange,
			Confidence: trends.UsageGrowth.Confidence,
		},
		PerformanceTrend: TrendInfo{
			Direction:  trends.PerformanceTrend.Direction,
			Percentage: trends.PerformanceTrend.Percentage,
			Period:     trends.PerformanceTrend.TimeRange,
			Confidence: trends.PerformanceTrend.Confidence,
		},
		ErrorTrend: TrendInfo{
			Direction:  trends.ErrorTrend.Direction,
			Percentage: trends.ErrorTrend.Percentage,
			Period:     trends.ErrorTrend.TimeRange,
			Confidence: trends.ErrorTrend.Confidence,
		},
		PopularityTrends: []TrendInfo{},
		Predictions:      []Prediction{},
		GrowthMetrics: GrowthInfo{
			Daily:  trends.PredictedGrowth,
			Trend:  "stable",
		},
	}
}

func (d *Dashboard) buildAlertWidget(alerts []ActiveAlert) AlertWidget {
	alertInfos := make([]AlertInfo, len(alerts))
	for i, alert := range alerts {
		alertInfos[i] = AlertInfo{
			ID:       alert.ID,
			Type:     alert.Type,
			Message:  alert.Message,
			Severity: alert.Severity,
			Time:     alert.Timestamp,
			Acknowledged: alert.Acknowledged,
		}
	}

	return AlertWidget{
		ActiveAlerts:  alertInfos,
		RecentAlerts:  []AlertInfo{},
		AlertsByType:  []AlertTypeInfo{},
		AlertTrends:   []TimeDistribution{},
		AlertSettings: AlertSettings{Enabled: true},
	}
}

func (d *Dashboard) buildChartData(stats *MonitoringStats) ChartData {
	// Simplified chart data - in production this would have real time series data
	return ChartData{
		UsageChart: ChartDataset{
			Labels: []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"},
			Datasets: []ChartDatasetSeries{
				{
					Label: "Sessions",
					Data:  []float64{10, 15, 12, 18, 20, 8, 5},
					Color: "#3498db",
				},
			},
		},
		PerformanceChart: ChartDataset{
			Labels: []string{"CPU", "Memory", "Goroutines", "GC"},
			Datasets: []ChartDatasetSeries{
				{
					Label: "Usage %",
					Data:  []float64{45, 62, 35, 20},
					Color: "#2ecc71",
				},
			},
		},
		ErrorChart: ChartDataset{
			Labels: []string{"Today", "Yesterday", "2 days ago", "3 days ago"},
			Datasets: []ChartDatasetSeries{
				{
					Label: "Errors",
					Data:  []float64{2, 5, 1, 3},
					Color: "#e74c3c",
				},
			},
		},
	}
}

// generateDashboardHTML generates the dashboard HTML
func (d *Dashboard) generateDashboardHTML() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Claude Squad Monitoring Dashboard</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 0; padding: 20px; background: #f5f5f5; }
        .header { background: #2c3e50; color: white; padding: 20px; border-radius: 8px; margin-bottom: 20px; }
        .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; }
        .card { background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .metric { font-size: 2em; font-weight: bold; color: #3498db; }
        .label { color: #7f8c8d; font-size: 0.9em; }
        .status-good { color: #27ae60; }
        .status-warning { color: #f39c12; }
        .status-critical { color: #e74c3c; }
        .refresh { position: fixed; top: 20px; right: 20px; background: #3498db; color: white; border: none; padding: 10px 20px; border-radius: 4px; cursor: pointer; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Claude Squad Monitoring Dashboard</h1>
        <p>Real-time monitoring and analytics</p>
    </div>
    
    <button class="refresh" onclick="refreshData()">Refresh</button>
    
    <div class="grid" id="dashboard">
        <div class="card">
            <h3>Loading...</h3>
            <p>Please wait while data is being loaded.</p>
        </div>
    </div>

    <script>
        let data = {};
        
        async function refreshData() {
            try {
                const response = await fetch('/api/dashboard');
                data = await response.json();
                updateDashboard();
            } catch (error) {
                console.error('Failed to refresh data:', error);
            }
        }
        
        function updateDashboard() {
            const dashboard = document.getElementById('dashboard');
            dashboard.innerHTML = generateCards();
        }
        
        function generateCards() {
            if (!data.overview) return '<div class="card"><h3>No data available</h3></div>';
            
            return ` + "`" + `
                <div class="card">
                    <h3>Overview</h3>
                    <div class="metric">${data.overview.active_sessions}</div>
                    <div class="label">Active Sessions</div>
                    <div class="metric">${data.overview.total_commands}</div>
                    <div class="label">Total Commands</div>
                </div>
                
                <div class="card">
                    <h3>Performance</h3>
                    <div class="metric">${data.performance.memory.percentage.toFixed(1)}%</div>
                    <div class="label">Memory Usage</div>
                    <div class="metric">${data.performance.goroutines.count}</div>
                    <div class="label">Goroutines</div>
                </div>
                
                <div class="card">
                    <h3>System Health</h3>
                    <div class="metric status-${data.health.overall_status}">${data.health.overall_status}</div>
                    <div class="label">Overall Status</div>
                    <div class="metric">${data.health.health_score.toFixed(1)}</div>
                    <div class="label">Health Score</div>
                </div>
                
                <div class="card">
                    <h3>Git Activity</h3>
                    <div class="metric">${data.git.total_commits}</div>
                    <div class="label">Total Commits</div>
                    <div class="metric">${data.git.total_pushes}</div>
                    <div class="label">Total Pushes</div>
                </div>
                
                <div class="card">
                    <h3>Errors</h3>
                    <div class="metric status-${data.errors.total_errors > 0 ? 'warning' : 'good'}">${data.errors.total_errors}</div>
                    <div class="label">Total Errors</div>
                    <div class="metric">${(data.errors.error_rate * 100).toFixed(2)}%</div>
                    <div class="label">Error Rate</div>
                </div>
                
                <div class="card">
                    <h3>Last Updated</h3>
                    <div class="label">${new Date(data.last_updated).toLocaleString()}</div>
                </div>
            ` + "`" + `;
        }
        
        // Initial load
        refreshData();
        
        // Auto-refresh every 30 seconds
        setInterval(refreshData, 30000);
    </script>
</body>
</html>`;
}

// DefaultDashboardConfig returns default dashboard configuration
func DefaultDashboardConfig() *DashboardConfig {
	return &DashboardConfig{
		Enabled:         true,
		Port:            8080,
		Host:            "localhost",
		RefreshInterval: time.Second * 5,
		EnableAuth:      false,
		Theme:           "light",
		Layout:          "detailed",
		EnableRealtime:  true,
		MaxDataPoints:   1000,
		CustomWidgets:   []WidgetConfig{},
	}
}