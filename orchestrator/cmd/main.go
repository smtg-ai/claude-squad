package main

import (
	"claude-squad/orchestrator"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	orchestratorURL string
	outputFormat    string

	rootCmd = &cobra.Command{
		Use:   "orchestrator",
		Short: "Oxigraph-powered concurrent agent orchestrator CLI",
		Long: `Advanced concurrent agent orchestration system for Claude Squad.
Manages up to 10 concurrent agents with semantic task tracking using Oxigraph.`,
	}

	healthCmd = &cobra.Command{
		Use:   "health",
		Short: "Check orchestrator service health",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := orchestrator.NewClient(orchestratorURL)
			if err := client.Health(); err != nil {
				fmt.Printf("❌ Service unhealthy: %v\n", err)
				return err
			}
			fmt.Println("✅ Service is healthy")
			return nil
		},
	}

	submitCmd = &cobra.Command{
		Use:   "submit [description]",
		Short: "Submit a new task",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := orchestrator.NewClient(orchestratorURL)

			priority, _ := cmd.Flags().GetInt("priority")
			dependencies, _ := cmd.Flags().GetStringSlice("depends-on")

			task := &orchestrator.Task{
				Description:  strings.Join(args, " "),
				Priority:     priority,
				Dependencies: dependencies,
				Status:       orchestrator.StatusPending,
				CreatedAt:    time.Now().UTC().Format(time.RFC3339),
			}

			taskID, err := client.CreateTask(task)
			if err != nil {
				return fmt.Errorf("failed to create task: %w", err)
			}

			fmt.Printf("✅ Task created: %s\n", taskID)
			return nil
		},
	}

	listCmd = &cobra.Command{
		Use:   "list",
		Short: "List tasks by status",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := orchestrator.NewClient(orchestratorURL)

			status, _ := cmd.Flags().GetString("status")

			switch status {
			case "ready":
				tasks, err := client.GetReadyTasks(50)
				if err != nil {
					return err
				}
				fmt.Printf("📋 Ready Tasks (%d):\n", len(tasks))
				for _, task := range tasks {
					fmt.Printf("  • %s [Priority: %d] %s\n",
						task.ID, task.Priority, task.Description)
				}

			case "running":
				tasks, err := client.GetRunningTasks()
				if err != nil {
					return err
				}
				fmt.Printf("🏃 Running Tasks (%d):\n", len(tasks))
				for _, taskID := range tasks {
					fmt.Printf("  • %s\n", taskID)
				}

			default:
				return fmt.Errorf("unknown status: %s (use 'ready' or 'running')", status)
			}

			return nil
		},
	}

	analyticsCmd = &cobra.Command{
		Use:   "analytics",
		Short: "Show task analytics",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := orchestrator.NewClient(orchestratorURL)

			analytics, err := client.GetAnalytics()
			if err != nil {
				return err
			}

			fmt.Println("📊 Task Analytics")
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			fmt.Printf("Total Tasks:      %d\n", analytics.TotalTasks)
			fmt.Printf("Running:          %d/%d (%.1f%% utilization)\n",
				analytics.RunningCount,
				analytics.MaxConcurrent,
				float64(analytics.RunningCount)/float64(analytics.MaxConcurrent)*100)
			fmt.Printf("Available Slots:  %d\n", analytics.AvailableSlots)
			fmt.Println("")
			fmt.Println("Status Breakdown:")
			for status, count := range analytics.StatusCounts {
				icon := getStatusIcon(status)
				fmt.Printf("  %s %-10s %d\n", icon, status+":", count)
			}

			return nil
		},
	}

	chainCmd = &cobra.Command{
		Use:   "chain [task-id]",
		Short: "Show task dependency chain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := orchestrator.NewClient(orchestratorURL)

			chain, err := client.GetTaskChain(args[0])
			if err != nil {
				return err
			}

			fmt.Printf("🔗 Dependency Chain for %s\n", args[0])
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			for i, dep := range chain {
				icon := getStatusIcon(dep.Status)
				indent := strings.Repeat("  ", i)
				fmt.Printf("%s%s %s [%s]\n", indent, icon, dep.ID, dep.Status)
				fmt.Printf("%s   %s\n", indent, dep.Description)
			}

			return nil
		},
	}

	optimizeCmd = &cobra.Command{
		Use:   "optimize",
		Short: "Get optimized task distribution",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := orchestrator.NewClient(orchestratorURL)

			tasks, err := client.OptimizeDistribution()
			if err != nil {
				return err
			}

			fmt.Printf("🎯 Optimized Task Distribution (%d tasks)\n", len(tasks))
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			for i, taskID := range tasks {
				fmt.Printf("%d. %s\n", i+1, taskID)
			}

			return nil
		},
	}

	dashboardCmd = &cobra.Command{
		Use:   "dashboard",
		Short: "Open interactive TUI dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			return orchestrator.RunDashboard(context.Background(), orchestratorURL)
		},
	}

	exampleCmd = &cobra.Command{
		Use:   "example [type]",
		Short: "Run example workflows",
		Long:  "Run example workflows. Types: basic, advanced",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "basic":
				return orchestrator.ExampleUsage()
			case "advanced":
				return orchestrator.AdvancedExample()
			default:
				return fmt.Errorf("unknown example type: %s", args[0])
			}
		},
	}
)

func getStatusIcon(status string) string {
	switch status {
	case orchestrator.StatusPending:
		return "⏳"
	case orchestrator.StatusRunning:
		return "🏃"
	case orchestrator.StatusCompleted:
		return "✅"
	case orchestrator.StatusFailed:
		return "❌"
	default:
		return "❓"
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&orchestratorURL, "url", "u", "http://localhost:5000",
		"Orchestrator service URL")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "text",
		"Output format (text, json)")

	submitCmd.Flags().IntP("priority", "p", 5, "Task priority (0-10)")
	submitCmd.Flags().StringSliceP("depends-on", "d", []string{}, "Task dependencies")

	listCmd.Flags().StringP("status", "s", "ready", "Task status to filter by")

	rootCmd.AddCommand(healthCmd)
	rootCmd.AddCommand(submitCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(analyticsCmd)
	rootCmd.AddCommand(chainCmd)
	rootCmd.AddCommand(optimizeCmd)
	rootCmd.AddCommand(dashboardCmd)
	rootCmd.AddCommand(exampleCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
