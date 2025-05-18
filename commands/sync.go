package commands

import (
	"claude-squad/config"
	"claude-squad/log"
	"claude-squad/session"
	"claude-squad/session/git"
	"claude-squad/ui"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	pullMainFlag     bool
	syncSubmodFlag   bool
	autoResolveFlag  bool
	allInstancesFlag bool
)

// SyncCmd is the command for manually synchronizing git repositories
var SyncCmd = &cobra.Command{
	Use:   "sync [instance-name...]",
	Short: "Sync git repositories with main and update submodules",
	Long: `Sync git repositories with main branch and update submodules.
You can specify which instances to sync, or use --all to sync all instances.
By default, it will sync from main branch and update submodules.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Initialize(false)
		defer log.Close()

		// Load config and state
		cfg := config.LoadConfig()
		state := config.LoadState()
		storage, err := session.NewStorage(state)
		if err != nil {
			return fmt.Errorf("failed to initialize storage: %w", err)
		}

		// Load all instances
		instances, err := storage.LoadInstances()
		if err != nil {
			return fmt.Errorf("failed to load instances: %w", err)
		}

		// Filter instances if specific ones were requested
		var targetInstances []*session.Instance
		if allInstancesFlag {
			targetInstances = instances
		} else if len(args) > 0 {
			// Filter instances by name
			instanceMap := make(map[string]*session.Instance)
			for _, instance := range instances {
				instanceMap[instance.Title] = instance
			}

			for _, name := range args {
				if instance, ok := instanceMap[name]; ok {
					targetInstances = append(targetInstances, instance)
				} else {
					log.WarningLog.Printf("instance '%s' not found, skipping", name)
				}
			}

			if len(targetInstances) == 0 {
				return fmt.Errorf("no matching instances found")
			}
		} else {
			// If no instances specified and --all not given, show usage
			return cmd.Help()
		}

		// Create sync options from flags or config defaults
		syncOptions := git.SyncOptions{
			PullFromMain:         pullMainFlag,
			UpdateSubmodules:     syncSubmodFlag,
			AutoResolveConflicts: autoResolveFlag,
			CommitMessage:        fmt.Sprintf("Manual sync at %s", time.Now().Format(time.RFC3339)),
		}

		// Perform synchronization
		fmt.Println(lipgloss.NewStyle().Bold(true).Render("🔄 Synchronizing repositories..."))
		
		results, err := ui.SyncInstances(targetInstances, syncOptions)
		if err != nil {
			return fmt.Errorf("sync failed: %w", err)
		}

		// Display results
		fmt.Println()
		successCount, failCount := 0, 0
		
		for i, result := range results {
			instance := targetInstances[i]
			prefix := "✅"
			if !result.Success {
				prefix = "❌"
				failCount++
			} else {
				successCount++
			}
			
			fmt.Printf("%s %s: %s\n", prefix, 
				lipgloss.NewStyle().Bold(true).Render(instance.Title), 
				result.Message)
		}
		
		fmt.Println()
		summaryStyle := lipgloss.NewStyle().Bold(true)
		if failCount > 0 {
			fmt.Println(summaryStyle.Foreground(lipgloss.Color("9")).
				Render(fmt.Sprintf("Sync completed with %d successful, %d failed", successCount, failCount)))
		} else {
			fmt.Println(summaryStyle.Foreground(lipgloss.Color("10")).
				Render(fmt.Sprintf("Sync completed successfully (%d instances)", successCount)))
		}

		return nil
	},
}

// init adds the sync command to the root command
func init() {
	SyncCmd.Flags().BoolVar(&pullMainFlag, "pull-main", true,
		"Pull and merge changes from the main branch")
	SyncCmd.Flags().BoolVar(&syncSubmodFlag, "update-submodules", true,
		"Update git submodules recursively")
	SyncCmd.Flags().BoolVar(&autoResolveFlag, "auto-resolve", false,
		"Automatically resolve conflicts (uses 'ours' strategy)")
	SyncCmd.Flags().BoolVarP(&allInstancesFlag, "all", "a", false,
		"Sync all instances")
}