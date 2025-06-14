package monitoring

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"
)

// MetricsCollector handles system and application metrics
type MetricsCollector struct {
	config      *MetricsConfig
	tracker     *UsageTracker
	mutex       sync.RWMutex
	metrics     *SystemMetrics
	history     []MetricsSnapshot
	alerts      *AlertManager
	stopChan    chan struct{}
	running     bool
}

// MetricsConfig represents metrics collection configuration
type MetricsConfig struct {
	Enabled              bool          `json:"enabled"`
	CollectionInterval   time.Duration `json:"collection_interval"`
	HistorySize          int           `json:"history_size"`
	AlertThresholds      AlertThresholds `json:"alert_thresholds"`
	EnableSystemMetrics  bool          `json:"enable_system_metrics"`
	EnableAppMetrics     bool          `json:"enable_app_metrics"`
	EnablePerformance    bool          `json:"enable_performance"`
	ExportInterval       time.Duration `json:"export_interval"`
	ExportFormat         string        `json:"export_format"` // json, csv, prometheus
	StorageDir           string        `json:"storage_dir"`
}

// SystemMetrics represents comprehensive system metrics
type SystemMetrics struct {
	Timestamp        time.Time         `json:"timestamp"`
	System          SystemInfo         `json:"system"`
	Performance     PerformanceMetrics `json:"performance"`
	Application     ApplicationMetrics `json:"application"`
	Usage           UsageMetrics       `json:"usage"`
	Health          HealthMetrics      `json:"health"`
	Trends          TrendMetrics       `json:"trends"`
}

// SystemInfo represents system information
type SystemInfo struct {
	OS               string `json:"os"`
	Architecture     string `json:"architecture"`
	CPUCount         int    `json:"cpu_count"`
	GoVersion        string `json:"go_version"`
	Uptime           time.Duration `json:"uptime"`
	ProcessID        int    `json:"process_id"`
	WorkingDirectory string `json:"working_directory"`
}

// PerformanceMetrics represents performance metrics
type PerformanceMetrics struct {
	Memory          MemoryMetrics     `json:"memory"`
	CPU             CPUMetrics        `json:"cpu"`
	Goroutines      GoroutineMetrics  `json:"goroutines"`
	GC              GCMetrics         `json:"gc"`
	Network         NetworkMetrics    `json:"network"`
	Disk            DiskMetrics       `json:"disk"`
	ResponseTimes   ResponseTimeMetrics `json:"response_times"`
}

// ApplicationMetrics represents application-specific metrics
type ApplicationMetrics struct {
	ActiveSessions    int64              `json:"active_sessions"`
	TotalSessions     int64              `json:"total_sessions"`
	CommandsExecuted  int64              `json:"commands_executed"`
	GitOperations     int64              `json:"git_operations"`
	ErrorCount        int64              `json:"error_count"`
	ConfigChanges     int64              `json:"config_changes"`
	SecurityEvents    int64              `json:"security_events"`
	SessionsByProgram map[string]int64   `json:"sessions_by_program"`
	ActivePrograms    []string           `json:"active_programs"`
}

// UsageMetrics represents usage patterns
type UsageMetrics struct {
	HourlyDistribution   map[string]int64 `json:"hourly_distribution"`
	DailyDistribution    map[string]int64 `json:"daily_distribution"`
	PeakUsageHour        string           `json:"peak_usage_hour"`
	EventsPerMinute      float64          `json:"events_per_minute"`
	CommandsPerSession   float64          `json:"commands_per_session"`
	AverageSessionLength time.Duration    `json:"average_session_length"`
	MostActiveRepository string           `json:"most_active_repository"`
}

// HealthMetrics represents system health indicators
type HealthMetrics struct {
	OverallHealth     HealthStatus      `json:"overall_health"`
	ComponentHealth   map[string]HealthStatus `json:"component_health"`
	ErrorRate         float64           `json:"error_rate"`
	SuccessRate       float64           `json:"success_rate"`
	PerformanceScore  float64           `json:"performance_score"`
	SecurityScore     float64           `json:"security_score"`
	Alerts            []ActiveAlert     `json:"alerts"`
}

// TrendMetrics represents trending data
type TrendMetrics struct {
	UsageTrend        TrendDirection    `json:"usage_trend"`
	PerformanceTrend  TrendDirection    `json:"performance_trend"`
	ErrorTrend        TrendDirection    `json:"error_trend"`
	GrowthRate        float64           `json:"growth_rate"`
	Predictions       map[string]float64 `json:"predictions"`
}

// Supporting metric types
type MemoryMetrics struct {
	Allocated      uint64  `json:"allocated"`
	TotalAlloc     uint64  `json:"total_alloc"`
	System         uint64  `json:"system"`
	HeapInUse      uint64  `json:"heap_in_use"`
	HeapIdle       uint64  `json:"heap_idle"`
	StackInUse     uint64  `json:"stack_in_use"`
	MemoryPercent  float64 `json:"memory_percent"`
}

type CPUMetrics struct {
	UsagePercent   float64       `json:"usage_percent"`
	LoadAverage    float64       `json:"load_average"`
	ProcessCPU     float64       `json:"process_cpu"`
	SystemCPU      float64       `json:"system_cpu"`
	IdleTime       time.Duration `json:"idle_time"`
}

type GoroutineMetrics struct {
	Count          int           `json:"count"`
	Running        int           `json:"running"`
	Waiting        int           `json:"waiting"`
	MaxSeen        int           `json:"max_seen"`
	CreationRate   float64       `json:"creation_rate"`
}

type GCMetrics struct {
	NumGC          uint32        `json:"num_gc"`
	PauseTotal     time.Duration `json:"pause_total"`
	LastPause      time.Duration `json:"last_pause"`
	PausePercentile95 time.Duration `json:"pause_percentile_95"`
	GCPercent      float64       `json:"gc_percent"`
	NextGC         uint64        `json:"next_gc"`
}

type NetworkMetrics struct {
	BytesSent      uint64  `json:"bytes_sent"`
	BytesReceived  uint64  `json:"bytes_received"`
	PacketsSent    uint64  `json:"packets_sent"`
	PacketsReceived uint64 `json:"packets_received"`
	ConnectionCount int    `json:"connection_count"`
	Bandwidth      float64 `json:"bandwidth"`
}

type DiskMetrics struct {
	UsedSpace      uint64  `json:"used_space"`
	FreeSpace      uint64  `json:"free_space"`
	TotalSpace     uint64  `json:"total_space"`
	UsagePercent   float64 `json:"usage_percent"`
	IOOperations   uint64  `json:"io_operations"`
	ReadThroughput float64 `json:"read_throughput"`
	WriteThroughput float64 `json:"write_throughput"`
}

type ResponseTimeMetrics struct {
	Average        time.Duration    `json:"average"`
	Median         time.Duration    `json:"median"`
	P95            time.Duration    `json:"p95"`
	P99            time.Duration    `json:"p99"`
	Min            time.Duration    `json:"min"`
	Max            time.Duration    `json:"max"`
	Distribution   []time.Duration  `json:"distribution"`
}

type AlertThresholds struct {
	CPUUsage           float64 `json:"cpu_usage"`
	MemoryUsage        float64 `json:"memory_usage"`
	DiskUsage          float64 `json:"disk_usage"`
	ErrorRate          float64 `json:"error_rate"`
	ResponseTime       time.Duration `json:"response_time"`
	GoroutineCount     int     `json:"goroutine_count"`
	SessionCount       int64   `json:"session_count"`
}

type HealthStatus string

const (
	HealthHealthy   HealthStatus = "healthy"
	HealthWarning   HealthStatus = "warning"
	HealthCritical  HealthStatus = "critical"
	HealthUnknown   HealthStatus = "unknown"
)

type TrendDirection string

const (
	TrendUp       TrendDirection = "up"
	TrendDown     TrendDirection = "down"
	TrendStable   TrendDirection = "stable"
	TrendVolatile TrendDirection = "volatile"
)

type ActiveAlert struct {
	ID          string       `json:"id"`
	Type        string       `json:"type"`
	Message     string       `json:"message"`
	Severity    HealthStatus `json:"severity"`
	Timestamp   time.Time    `json:"timestamp"`
	Acknowledged bool        `json:"acknowledged"`
}

type MetricsSnapshot struct {
	Timestamp time.Time      `json:"timestamp"`
	Metrics   SystemMetrics  `json:"metrics"`
}

// AlertManager handles metric-based alerting
type AlertManager struct {
	thresholds  AlertThresholds
	alerts      []ActiveAlert
	mutex       sync.RWMutex
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(config *MetricsConfig, tracker *UsageTracker) (*MetricsCollector, error) {
	collector := &MetricsCollector{
		config:   config,
		tracker:  tracker,
		metrics:  &SystemMetrics{},
		history:  make([]MetricsSnapshot, 0, config.HistorySize),
		alerts:   NewAlertManager(config.AlertThresholds),
		stopChan: make(chan struct{}),
	}

	if !config.Enabled {
		return collector, nil
	}

	// Create storage directory
	if err := os.MkdirAll(config.StorageDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create metrics storage directory: %w", err)
	}

	// Start collection loops
	go collector.collectionLoop()
	go collector.exportLoop()

	collector.running = true
	return collector, nil
}

// collectionLoop periodically collects metrics
func (mc *MetricsCollector) collectionLoop() {
	ticker := time.NewTicker(mc.config.CollectionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mc.collectMetrics()
		case <-mc.stopChan:
			return
		}
	}
}

// exportLoop periodically exports metrics
func (mc *MetricsCollector) exportLoop() {
	ticker := time.NewTicker(mc.config.ExportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mc.exportMetrics()
		case <-mc.stopChan:
			return
		}
	}
}

// collectMetrics collects all system and application metrics
func (mc *MetricsCollector) collectMetrics() {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	now := time.Now()
	mc.metrics.Timestamp = now

	// Collect system metrics
	if mc.config.EnableSystemMetrics {
		mc.collectSystemInfo()
	}

	// Collect performance metrics
	if mc.config.EnablePerformance {
		mc.collectPerformanceMetrics()
	}

	// Collect application metrics
	if mc.config.EnableAppMetrics {
		mc.collectApplicationMetrics()
	}

	// Collect usage metrics
	mc.collectUsageMetrics()

	// Calculate health metrics
	mc.calculateHealthMetrics()

	// Calculate trends
	mc.calculateTrends()

	// Check alerts
	mc.checkAlerts()

	// Add to history
	mc.addToHistory()
}

// collectSystemInfo collects basic system information
func (mc *MetricsCollector) collectSystemInfo() {
	wd, _ := os.Getwd()
	
	mc.metrics.System = SystemInfo{
		OS:               runtime.GOOS,
		Architecture:     runtime.GOARCH,
		CPUCount:         runtime.NumCPU(),
		GoVersion:        runtime.Version(),
		ProcessID:        os.Getpid(),
		WorkingDirectory: wd,
		Uptime:           time.Since(time.Now()), // Placeholder
	}
}

// collectPerformanceMetrics collects performance metrics
func (mc *MetricsCollector) collectPerformanceMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Memory metrics
	mc.metrics.Performance.Memory = MemoryMetrics{
		Allocated:     m.Alloc,
		TotalAlloc:    m.TotalAlloc,
		System:        m.Sys,
		HeapInUse:     m.HeapInuse,
		HeapIdle:      m.HeapIdle,
		StackInUse:    m.StackInuse,
		MemoryPercent: float64(m.Alloc) / float64(m.Sys) * 100,
	}

	// GC metrics
	mc.metrics.Performance.GC = GCMetrics{
		NumGC:     m.NumGC,
		PauseTotal: time.Duration(m.PauseTotalNs),
		NextGC:    m.NextGC,
		GCPercent: float64(m.GCCPUFraction) * 100,
	}

	if len(m.PauseNs) > 0 {
		mc.metrics.Performance.GC.LastPause = time.Duration(m.PauseNs[(m.NumGC+255)%256])
	}

	// Goroutine metrics
	mc.metrics.Performance.Goroutines = GoroutineMetrics{
		Count: runtime.NumGoroutine(),
	}

	// CPU metrics (simplified)
	mc.metrics.Performance.CPU = CPUMetrics{
		UsagePercent: 0, // Would need OS-specific implementation
		ProcessCPU:   0, // Would need OS-specific implementation
	}

	// Network and Disk metrics would require OS-specific implementations
	mc.metrics.Performance.Network = NetworkMetrics{}
	mc.metrics.Performance.Disk = DiskMetrics{}
	mc.metrics.Performance.ResponseTimes = ResponseTimeMetrics{}
}

// collectApplicationMetrics collects application-specific metrics
func (mc *MetricsCollector) collectApplicationMetrics() {
	if mc.tracker == nil {
		return
	}

	stats := mc.tracker.GetStats()
	sessions := mc.tracker.GetSessions()

	activeCount := int64(0)
	programs := make(map[string]bool)
	
	for _, session := range sessions {
		if session.Active {
			activeCount++
			if session.Program != "" {
				programs[session.Program] = true
			}
		}
	}

	activePrograms := make([]string, 0, len(programs))
	for program := range programs {
		activePrograms = append(activePrograms, program)
	}

	mc.metrics.Application = ApplicationMetrics{
		ActiveSessions:    activeCount,
		TotalSessions:     stats.SessionStats.TotalSessions,
		CommandsExecuted:  stats.CommandStats.TotalCommands,
		GitOperations:     stats.GitStats.TotalCommits + stats.GitStats.TotalPushes + stats.GitStats.TotalPulls,
		ErrorCount:        stats.ErrorStats.TotalErrors,
		SessionsByProgram: stats.SessionStats.SessionsByProgram,
		ActivePrograms:    activePrograms,
	}
}

// collectUsageMetrics collects usage pattern metrics
func (mc *MetricsCollector) collectUsageMetrics() {
	if mc.tracker == nil {
		return
	}

	stats := mc.tracker.GetStats()
	
	// Calculate events per minute
	timeSpan := stats.TimeRange.End.Sub(stats.TimeRange.Start)
	var eventsPerMinute float64
	if timeSpan.Minutes() > 0 {
		eventsPerMinute = float64(stats.TotalEvents) / timeSpan.Minutes()
	}

	// Find peak usage hour (simplified)
	peakHour := "unknown"
	maxEvents := int64(0)
	hourlyDist := make(map[string]int64)
	
	// This would be calculated from actual event timestamps
	currentHour := time.Now().Format("15")
	hourlyDist[currentHour] = stats.TotalEvents

	mc.metrics.Usage = UsageMetrics{
		HourlyDistribution:   hourlyDist,
		DailyDistribution:    make(map[string]int64),
		PeakUsageHour:        peakHour,
		EventsPerMinute:      eventsPerMinute,
		CommandsPerSession:   stats.CommandStats.CommandsPerSession,
		AverageSessionLength: stats.SessionStats.AverageSessionTime,
		MostActiveRepository: mc.findMostActiveRepo(stats.GitStats.RepositoryActivity),
	}
}

// calculateHealthMetrics calculates overall system health
func (mc *MetricsCollector) calculateHealthMetrics() {
	componentHealth := make(map[string]HealthStatus)
	
	// Memory health
	memPercent := mc.metrics.Performance.Memory.MemoryPercent
	if memPercent > 90 {
		componentHealth["memory"] = HealthCritical
	} else if memPercent > 70 {
		componentHealth["memory"] = HealthWarning
	} else {
		componentHealth["memory"] = HealthHealthy
	}

	// Goroutine health
	goroutineCount := mc.metrics.Performance.Goroutines.Count
	if goroutineCount > 10000 {
		componentHealth["goroutines"] = HealthCritical
	} else if goroutineCount > 1000 {
		componentHealth["goroutines"] = HealthWarning
	} else {
		componentHealth["goroutines"] = HealthHealthy
	}

	// Error rate health
	stats := mc.tracker.GetStats()
	errorRate := stats.ErrorStats.ErrorRate
	if errorRate > 0.1 {
		componentHealth["errors"] = HealthCritical
	} else if errorRate > 0.05 {
		componentHealth["errors"] = HealthWarning
	} else {
		componentHealth["errors"] = HealthHealthy
	}

	// Calculate overall health
	overallHealth := HealthHealthy
	for _, health := range componentHealth {
		if health == HealthCritical {
			overallHealth = HealthCritical
			break
		} else if health == HealthWarning && overallHealth != HealthCritical {
			overallHealth = HealthWarning
		}
	}

	// Calculate scores
	performanceScore := mc.calculatePerformanceScore()
	securityScore := mc.calculateSecurityScore()
	successRate := 1.0 - errorRate

	mc.metrics.Health = HealthMetrics{
		OverallHealth:    overallHealth,
		ComponentHealth:  componentHealth,
		ErrorRate:        errorRate,
		SuccessRate:      successRate,
		PerformanceScore: performanceScore,
		SecurityScore:    securityScore,
		Alerts:           mc.alerts.GetActiveAlerts(),
	}
}

// calculateTrends calculates trending metrics
func (mc *MetricsCollector) calculateTrends() {
	// Simple trend calculation based on recent history
	if len(mc.history) < 2 {
		mc.metrics.Trends = TrendMetrics{
			UsageTrend:       TrendStable,
			PerformanceTrend: TrendStable,
			ErrorTrend:       TrendStable,
			GrowthRate:       0,
			Predictions:      make(map[string]float64),
		}
		return
	}

	// Compare with previous metrics
	prev := mc.history[len(mc.history)-1].Metrics
	current := *mc.metrics

	// Usage trend
	usageTrend := TrendStable
	if current.Application.TotalSessions > prev.Application.TotalSessions {
		usageTrend = TrendUp
	} else if current.Application.TotalSessions < prev.Application.TotalSessions {
		usageTrend = TrendDown
	}

	// Performance trend (based on memory usage)
	perfTrend := TrendStable
	if current.Performance.Memory.MemoryPercent > prev.Performance.Memory.MemoryPercent+5 {
		perfTrend = TrendDown // Higher memory usage = worse performance
	} else if current.Performance.Memory.MemoryPercent < prev.Performance.Memory.MemoryPercent-5 {
		perfTrend = TrendUp
	}

	// Error trend
	errorTrend := TrendStable
	if current.Health.ErrorRate > prev.Health.ErrorRate {
		errorTrend = TrendUp
	} else if current.Health.ErrorRate < prev.Health.ErrorRate {
		errorTrend = TrendDown
	}

	mc.metrics.Trends = TrendMetrics{
		UsageTrend:       usageTrend,
		PerformanceTrend: perfTrend,
		ErrorTrend:       errorTrend,
		GrowthRate:       mc.calculateGrowthRate(),
		Predictions:      mc.generatePredictions(),
	}
}

// checkAlerts checks metrics against alert thresholds
func (mc *MetricsCollector) checkAlerts() {
	thresholds := mc.config.AlertThresholds

	// Check CPU usage
	if mc.metrics.Performance.CPU.UsagePercent > thresholds.CPUUsage {
		mc.alerts.TriggerAlert("cpu_usage", fmt.Sprintf("CPU usage %.1f%% exceeds threshold %.1f%%", 
			mc.metrics.Performance.CPU.UsagePercent, thresholds.CPUUsage), HealthWarning)
	}

	// Check memory usage
	if mc.metrics.Performance.Memory.MemoryPercent > thresholds.MemoryUsage {
		mc.alerts.TriggerAlert("memory_usage", fmt.Sprintf("Memory usage %.1f%% exceeds threshold %.1f%%", 
			mc.metrics.Performance.Memory.MemoryPercent, thresholds.MemoryUsage), HealthWarning)
	}

	// Check error rate
	if mc.metrics.Health.ErrorRate > thresholds.ErrorRate {
		mc.alerts.TriggerAlert("error_rate", fmt.Sprintf("Error rate %.3f exceeds threshold %.3f", 
			mc.metrics.Health.ErrorRate, thresholds.ErrorRate), HealthCritical)
	}

	// Check goroutine count
	if mc.metrics.Performance.Goroutines.Count > thresholds.GoroutineCount {
		mc.alerts.TriggerAlert("goroutine_count", fmt.Sprintf("Goroutine count %d exceeds threshold %d", 
			mc.metrics.Performance.Goroutines.Count, thresholds.GoroutineCount), HealthWarning)
	}
}

// addToHistory adds current metrics to history
func (mc *MetricsCollector) addToHistory() {
	snapshot := MetricsSnapshot{
		Timestamp: mc.metrics.Timestamp,
		Metrics:   *mc.metrics,
	}

	mc.history = append(mc.history, snapshot)

	// Maintain history size
	if len(mc.history) > mc.config.HistorySize {
		mc.history = mc.history[1:]
	}
}

// exportMetrics exports metrics to configured format
func (mc *MetricsCollector) exportMetrics() {
	if !mc.config.Enabled {
		return
	}

	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("metrics_%s.%s", timestamp, mc.config.ExportFormat)
	filepath := filepath.Join(mc.config.StorageDir, filename)

	switch mc.config.ExportFormat {
	case "json":
		mc.exportJSON(filepath)
	case "csv":
		mc.exportCSV(filepath)
	case "prometheus":
		mc.exportPrometheus(filepath)
	default:
		mc.exportJSON(filepath)
	}
}

// exportJSON exports metrics as JSON
func (mc *MetricsCollector) exportJSON(filepath string) {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()

	data, err := json.MarshalIndent(mc.metrics, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(filepath, data, 0644)
}

// exportCSV exports metrics as CSV (simplified)
func (mc *MetricsCollector) exportCSV(filepath string) {
	// Implementation would create CSV format
	// This is a placeholder
}

// exportPrometheus exports metrics in Prometheus format
func (mc *MetricsCollector) exportPrometheus(filepath string) {
	// Implementation would create Prometheus format
	// This is a placeholder
}

// GetMetrics returns current metrics
func (mc *MetricsCollector) GetMetrics() *SystemMetrics {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()

	// Return a copy
	metrics := *mc.metrics
	return &metrics
}

// GetHistory returns metrics history
func (mc *MetricsCollector) GetHistory() []MetricsSnapshot {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()

	history := make([]MetricsSnapshot, len(mc.history))
	copy(history, mc.history)
	return history
}

// Stop stops the metrics collector
func (mc *MetricsCollector) Stop() error {
	if !mc.running {
		return nil
	}

	close(mc.stopChan)
	mc.exportMetrics() // Final export
	mc.running = false
	return nil
}

// Helper functions

func (mc *MetricsCollector) findMostActiveRepo(repos map[string]int64) string {
	if len(repos) == 0 {
		return "unknown"
	}

	maxRepo := ""
	maxCount := int64(0)
	for repo, count := range repos {
		if count > maxCount {
			maxCount = count
			maxRepo = repo
		}
	}
	return maxRepo
}

func (mc *MetricsCollector) calculatePerformanceScore() float64 {
	// Simple performance scoring based on memory and goroutines
	memScore := math.Max(0, 100-mc.metrics.Performance.Memory.MemoryPercent)
	goroutineScore := math.Max(0, 100-float64(mc.metrics.Performance.Goroutines.Count)/100)
	
	return (memScore + goroutineScore) / 2
}

func (mc *MetricsCollector) calculateSecurityScore() float64 {
	// Placeholder security scoring
	// Would integrate with security framework
	return 85.0
}

func (mc *MetricsCollector) calculateGrowthRate() float64 {
	if len(mc.history) < 2 {
		return 0
	}

	prev := mc.history[len(mc.history)-1].Metrics.Application.TotalSessions
	current := mc.metrics.Application.TotalSessions
	
	if prev == 0 {
		return 0
	}

	return float64(current-prev) / float64(prev) * 100
}

func (mc *MetricsCollector) generatePredictions() map[string]float64 {
	predictions := make(map[string]float64)
	
	// Simple linear predictions based on trends
	if len(mc.history) >= 3 {
		// Predict memory usage
		memValues := make([]float64, len(mc.history))
		for i, snapshot := range mc.history {
			memValues[i] = snapshot.Metrics.Performance.Memory.MemoryPercent
		}
		predictions["memory_usage_1h"] = mc.linearPredict(memValues)
		
		// Predict session count
		sessionValues := make([]float64, len(mc.history))
		for i, snapshot := range mc.history {
			sessionValues[i] = float64(snapshot.Metrics.Application.ActiveSessions)
		}
		predictions["active_sessions_1h"] = mc.linearPredict(sessionValues)
	}

	return predictions
}

func (mc *MetricsCollector) linearPredict(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}

	// Simple linear regression for next point
	n := float64(len(values))
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0

	for i, y := range values {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	intercept := (sumY - slope*sumX) / n

	// Predict next point
	nextX := float64(len(values))
	return slope*nextX + intercept
}

// NewAlertManager creates a new alert manager
func NewAlertManager(thresholds AlertThresholds) *AlertManager {
	return &AlertManager{
		thresholds: thresholds,
		alerts:     make([]ActiveAlert, 0),
	}
}

// TriggerAlert triggers a new alert
func (am *AlertManager) TriggerAlert(alertType, message string, severity HealthStatus) {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	alert := ActiveAlert{
		ID:        fmt.Sprintf("alert_%d", time.Now().UnixNano()),
		Type:      alertType,
		Message:   message,
		Severity:  severity,
		Timestamp: time.Now(),
		Acknowledged: false,
	}

	am.alerts = append(am.alerts, alert)
}

// GetActiveAlerts returns all active alerts
func (am *AlertManager) GetActiveAlerts() []ActiveAlert {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	alerts := make([]ActiveAlert, len(am.alerts))
	copy(alerts, am.alerts)
	return alerts
}

// AcknowledgeAlert acknowledges an alert
func (am *AlertManager) AcknowledgeAlert(alertID string) {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	for i := range am.alerts {
		if am.alerts[i].ID == alertID {
			am.alerts[i].Acknowledged = true
			break
		}
	}
}

// ClearAcknowledgedAlerts removes acknowledged alerts
func (am *AlertManager) ClearAcknowledgedAlerts() {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	var activeAlerts []ActiveAlert
	for _, alert := range am.alerts {
		if !alert.Acknowledged {
			activeAlerts = append(activeAlerts, alert)
		}
	}
	am.alerts = activeAlerts
}

// DefaultMetricsConfig returns default metrics configuration
func DefaultMetricsConfig() *MetricsConfig {
	return &MetricsConfig{
		Enabled:              true,
		CollectionInterval:   time.Minute * 1,
		HistorySize:          100,
		AlertThresholds: AlertThresholds{
			CPUUsage:        80.0,
			MemoryUsage:     85.0,
			DiskUsage:       90.0,
			ErrorRate:       0.05,
			ResponseTime:    time.Second * 5,
			GoroutineCount:  5000,
			SessionCount:    100,
		},
		EnableSystemMetrics:  true,
		EnableAppMetrics:     true,
		EnablePerformance:    true,
		ExportInterval:       time.Hour * 1,
		ExportFormat:         "json",
		StorageDir:           "~/.claude-squad/monitoring/metrics",
	}
}