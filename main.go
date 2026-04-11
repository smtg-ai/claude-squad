package main

import (
	"claude-squad/app"
	cmd2 "claude-squad/cmd"
	"claude-squad/config"
	"claude-squad/daemon"
	"claude-squad/log"
	"claude-squad/session"
	"claude-squad/session/git"
	"claude-squad/session/tmux"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	version     = "1.0.17"
	programFlag string
	autoYesFlag bool
	daemonFlag  bool
	rootCmd     = &cobra.Command{
		Use:   "claude-squad",
		Short: "Claude Squad - Manage multiple AI agents like Claude Code, Aider, Codex, and Amp.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			log.Initialize(daemonFlag)
			defer log.Close()

			if daemonFlag {
				cfg := config.LoadConfig()
				err := daemon.RunDaemon(cfg)
				log.ErrorLog.Printf("failed to start daemon %v", err)
				return err
			}

			// Check if we're in a git repository
			currentDir, err := filepath.Abs(".")
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}

			if !git.IsGitRepo(currentDir) {
				return fmt.Errorf("error: claude-squad must be run from within a git repository")
			}

			cfg := config.LoadConfig()

			// Program flag overrides config
			program := cfg.GetProgram()
			if programFlag != "" {
				program = programFlag
			}
			// AutoYes flag overrides config
			autoYes := cfg.AutoYes
			if autoYesFlag {
				autoYes = true
			}
			if autoYes {
				defer func() {
					if err := daemon.LaunchDaemon(); err != nil {
						log.ErrorLog.Printf("failed to launch daemon: %v", err)
					}
				}()
			}
			// Kill any daemon that's running.
			if err := daemon.StopDaemon(); err != nil {
				log.ErrorLog.Printf("failed to stop daemon: %v", err)
			}

			return app.Run(ctx, program, autoYes)
		},
	}

	resetCmd = &cobra.Command{
		Use:   "reset",
		Short: "Reset all stored instances",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Initialize(false)
			defer log.Close()

			state := config.LoadState()
			storage, err := session.NewStorage(state)
			if err != nil {
				return fmt.Errorf("failed to initialize storage: %w", err)
			}
			if err := storage.DeleteAllInstances(); err != nil {
				return fmt.Errorf("failed to reset storage: %w", err)
			}
			fmt.Println("Storage has been reset successfully")

			if err := tmux.CleanupSessions(cmd2.MakeExecutor()); err != nil {
				return fmt.Errorf("failed to cleanup tmux sessions: %w", err)
			}
			fmt.Println("Tmux sessions have been cleaned up")

			if err := git.CleanupWorktrees(); err != nil {
				return fmt.Errorf("failed to cleanup worktrees: %w", err)
			}
			fmt.Println("Worktrees have been cleaned up")

			// Kill any daemon that's running.
			if err := daemon.StopDaemon(); err != nil {
				return err
			}
			fmt.Println("daemon has been stopped")

			return nil
		},
	}

	debugCmd = &cobra.Command{
		Use:   "debug",
		Short: "Print debug information like config paths",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Initialize(false)
			defer log.Close()

			cfg := config.LoadConfig()

			configDir, err := config.GetConfigDir()
			if err != nil {
				return fmt.Errorf("failed to get config directory: %w", err)
			}
			configJson, _ := json.MarshalIndent(cfg, "", "  ")

			fmt.Printf("Config: %s\n%s\n", filepath.Join(configDir, config.ConfigFileName), configJson)

			return nil
		},
	}

	recoverCmd = &cobra.Command{
		Use:   "recover",
		Short: "Recover instances with dead tmux sessions",
		Long: `Recover instances whose tmux sessions died (e.g. after a system restart)
but whose git worktrees are still intact. For Claude programs, sessions are
restarted with --resume to pick up the previous conversation.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Initialize(false)
			defer log.Close()

			state := config.LoadState()
			storage, err := session.NewStorage(state)
			if err != nil {
				return fmt.Errorf("failed to initialize storage: %w", err)
			}

			instancesData, err := storage.LoadInstancesRaw()
			if err != nil {
				return fmt.Errorf("failed to load instances: %w", err)
			}

			if len(instancesData) == 0 {
				fmt.Println("No instances found.")
				return nil
			}

			// Find recoverable instances
			var recoverable []session.InstanceData
			var alive []session.InstanceData
			var dead []session.InstanceData

			for _, data := range instancesData {
				if data.Status == session.Paused {
					continue
				}
				if session.IsRecoverable(data) {
					recoverable = append(recoverable, data)
				} else {
					ts := tmux.NewTmuxSession(data.Title, data.Program)
					if ts.DoesSessionExist() {
						alive = append(alive, data)
					} else {
						dead = append(dead, data)
					}
				}
			}

			if len(alive) > 0 {
				fmt.Printf("Alive (%d):\n", len(alive))
				for _, d := range alive {
					fmt.Printf("  ✓ %s [%s]\n", d.Title, d.Branch)
				}
			}

			if len(dead) > 0 {
				fmt.Printf("Dead (worktree missing, cannot recover) (%d):\n", len(dead))
				for _, d := range dead {
					fmt.Printf("  ✗ %s [%s]\n", d.Title, d.Branch)
				}
			}

			if len(recoverable) == 0 {
				fmt.Println("\nNo recoverable instances found.")
				return nil
			}

			fmt.Printf("\nRecoverable (%d):\n", len(recoverable))
			for _, d := range recoverable {
				fmt.Printf("  ↻ %s [%s] %s\n", d.Title, d.Branch, d.Worktree.WorktreePath)
			}

			// Recover all instances
			fmt.Printf("\nRecovering %d instance(s)...\n", len(recoverable))

			// First, load all instances that are still alive (including paused)
			var allInstances []*session.Instance
			for _, data := range instancesData {
				isRecoverable := false
				for _, r := range recoverable {
					if r.Title == data.Title {
						isRecoverable = true
						break
					}
				}

				if isRecoverable {
					instance, err := session.RecoverInstance(data)
					if err != nil {
						fmt.Printf("  ✗ Failed to recover %s: %v\n", data.Title, err)
						continue
					}
					allInstances = append(allInstances, instance)
					fmt.Printf("  ✓ Recovered %s\n", data.Title)
				} else {
					// For alive/paused instances, load normally but handle errors gracefully
					instance, err := session.FromInstanceData(data)
					if err != nil {
						fmt.Printf("  ⚠ Skipping %s (load error: %v)\n", data.Title, err)
						continue
					}
					allInstances = append(allInstances, instance)
				}
			}

			// Save updated state
			if err := storage.SaveInstances(allInstances); err != nil {
				return fmt.Errorf("failed to save instances: %w", err)
			}

			fmt.Println("\nRecovery complete. Run 'cs' to manage your instances.")
			return nil
		},
	}

	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the version number of claude-squad",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("claude-squad version %s\n", version)
			fmt.Printf("https://github.com/smtg-ai/claude-squad/releases/tag/v%s\n", version)
		},
	}
)

func init() {
	rootCmd.Flags().StringVarP(&programFlag, "program", "p", "",
		"Program to run in new instances (e.g. 'aider --model ollama_chat/gemma3:1b')")
	rootCmd.Flags().BoolVarP(&autoYesFlag, "autoyes", "y", false,
		"[experimental] If enabled, all instances will automatically accept prompts")
	rootCmd.Flags().BoolVar(&daemonFlag, "daemon", false, "Run a program that loads all sessions"+
		" and runs autoyes mode on them.")

	// Hide the daemonFlag as it's only for internal use
	err := rootCmd.Flags().MarkHidden("daemon")
	if err != nil {
		panic(err)
	}

	rootCmd.AddCommand(debugCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(resetCmd)
	rootCmd.AddCommand(recoverCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
	}
}
