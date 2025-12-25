package concurrency

import (
	"context"
	"fmt"
	"log"
	"time"
)

// Example recovery actions

// TmuxRestartRecovery restarts stale tmux sessions
type TmuxRestartRecovery struct {
	sessionName string
}

func NewTmuxRestartRecovery(sessionName string) *TmuxRestartRecovery {
	return &TmuxRestartRecovery{sessionName: sessionName}
}

func (t *TmuxRestartRecovery) Execute(ctx context.Context) error {
	log.Printf("Attempting to restart tmux session: %s", t.sessionName)
	// Implementation would go here
	// For example: kill and recreate the session
	return nil
}

func (t *TmuxRestartRecovery) Description() string {
	return fmt.Sprintf("restart tmux session %s", t.sessionName)
}

// GitWorktreePruneRecovery prunes stale git worktrees
type GitWorktreePruneRecovery struct {
	repoPath string
}

func NewGitWorktreePruneRecovery(repoPath string) *GitWorktreePruneRecovery {
	return &GitWorktreePruneRecovery{repoPath: repoPath}
}

func (g *GitWorktreePruneRecovery) Execute(ctx context.Context) error {
	log.Printf("Attempting to prune git worktrees in: %s", g.repoPath)
	// Implementation would go here
	// For example: run git worktree prune
	return nil
}

func (g *GitWorktreePruneRecovery) Description() string {
	return "prune stale git worktrees"
}

// AgentRestartRecovery restarts unhealthy agent instances
type AgentRestartRecovery struct {
	instanceManager interface{} // Reference to instance manager
}

func NewAgentRestartRecovery(instanceManager interface{}) *AgentRestartRecovery {
	return &AgentRestartRecovery{instanceManager: instanceManager}
}

func (a *AgentRestartRecovery) Execute(ctx context.Context) error {
	log.Println("Attempting to restart unhealthy agent instances")
	// Implementation would go here
	// For example: identify and restart unhealthy instances
	return nil
}

func (a *AgentRestartRecovery) Description() string {
	return "restart unhealthy agent instances"
}

// AdvancedHealthMonitorExample demonstrates advanced usage of the health monitoring system
func AdvancedHealthMonitorExample() {
	// Create custom configuration
	config := HealthMonitorConfig{
		CheckInterval:   15 * time.Second,
		HistorySize:     200, // Keep 200 historical records per component
		MaxAlerts:       500,
		AlertThrottle:   2 * time.Minute,
		RecoveryEnabled: true, // Enable auto-recovery
	}

	// Create the health monitor
	monitor := NewHealthMonitor(config)

	// Register health checks for all components
	monitor.RegisterHealthCheck(NewTmuxHealthCheck("claudesquad_", 100))
	monitor.RegisterHealthCheck(NewGitHealthCheck("/workspace/repo"))
	monitor.RegisterHealthCheck(NewAgentHealthCheck(func() (int, int, error) {
		// In a real implementation, this would query your instance manager
		// Example: return instanceManager.GetHealthStats()
		total := 20
		healthy := 18
		return total, healthy, nil
	}))

	// Register recovery actions for automatic remediation
	monitor.RegisterRecoveryAction("tmux", NewTmuxRestartRecovery("claudesquad_session1"))
	monitor.RegisterRecoveryAction("git", NewGitWorktreePruneRecovery("/workspace/repo"))
	monitor.RegisterRecoveryAction("agents", NewAgentRestartRecovery(nil))

	// Register multiple alert handlers for different notification channels

	// Console logger
	monitor.RegisterAlertHandler(func(alert Alert) {
		log.Printf("[ALERT] %s | %s | %s | %s",
			alert.Timestamp.Format(time.RFC3339),
			alert.Component,
			alert.Status,
			alert.Message,
		)
	})

	// Critical alerts (for pager/SMS integration)
	monitor.RegisterAlertHandler(func(alert Alert) {
		if alert.Status == Unhealthy {
			// In production, this would send to PagerDuty, Slack, etc.
			log.Printf("[CRITICAL] Component %s is UNHEALTHY: %s",
				alert.Component,
				alert.Message,
			)
		}
	})

	// Metrics collector (for Prometheus/Grafana)
	monitor.RegisterAlertHandler(func(alert Alert) {
		// In production, this would update Prometheus metrics
		log.Printf("[METRICS] health_alert{component=%s,status=%s} 1",
			alert.Component,
			alert.Status,
		)
	})

	// Start the health monitoring system
	if err := monitor.Start(); err != nil {
		log.Fatalf("Failed to start health monitor: %v", err)
	}

	log.Println("Health monitoring started")

	// Simulate running for some time
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for i := 0; i < 5; i++ {
		<-ticker.C

		// Periodically check and report overall health
		status, results := monitor.GetHealth()
		log.Printf("=== Health Status Report ===")
		log.Printf("Overall Status: %s", status)

		for name, result := range results {
			log.Printf("  Component: %s", name)
			log.Printf("    Status: %s", result.Status)
			log.Printf("    Message: %s", result.Message)
			log.Printf("    Checked: %s", result.Timestamp.Format(time.RFC3339))

			// Print metadata
			for key, value := range result.Metadata {
				log.Printf("    %s: %v", key, value)
			}

			// Check trends
			improving, degrading := monitor.GetComponentTrend(name, 10)
			if improving {
				log.Printf("    Trend: IMPROVING")
			} else if degrading {
				log.Printf("    Trend: DEGRADING")
			}
		}

		log.Println("===========================")

		// Check recent alerts
		alerts := monitor.GetAlerts()
		if len(alerts) > 0 {
			log.Printf("Recent alerts: %d", len(alerts))
			for _, alert := range alerts {
				if !alert.Acknowledged {
					log.Printf("  [%s] %s: %s",
						alert.Status,
						alert.Component,
						alert.Message,
					)
				}
			}
		}
	}

	// Stop monitoring gracefully
	if err := monitor.Stop(); err != nil {
		log.Fatalf("Failed to stop health monitor: %v", err)
	}

	log.Println("Health monitoring stopped")
}

// HealthDashboardExample shows how to create a real-time health dashboard
func HealthDashboardExample() {
	config := DefaultHealthMonitorConfig()
	monitor := NewHealthMonitor(config)

	// Register checks
	monitor.RegisterHealthCheck(NewTmuxHealthCheck("claudesquad_", 50))
	monitor.RegisterHealthCheck(NewGitHealthCheck("/workspace"))
	monitor.RegisterHealthCheck(NewAgentHealthCheck(func() (int, int, error) {
		return 10, 9, nil
	}))

	if err := monitor.Start(); err != nil {
		log.Fatalf("Failed to start monitor: %v", err)
	}
	defer monitor.Stop()

	// Simulate a dashboard that updates every 5 seconds
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for i := 0; i < 12; i++ { // Run for 1 minute
		<-ticker.C

		// Clear screen (in a real terminal)
		fmt.Println("\n\n=== HEALTH DASHBOARD ===")
		fmt.Printf("Time: %s\n\n", time.Now().Format(time.RFC3339))

		status, results := monitor.GetHealth()

		// Overall status with color coding (simplified)
		statusSymbol := "✓"
		if status == Degraded {
			statusSymbol = "⚠"
		} else if status == Unhealthy {
			statusSymbol = "✗"
		}

		fmt.Printf("Overall Status: %s %s\n\n", statusSymbol, status)

		// Component details
		fmt.Println("Components:")
		for name, result := range results {
			symbol := "✓"
			if result.Status == Degraded {
				symbol = "⚠"
			} else if result.Status == Unhealthy {
				symbol = "✗"
			}

			fmt.Printf("  %s %-10s %s - %s\n",
				symbol,
				name,
				result.Status,
				result.Message,
			)

			// Show history sparkline (simplified)
			history := monitor.GetComponentHistory(name, 10)
			fmt.Print("    History: ")
			for _, h := range history {
				switch h.Status {
				case Healthy:
					fmt.Print("█")
				case Degraded:
					fmt.Print("▓")
				case Unhealthy:
					fmt.Print("░")
				default:
					fmt.Print("?")
				}
			}
			fmt.Println()
		}

		// Recent alerts
		alerts := monitor.GetAlerts()
		if len(alerts) > 0 {
			fmt.Printf("\nRecent Alerts (%d):\n", len(alerts))
			count := 0
			for i := len(alerts) - 1; i >= 0 && count < 5; i-- {
				alert := alerts[i]
				fmt.Printf("  [%s] %s: %s\n",
					alert.Timestamp.Format("15:04:05"),
					alert.Component,
					alert.Message,
				)
				count++
			}
		}

		fmt.Println("\n========================")
	}
}

// StreamingHealthExample shows how to stream health updates in real-time
func StreamingHealthExample() {
	config := DefaultHealthMonitorConfig()
	config.CheckInterval = 5 * time.Second

	monitor := NewHealthMonitor(config)

	// Register components
	monitor.RegisterHealthCheck(NewTmuxHealthCheck("claudesquad_", 50))
	monitor.RegisterHealthCheck(NewGitHealthCheck("/workspace"))

	// Create a channel for health updates
	healthUpdates := make(chan string, 100)

	// Register alert handler that sends to channel
	monitor.RegisterAlertHandler(func(alert Alert) {
		msg := fmt.Sprintf("[%s] %s: %s (%s)",
			alert.Timestamp.Format(time.RFC3339),
			alert.Component,
			alert.Status,
			alert.Message,
		)
		select {
		case healthUpdates <- msg:
		default:
			log.Println("Health update channel full, dropping message")
		}
	})

	if err := monitor.Start(); err != nil {
		log.Fatalf("Failed to start monitor: %v", err)
	}
	defer monitor.Stop()

	// Consumer goroutine (simulating a WebSocket or SSE endpoint)
	go func() {
		for msg := range healthUpdates {
			// In production, this would send to connected clients
			log.Printf("Broadcasting health update: %s", msg)
		}
	}()

	// Run for a while
	time.Sleep(30 * time.Second)
	close(healthUpdates)
}

// HealthCheckWithTimeoutExample demonstrates proper context usage
func HealthCheckWithTimeoutExample() {
	// Custom health check with timeout handling
	type SlowHealthCheck struct{}

	_ = &SlowHealthCheck{} // Example type for demonstration

	// This check respects context cancellation
	checkFunc := func(ctx context.Context) HealthCheckResult {
		result := HealthCheckResult{
			Timestamp: time.Now(),
			Metadata:  make(map[string]interface{}),
		}

		// Simulate slow operation
		select {
		case <-time.After(5 * time.Second):
			result.Status = Healthy
			result.Message = "slow check completed"
		case <-ctx.Done():
			result.Status = Degraded
			result.Message = "check timed out"
		}

		return result
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result := checkFunc(ctx)
	log.Printf("Health check result: %s - %s", result.Status, result.Message)
}
