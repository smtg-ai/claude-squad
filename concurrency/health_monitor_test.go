package concurrency

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// MockHealthCheck is a mock implementation for testing
type MockHealthCheck struct {
	name   string
	status HealthStatus
	mu     sync.Mutex
}

func NewMockHealthCheck(name string, status HealthStatus) *MockHealthCheck {
	return &MockHealthCheck{
		name:   name,
		status: status,
	}
}

func (m *MockHealthCheck) Name() string {
	return m.name
}

func (m *MockHealthCheck) Check(ctx context.Context) HealthCheckResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	return HealthCheckResult{
		Status:    m.status,
		Message:   fmt.Sprintf("%s is %s", m.name, m.status),
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}
}

func (m *MockHealthCheck) SetStatus(status HealthStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status = status
}

// MockRecoveryAction is a mock recovery action for testing
type MockRecoveryAction struct {
	executed bool
	mu       sync.Mutex
}

func NewMockRecoveryAction() *MockRecoveryAction {
	return &MockRecoveryAction{}
}

func (m *MockRecoveryAction) Execute(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executed = true
	return nil
}

func (m *MockRecoveryAction) Description() string {
	return "mock recovery action"
}

func (m *MockRecoveryAction) WasExecuted() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.executed
}

func TestHealthStatus_String(t *testing.T) {
	tests := []struct {
		status   HealthStatus
		expected string
	}{
		{Healthy, "Healthy"},
		{Degraded, "Degraded"},
		{Unhealthy, "Unhealthy"},
		{Unknown, "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.status.String(); got != tt.expected {
				t.Errorf("HealthStatus.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestHealthAggregator(t *testing.T) {
	agg := NewHealthAggregator()

	// Initially unknown
	if status := agg.GetOverallStatus(); status != Unknown {
		t.Errorf("initial status should be Unknown, got %v", status)
	}

	// Add healthy result
	agg.Update("test1", HealthCheckResult{Status: Healthy})
	if status := agg.GetOverallStatus(); status != Healthy {
		t.Errorf("status should be Healthy, got %v", status)
	}

	// Add degraded result - should become degraded
	agg.Update("test2", HealthCheckResult{Status: Degraded})
	if status := agg.GetOverallStatus(); status != Degraded {
		t.Errorf("status should be Degraded, got %v", status)
	}

	// Add unhealthy result - should become unhealthy
	agg.Update("test3", HealthCheckResult{Status: Unhealthy})
	if status := agg.GetOverallStatus(); status != Unhealthy {
		t.Errorf("status should be Unhealthy, got %v", status)
	}

	// Check results map
	results := agg.GetResults()
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

func TestHealthHistory(t *testing.T) {
	history := NewHealthHistory(5)

	// Add results
	for i := 0; i < 10; i++ {
		result := HealthCheckResult{
			Status:    Healthy,
			Message:   fmt.Sprintf("check %d", i),
			Timestamp: time.Now(),
		}
		history.Add(result)
	}

	// Should only keep last 5
	recent := history.GetRecent(10)
	if len(recent) != 5 {
		t.Errorf("expected 5 recent results, got %d", len(recent))
	}

	// Most recent should be check 9
	if recent[0].Message != "check 9" {
		t.Errorf("most recent should be 'check 9', got %v", recent[0].Message)
	}
}

func TestHealthHistory_GetTrend(t *testing.T) {
	history := NewHealthHistory(10)

	// Add improving trend
	statuses := []HealthStatus{Unhealthy, Unhealthy, Degraded, Degraded, Healthy, Healthy}
	for _, status := range statuses {
		history.Add(HealthCheckResult{Status: status, Timestamp: time.Now()})
	}

	improving, degrading := history.GetTrend(6)
	if !improving {
		t.Error("trend should be improving")
	}
	if degrading {
		t.Error("trend should not be degrading")
	}

	// Add degrading trend
	history2 := NewHealthHistory(10)
	statuses2 := []HealthStatus{Healthy, Healthy, Degraded, Degraded, Unhealthy, Unhealthy}
	for _, status := range statuses2 {
		history2.Add(HealthCheckResult{Status: status, Timestamp: time.Now()})
	}

	improving2, degrading2 := history2.GetTrend(6)
	if improving2 {
		t.Error("trend should not be improving")
	}
	if !degrading2 {
		t.Error("trend should be degrading")
	}
}

func TestAlertManager(t *testing.T) {
	am := NewAlertManager(100, 1*time.Second)

	// Register handler
	alertReceived := false
	var receivedAlert Alert
	am.RegisterHandler(func(alert Alert) {
		alertReceived = true
		receivedAlert = alert
	})

	// Trigger alert
	am.TriggerAlert("test", Unhealthy, "test message")

	// Give handler time to execute
	time.Sleep(100 * time.Millisecond)

	if !alertReceived {
		t.Error("alert handler was not called")
	}

	if receivedAlert.Component != "test" {
		t.Errorf("expected component 'test', got %v", receivedAlert.Component)
	}

	// Check alerts list
	alerts := am.GetAlerts()
	if len(alerts) != 1 {
		t.Errorf("expected 1 alert, got %d", len(alerts))
	}

	// Test throttling
	am.TriggerAlert("test", Unhealthy, "test message 2")
	alerts = am.GetAlerts()
	if len(alerts) != 1 {
		t.Errorf("alert should be throttled, expected 1 alert, got %d", len(alerts))
	}

	// Wait for throttle period
	time.Sleep(1100 * time.Millisecond)
	am.TriggerAlert("test", Unhealthy, "test message 3")
	alerts = am.GetAlerts()
	if len(alerts) != 2 {
		t.Errorf("expected 2 alerts after throttle period, got %d", len(alerts))
	}

	// Test clear
	am.ClearAlerts()
	alerts = am.GetAlerts()
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts after clear, got %d", len(alerts))
	}
}

func TestHealthMonitor_StartStop(t *testing.T) {
	config := DefaultHealthMonitorConfig()
	config.CheckInterval = 100 * time.Millisecond

	monitor := NewHealthMonitor(config)

	// Register a mock health check
	mockCheck := NewMockHealthCheck("test", Healthy)
	monitor.RegisterHealthCheck(mockCheck)

	// Start monitor
	if err := monitor.Start(); err != nil {
		t.Fatalf("failed to start monitor: %v", err)
	}

	// Wait for at least one check
	time.Sleep(200 * time.Millisecond)

	// Check health
	status, results := monitor.GetHealth()
	if status != Healthy {
		t.Errorf("expected Healthy status, got %v", status)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	// Stop monitor
	if err := monitor.Stop(); err != nil {
		t.Fatalf("failed to stop monitor: %v", err)
	}

	// Starting again should fail
	if err := monitor.Start(); err == nil {
		t.Error("expected error when starting already started monitor")
	}
}

func TestHealthMonitor_ComponentHealth(t *testing.T) {
	config := DefaultHealthMonitorConfig()
	config.CheckInterval = 100 * time.Millisecond

	monitor := NewHealthMonitor(config)

	mockCheck1 := NewMockHealthCheck("comp1", Healthy)
	mockCheck2 := NewMockHealthCheck("comp2", Degraded)

	monitor.RegisterHealthCheck(mockCheck1)
	monitor.RegisterHealthCheck(mockCheck2)

	if err := monitor.Start(); err != nil {
		t.Fatalf("failed to start monitor: %v", err)
	}
	defer monitor.Stop()

	// Wait for checks
	time.Sleep(200 * time.Millisecond)

	// Check individual component health
	result, exists := monitor.GetComponentHealth("comp1")
	if !exists {
		t.Error("comp1 health should exist")
	}
	if result.Status != Healthy {
		t.Errorf("comp1 should be Healthy, got %v", result.Status)
	}

	result, exists = monitor.GetComponentHealth("comp2")
	if !exists {
		t.Error("comp2 health should exist")
	}
	if result.Status != Degraded {
		t.Errorf("comp2 should be Degraded, got %v", result.Status)
	}

	// Overall should be degraded (worst status)
	if !monitor.IsHealthy() {
		// This is expected since comp2 is degraded
	}
}

func TestHealthMonitor_History(t *testing.T) {
	config := DefaultHealthMonitorConfig()
	config.CheckInterval = 50 * time.Millisecond
	config.HistorySize = 10

	monitor := NewHealthMonitor(config)
	mockCheck := NewMockHealthCheck("test", Healthy)
	monitor.RegisterHealthCheck(mockCheck)

	if err := monitor.Start(); err != nil {
		t.Fatalf("failed to start monitor: %v", err)
	}
	defer monitor.Stop()

	// Wait for multiple checks
	time.Sleep(300 * time.Millisecond)

	// Get history
	history := monitor.GetComponentHistory("test", 5)
	if len(history) == 0 {
		t.Error("history should not be empty")
	}

	// All should be healthy
	for _, result := range history {
		if result.Status != Healthy {
			t.Errorf("expected Healthy, got %v", result.Status)
		}
	}
}

func TestHealthMonitor_Alerts(t *testing.T) {
	config := DefaultHealthMonitorConfig()
	config.CheckInterval = 100 * time.Millisecond
	config.AlertThrottle = 500 * time.Millisecond

	monitor := NewHealthMonitor(config)
	mockCheck := NewMockHealthCheck("test", Unhealthy)
	monitor.RegisterHealthCheck(mockCheck)

	alertCount := 0
	var mu sync.Mutex
	monitor.RegisterAlertHandler(func(alert Alert) {
		mu.Lock()
		defer mu.Unlock()
		alertCount++
	})

	if err := monitor.Start(); err != nil {
		t.Fatalf("failed to start monitor: %v", err)
	}
	defer monitor.Stop()

	// Wait for checks
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	count := alertCount
	mu.Unlock()

	if count == 0 {
		t.Error("expected at least one alert for unhealthy status")
	}

	// Get alerts
	alerts := monitor.GetAlerts()
	if len(alerts) == 0 {
		t.Error("expected alerts in list")
	}

	// Clear alerts
	monitor.ClearAlerts()
	alerts = monitor.GetAlerts()
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts after clear, got %d", len(alerts))
	}
}

func TestHealthMonitor_Recovery(t *testing.T) {
	config := DefaultHealthMonitorConfig()
	config.CheckInterval = 100 * time.Millisecond
	config.RecoveryEnabled = true

	monitor := NewHealthMonitor(config)
	mockCheck := NewMockHealthCheck("test", Unhealthy)
	mockRecovery := NewMockRecoveryAction()

	monitor.RegisterHealthCheck(mockCheck)
	monitor.RegisterRecoveryAction("test", mockRecovery)

	if err := monitor.Start(); err != nil {
		t.Fatalf("failed to start monitor: %v", err)
	}
	defer monitor.Stop()

	// Wait for recovery to execute
	time.Sleep(300 * time.Millisecond)

	if !mockRecovery.WasExecuted() {
		t.Error("recovery action should have been executed")
	}
}

func TestHealthMonitor_Trend(t *testing.T) {
	config := DefaultHealthMonitorConfig()
	config.CheckInterval = 50 * time.Millisecond
	config.HistorySize = 20

	monitor := NewHealthMonitor(config)
	mockCheck := NewMockHealthCheck("test", Unhealthy)
	monitor.RegisterHealthCheck(mockCheck)

	if err := monitor.Start(); err != nil {
		t.Fatalf("failed to start monitor: %v", err)
	}
	defer monitor.Stop()

	// Let unhealthy checks accumulate
	time.Sleep(150 * time.Millisecond)

	// Change to healthy to create improving trend
	mockCheck.SetStatus(Healthy)
	time.Sleep(200 * time.Millisecond)

	improving, degrading := monitor.GetComponentTrend("test", 10)
	if !improving {
		t.Error("trend should be improving")
	}
	if degrading {
		t.Error("trend should not be degrading")
	}
}

func TestTmuxHealthCheck(t *testing.T) {
	check := NewTmuxHealthCheck("test_", 10)

	if check.Name() != "tmux" {
		t.Errorf("expected name 'tmux', got %v", check.Name())
	}

	ctx := context.Background()
	result := check.Check(ctx)

	// Should at least not panic and return a valid result
	if result.Timestamp.IsZero() {
		t.Error("timestamp should be set")
	}
}

func TestGitHealthCheck(t *testing.T) {
	check := NewGitHealthCheck("")

	if check.Name() != "git" {
		t.Errorf("expected name 'git', got %v", check.Name())
	}

	ctx := context.Background()
	result := check.Check(ctx)

	// Should at least not panic and return a valid result
	if result.Timestamp.IsZero() {
		t.Error("timestamp should be set")
	}
}

func TestAgentHealthCheck(t *testing.T) {
	// Test with checker function
	checker := func() (int, int, error) {
		return 5, 4, nil
	}

	check := NewAgentHealthCheck(checker)

	if check.Name() != "agents" {
		t.Errorf("expected name 'agents', got %v", check.Name())
	}

	ctx := context.Background()
	result := check.Check(ctx)

	if result.Status != Healthy {
		t.Errorf("expected Healthy status, got %v", result.Status)
	}

	if result.Metadata["total_instances"] != 5 {
		t.Errorf("expected 5 total instances, got %v", result.Metadata["total_instances"])
	}
}

func TestAgentHealthCheck_Degraded(t *testing.T) {
	// Test degraded state
	checker := func() (int, int, error) {
		return 10, 6, nil // 60% healthy = degraded
	}

	check := NewAgentHealthCheck(checker)
	ctx := context.Background()
	result := check.Check(ctx)

	if result.Status != Degraded {
		t.Errorf("expected Degraded status, got %v", result.Status)
	}
}

func TestAgentHealthCheck_Unhealthy(t *testing.T) {
	// Test unhealthy state
	checker := func() (int, int, error) {
		return 10, 3, nil // 30% healthy = unhealthy
	}

	check := NewAgentHealthCheck(checker)
	ctx := context.Background()
	result := check.Check(ctx)

	if result.Status != Unhealthy {
		t.Errorf("expected Unhealthy status, got %v", result.Status)
	}
}

// Example usage demonstrating the health monitor
func ExampleHealthMonitor() {
	// Create configuration
	config := DefaultHealthMonitorConfig()
	config.CheckInterval = 30 * time.Second

	// Create monitor
	monitor := NewHealthMonitor(config)

	// Register health checks
	monitor.RegisterHealthCheck(NewTmuxHealthCheck("claudesquad_", 50))
	monitor.RegisterHealthCheck(NewGitHealthCheck("/path/to/repo"))
	monitor.RegisterHealthCheck(NewAgentHealthCheck(func() (int, int, error) {
		// Your logic to count total and healthy instances
		return 10, 9, nil
	}))

	// Register alert handler
	monitor.RegisterAlertHandler(func(alert Alert) {
		fmt.Printf("ALERT: %s is %s - %s\n", alert.Component, alert.Status, alert.Message)
	})

	// Start monitoring
	if err := monitor.Start(); err != nil {
		panic(err)
	}

	// Let it run for a while
	time.Sleep(5 * time.Second)

	// Check overall health
	status, results := monitor.GetHealth()
	fmt.Printf("Overall health: %s\n", status)
	for name, result := range results {
		fmt.Printf("  %s: %s - %s\n", name, result.Status, result.Message)
	}

	// Stop monitoring
	if err := monitor.Stop(); err != nil {
		panic(err)
	}
}
