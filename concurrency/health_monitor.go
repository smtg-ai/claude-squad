package concurrency

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

// HealthStatus represents the health state of a component or system
type HealthStatus int

const (
	// Unknown indicates health status cannot be determined
	Unknown HealthStatus = iota
	// Healthy indicates normal operation
	Healthy
	// Degraded indicates reduced functionality but still operational
	Degraded
	// Unhealthy indicates critical issues requiring attention
	Unhealthy
)

// String returns the string representation of HealthStatus
func (hs HealthStatus) String() string {
	switch hs {
	case Healthy:
		return "Healthy"
	case Degraded:
		return "Degraded"
	case Unhealthy:
		return "Unhealthy"
	case Unknown:
		return "Unknown"
	default:
		return "Unknown"
	}
}

// HealthCheckResult contains the result of a health check
type HealthCheckResult struct {
	// Status is the health status of the component
	Status HealthStatus
	// Message provides details about the health check
	Message string
	// Timestamp is when the check was performed
	Timestamp time.Time
	// Metadata contains additional context-specific information
	Metadata map[string]interface{}
}

// HealthCheck defines the interface for component health checks
type HealthCheck interface {
	// Check performs the health check and returns the result
	Check(ctx context.Context) HealthCheckResult
	// Name returns the name of the component being checked
	Name() string
}

// RecoveryAction defines the interface for automated recovery actions
type RecoveryAction interface {
	// Execute performs the recovery action
	Execute(ctx context.Context) error
	// Description returns a description of the recovery action
	Description() string
}

// TmuxHealthCheck checks the health of tmux sessions
type TmuxHealthCheck struct {
	sessionPrefix string
	maxSessions   int
}

// NewTmuxHealthCheck creates a new tmux health checker
func NewTmuxHealthCheck(sessionPrefix string, maxSessions int) *TmuxHealthCheck {
	return &TmuxHealthCheck{
		sessionPrefix: sessionPrefix,
		maxSessions:   maxSessions,
	}
}

// Name returns the name of this health check
func (t *TmuxHealthCheck) Name() string {
	return "tmux"
}

// Check performs the tmux health check
func (t *TmuxHealthCheck) Check(ctx context.Context) HealthCheckResult {
	result := HealthCheckResult{
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	// Check if tmux is installed
	if err := exec.CommandContext(ctx, "tmux", "-V").Run(); err != nil {
		result.Status = Unhealthy
		result.Message = "tmux is not installed or not accessible"
		return result
	}

	// List tmux sessions
	cmd := exec.CommandContext(ctx, "tmux", "ls")
	output, err := cmd.Output()
	if err != nil {
		// Exit code 1 means no sessions, which is ok
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			result.Status = Healthy
			result.Message = "tmux is healthy, no active sessions"
			result.Metadata["session_count"] = 0
			return result
		}
		result.Status = Degraded
		result.Message = fmt.Sprintf("failed to list tmux sessions: %v", err)
		return result
	}

	// Count sessions with our prefix
	sessionCount := 0
	lines := string(output)
	for i := 0; i < len(lines); i++ {
		if i == 0 || lines[i-1] == '\n' {
			if len(lines[i:]) >= len(t.sessionPrefix) &&
				lines[i:i+len(t.sessionPrefix)] == t.sessionPrefix {
				sessionCount++
			}
		}
	}

	result.Metadata["session_count"] = sessionCount

	if sessionCount > t.maxSessions {
		result.Status = Degraded
		result.Message = fmt.Sprintf("too many sessions: %d (max: %d)", sessionCount, t.maxSessions)
	} else {
		result.Status = Healthy
		result.Message = fmt.Sprintf("tmux is healthy with %d sessions", sessionCount)
	}

	return result
}

// GitHealthCheck checks the health of git operations
type GitHealthCheck struct {
	repoPath string
}

// NewGitHealthCheck creates a new git health checker
func NewGitHealthCheck(repoPath string) *GitHealthCheck {
	return &GitHealthCheck{
		repoPath: repoPath,
	}
}

// Name returns the name of this health check
func (g *GitHealthCheck) Name() string {
	return "git"
}

// Check performs the git health check
func (g *GitHealthCheck) Check(ctx context.Context) HealthCheckResult {
	result := HealthCheckResult{
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	// Check if git is installed
	if err := exec.CommandContext(ctx, "git", "--version").Run(); err != nil {
		result.Status = Unhealthy
		result.Message = "git is not installed or not accessible"
		return result
	}

	// Check if repository is accessible
	if g.repoPath != "" {
		cmd := exec.CommandContext(ctx, "git", "-C", g.repoPath, "status", "--porcelain")
		output, err := cmd.Output()
		if err != nil {
			result.Status = Degraded
			result.Message = fmt.Sprintf("failed to access repository: %v", err)
			return result
		}

		result.Metadata["has_changes"] = len(output) > 0

		// Check worktree status
		worktreeCmd := exec.CommandContext(ctx, "git", "-C", g.repoPath, "worktree", "list")
		worktreeOutput, err := worktreeCmd.Output()
		if err != nil {
			result.Status = Degraded
			result.Message = fmt.Sprintf("failed to list worktrees: %v", err)
			return result
		}

		// Count worktrees (simple count of newlines)
		worktreeCount := 0
		for _, c := range worktreeOutput {
			if c == '\n' {
				worktreeCount++
			}
		}
		result.Metadata["worktree_count"] = worktreeCount
	}

	result.Status = Healthy
	result.Message = "git is healthy"
	return result
}

// AgentHealthCheck checks the health of agent instances
type AgentHealthCheck struct {
	instanceChecker func() (int, int, error) // returns (total, healthy, error)
}

// NewAgentHealthCheck creates a new agent health checker
func NewAgentHealthCheck(instanceChecker func() (int, int, error)) *AgentHealthCheck {
	return &AgentHealthCheck{
		instanceChecker: instanceChecker,
	}
}

// Name returns the name of this health check
func (a *AgentHealthCheck) Name() string {
	return "agents"
}

// Check performs the agent health check
func (a *AgentHealthCheck) Check(ctx context.Context) HealthCheckResult {
	result := HealthCheckResult{
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	if a.instanceChecker == nil {
		result.Status = Unknown
		result.Message = "no instance checker configured"
		return result
	}

	total, healthy, err := a.instanceChecker()
	if err != nil {
		result.Status = Degraded
		result.Message = fmt.Sprintf("failed to check instances: %v", err)
		return result
	}

	result.Metadata["total_instances"] = total
	result.Metadata["healthy_instances"] = healthy

	if total == 0 {
		result.Status = Healthy
		result.Message = "no active agents"
		return result
	}

	healthyRatio := float64(healthy) / float64(total)
	if healthyRatio >= 0.9 {
		result.Status = Healthy
		result.Message = fmt.Sprintf("%d/%d agents healthy", healthy, total)
	} else if healthyRatio >= 0.5 {
		result.Status = Degraded
		result.Message = fmt.Sprintf("only %d/%d agents healthy", healthy, total)
	} else {
		result.Status = Unhealthy
		result.Message = fmt.Sprintf("critical: only %d/%d agents healthy", healthy, total)
	}

	return result
}

// HealthAggregator aggregates health checks to determine overall system health
type HealthAggregator struct {
	results map[string]HealthCheckResult
	mu      sync.RWMutex
}

// NewHealthAggregator creates a new health aggregator
func NewHealthAggregator() *HealthAggregator {
	return &HealthAggregator{
		results: make(map[string]HealthCheckResult),
	}
}

// Update updates the result for a specific component
func (ha *HealthAggregator) Update(name string, result HealthCheckResult) {
	ha.mu.Lock()
	defer ha.mu.Unlock()
	ha.results[name] = result
}

// GetOverallStatus returns the overall system health status
func (ha *HealthAggregator) GetOverallStatus() HealthStatus {
	ha.mu.RLock()
	defer ha.mu.RUnlock()

	if len(ha.results) == 0 {
		return Unknown
	}

	worstStatus := Healthy
	for _, result := range ha.results {
		if result.Status > worstStatus {
			worstStatus = result.Status
		}
	}

	return worstStatus
}

// GetResults returns a copy of all health check results
func (ha *HealthAggregator) GetResults() map[string]HealthCheckResult {
	ha.mu.RLock()
	defer ha.mu.RUnlock()

	results := make(map[string]HealthCheckResult, len(ha.results))
	for k, v := range ha.results {
		results[k] = v
	}
	return results
}

// Alert represents a health alert
type Alert struct {
	// Component is the name of the component
	Component string
	// Status is the health status that triggered the alert
	Status HealthStatus
	// Message describes the alert
	Message string
	// Timestamp is when the alert was created
	Timestamp time.Time
	// Acknowledged indicates if the alert has been acknowledged
	Acknowledged bool
}

// AlertHandler is a function that handles alerts
type AlertHandler func(alert Alert)

// AlertManager manages health alerts
type AlertManager struct {
	handlers       []AlertHandler
	alerts         []Alert
	mu             sync.RWMutex
	maxAlerts      int
	alertThrottle  map[string]time.Time
	throttlePeriod time.Duration
}

// NewAlertManager creates a new alert manager
func NewAlertManager(maxAlerts int, throttlePeriod time.Duration) *AlertManager {
	return &AlertManager{
		handlers:       make([]AlertHandler, 0),
		alerts:         make([]Alert, 0, maxAlerts),
		maxAlerts:      maxAlerts,
		alertThrottle:  make(map[string]time.Time),
		throttlePeriod: throttlePeriod,
	}
}

// RegisterHandler registers an alert handler
func (am *AlertManager) RegisterHandler(handler AlertHandler) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.handlers = append(am.handlers, handler)
}

// TriggerAlert creates and processes a new alert
func (am *AlertManager) TriggerAlert(component string, status HealthStatus, message string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Check throttle
	key := fmt.Sprintf("%s:%s", component, status)
	if lastAlert, exists := am.alertThrottle[key]; exists {
		if time.Since(lastAlert) < am.throttlePeriod {
			return // Throttled
		}
	}
	am.alertThrottle[key] = time.Now()

	alert := Alert{
		Component:    component,
		Status:       status,
		Message:      message,
		Timestamp:    time.Now(),
		Acknowledged: false,
	}

	// Add to alerts list (with circular buffer behavior)
	if len(am.alerts) >= am.maxAlerts {
		am.alerts = am.alerts[1:]
	}
	am.alerts = append(am.alerts, alert)

	// Notify handlers
	for _, handler := range am.handlers {
		go handler(alert)
	}
}

// GetAlerts returns all alerts
func (am *AlertManager) GetAlerts() []Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	alerts := make([]Alert, len(am.alerts))
	copy(alerts, am.alerts)
	return alerts
}

// ClearAlerts removes all alerts
func (am *AlertManager) ClearAlerts() {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.alerts = am.alerts[:0]
}

// HealthHistory maintains a circular buffer of health check results
type HealthHistory struct {
	buffer []HealthCheckResult
	size   int
	index  int
	count  int
	mu     sync.RWMutex
}

// NewHealthHistory creates a new health history with the specified buffer size
func NewHealthHistory(size int) *HealthHistory {
	return &HealthHistory{
		buffer: make([]HealthCheckResult, size),
		size:   size,
		index:  0,
		count:  0,
	}
}

// Add adds a health check result to the history
func (hh *HealthHistory) Add(result HealthCheckResult) {
	hh.mu.Lock()
	defer hh.mu.Unlock()

	hh.buffer[hh.index] = result
	hh.index = (hh.index + 1) % hh.size
	if hh.count < hh.size {
		hh.count++
	}
}

// GetRecent returns the most recent n health check results (or all if n > count)
func (hh *HealthHistory) GetRecent(n int) []HealthCheckResult {
	hh.mu.RLock()
	defer hh.mu.RUnlock()

	if n > hh.count {
		n = hh.count
	}

	results := make([]HealthCheckResult, n)
	for i := 0; i < n; i++ {
		idx := (hh.index - 1 - i + hh.size) % hh.size
		results[i] = hh.buffer[idx]
	}

	return results
}

// GetTrend analyzes the trend in health status over recent history
func (hh *HealthHistory) GetTrend(samples int) (improving bool, degrading bool) {
	recent := hh.GetRecent(samples)
	if len(recent) < 2 {
		return false, false
	}

	// Count status transitions
	betterCount := 0
	worseCount := 0

	for i := 0; i < len(recent)-1; i++ {
		if recent[i].Status < recent[i+1].Status {
			betterCount++
		} else if recent[i].Status > recent[i+1].Status {
			worseCount++
		}
	}

	improving = betterCount > worseCount && betterCount > len(recent)/3
	degrading = worseCount > betterCount && worseCount > len(recent)/3

	return improving, degrading
}

// HealthMonitorConfig contains configuration for the health monitor
type HealthMonitorConfig struct {
	// CheckInterval is how often to perform health checks
	CheckInterval time.Duration
	// HistorySize is the size of the circular buffer for each component
	HistorySize int
	// MaxAlerts is the maximum number of alerts to keep
	MaxAlerts int
	// AlertThrottle is the minimum time between alerts for the same component
	AlertThrottle time.Duration
	// RecoveryEnabled enables automatic recovery actions
	RecoveryEnabled bool
}

// DefaultHealthMonitorConfig returns a default configuration
func DefaultHealthMonitorConfig() HealthMonitorConfig {
	return HealthMonitorConfig{
		CheckInterval:   30 * time.Second,
		HistorySize:     100,
		MaxAlerts:       1000,
		AlertThrottle:   5 * time.Minute,
		RecoveryEnabled: false,
	}
}

// HealthMonitor monitors system health and triggers alerts
type HealthMonitor struct {
	config     HealthMonitorConfig
	checks     map[string]HealthCheck
	history    map[string]*HealthHistory
	aggregator *HealthAggregator
	alertMgr   *AlertManager
	recoveries map[string]RecoveryAction
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	started    bool
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor(config HealthMonitorConfig) *HealthMonitor {
	return &HealthMonitor{
		config:     config,
		checks:     make(map[string]HealthCheck),
		history:    make(map[string]*HealthHistory),
		aggregator: NewHealthAggregator(),
		alertMgr:   NewAlertManager(config.MaxAlerts, config.AlertThrottle),
		recoveries: make(map[string]RecoveryAction),
		started:    false,
	}
}

// RegisterHealthCheck registers a health check
func (hm *HealthMonitor) RegisterHealthCheck(check HealthCheck) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	name := check.Name()
	hm.checks[name] = check
	hm.history[name] = NewHealthHistory(hm.config.HistorySize)
}

// RegisterRecoveryAction registers a recovery action for a component
func (hm *HealthMonitor) RegisterRecoveryAction(component string, action RecoveryAction) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.recoveries[component] = action
}

// RegisterAlertHandler registers an alert handler
func (hm *HealthMonitor) RegisterAlertHandler(handler AlertHandler) {
	hm.alertMgr.RegisterHandler(handler)
}

// Start begins health monitoring
func (hm *HealthMonitor) Start() error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if hm.started {
		return fmt.Errorf("health monitor already started")
	}

	hm.ctx, hm.cancel = context.WithCancel(context.Background())
	hm.started = true

	// Start monitoring goroutines for each health check
	for name, check := range hm.checks {
		hm.wg.Add(1)
		go hm.monitorComponent(name, check)
	}

	return nil
}

// monitorComponent runs health checks for a specific component
func (hm *HealthMonitor) monitorComponent(name string, check HealthCheck) {
	defer hm.wg.Done()

	ticker := time.NewTicker(hm.config.CheckInterval)
	defer ticker.Stop()

	// Perform initial check immediately
	hm.performCheck(name, check)

	for {
		select {
		case <-hm.ctx.Done():
			return
		case <-ticker.C:
			hm.performCheck(name, check)
		}
	}
}

// performCheck executes a health check and processes the result
func (hm *HealthMonitor) performCheck(name string, check HealthCheck) {
	// Create a timeout context for the health check
	ctx, cancel := context.WithTimeout(hm.ctx, 10*time.Second)
	defer cancel()

	result := check.Check(ctx)

	// Update aggregator
	hm.aggregator.Update(name, result)

	// Add to history
	hm.mu.RLock()
	history := hm.history[name]
	hm.mu.RUnlock()

	if history != nil {
		history.Add(result)
	}

	// Trigger alerts for degraded or unhealthy status
	if result.Status == Degraded || result.Status == Unhealthy {
		hm.alertMgr.TriggerAlert(name, result.Status, result.Message)

		// Attempt recovery if enabled and an action is registered
		if hm.config.RecoveryEnabled {
			hm.mu.RLock()
			recovery, exists := hm.recoveries[name]
			hm.mu.RUnlock()

			if exists && result.Status == Unhealthy {
				go hm.attemptRecovery(name, recovery)
			}
		}
	}
}

// attemptRecovery attempts to execute a recovery action
func (hm *HealthMonitor) attemptRecovery(component string, action RecoveryAction) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := action.Execute(ctx); err != nil {
		hm.alertMgr.TriggerAlert(
			component,
			Unhealthy,
			fmt.Sprintf("recovery failed: %v", err),
		)
	} else {
		hm.alertMgr.TriggerAlert(
			component,
			Degraded,
			fmt.Sprintf("recovery action executed: %s", action.Description()),
		)
	}
}

// Stop stops the health monitor
func (hm *HealthMonitor) Stop() error {
	hm.mu.Lock()
	if !hm.started {
		hm.mu.Unlock()
		return fmt.Errorf("health monitor not started")
	}
	hm.started = false
	hm.mu.Unlock()

	// Cancel context to stop all goroutines
	if hm.cancel != nil {
		hm.cancel()
	}

	// Wait for all goroutines to finish
	hm.wg.Wait()

	return nil
}

// GetHealth returns the current overall health status and details
func (hm *HealthMonitor) GetHealth() (HealthStatus, map[string]HealthCheckResult) {
	status := hm.aggregator.GetOverallStatus()
	results := hm.aggregator.GetResults()
	return status, results
}

// GetComponentHealth returns the health status for a specific component
func (hm *HealthMonitor) GetComponentHealth(name string) (HealthCheckResult, bool) {
	results := hm.aggregator.GetResults()
	result, exists := results[name]
	return result, exists
}

// GetComponentHistory returns the health history for a specific component
func (hm *HealthMonitor) GetComponentHistory(name string, samples int) []HealthCheckResult {
	hm.mu.RLock()
	history, exists := hm.history[name]
	hm.mu.RUnlock()

	if !exists {
		return nil
	}

	return history.GetRecent(samples)
}

// GetComponentTrend returns the health trend for a specific component
func (hm *HealthMonitor) GetComponentTrend(name string, samples int) (improving bool, degrading bool) {
	hm.mu.RLock()
	history, exists := hm.history[name]
	hm.mu.RUnlock()

	if !exists {
		return false, false
	}

	return history.GetTrend(samples)
}

// GetAlerts returns all active alerts
func (hm *HealthMonitor) GetAlerts() []Alert {
	return hm.alertMgr.GetAlerts()
}

// ClearAlerts clears all alerts
func (hm *HealthMonitor) ClearAlerts() {
	hm.alertMgr.ClearAlerts()
}

// IsHealthy returns true if the overall system is healthy
func (hm *HealthMonitor) IsHealthy() bool {
	status, _ := hm.GetHealth()
	return status == Healthy
}
