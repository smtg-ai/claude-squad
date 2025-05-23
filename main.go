package main

import (
	"claude-squad/app"
	"claude-squad/commands"
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
	version          = "1.0.0"
	programFlag      string
	autoYesFlag      bool
	daemonFlag       bool
	syncFlag         bool
	pullMainFlag     bool
	syncSubmodFlag   bool
	autoResolveFlag  bool
	rootCmd     = &cobra.Command{
		Use:   "claude-squad [squad_name]",
		Short: "Claude Squad - A terminal-based session manager",
		Args:  cobra.ArbitraryArgs,
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
			program := cfg.DefaultProgram
			if programFlag != "" {
				program = programFlag
			}
			// AutoYes flag overrides config
			autoYes := cfg.AutoYes
			if autoYesFlag {
				autoYes = true
			}
			// Remove automatic daemon launch - always start in interactive mode
			// Kill any daemon that's running.
			if err := daemon.StopDaemon(); err != nil {
				log.ErrorLog.Printf("failed to stop daemon: %v", err)
			}

			// If a squad name is provided, store it for app.Run to use
			var squadName string
			if len(args) > 0 {
				squadName = args[0]
				log.InfoLog.Printf("Squad name provided: %s", squadName)
			} else if autoYes {
				// If autoYes is enabled but no squad specified, boot the default environment
				squadName = "CoreAgent"
				log.InfoLog.Printf("AutoYes mode enabled, booting default environment: %s", squadName)
			}

			return app.Run(ctx, program, autoYes, squadName)
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

			if err := tmux.CleanupSessions(); err != nil {
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

	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the version number of claude-squad",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("claude-squad version %s\n", version)
			fmt.Printf("https://github.com/smtg-ai/claude-squad/releases/tag/v%s\n", version)
		},
	}

	daemonCmd = &cobra.Command{
		Use:   "daemon",
		Short: "Start claude-squad in daemon mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Initialize(true)
			defer log.Close()
			
			cfg := config.LoadConfig()
			err := daemon.RunDaemon(cfg)
			log.ErrorLog.Printf("failed to start daemon %v", err)
			return err
		},
	}

	connectCmd = &cobra.Command{
		Use:   "connect",
		Short: "Connect to a running claude-squad daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			log.Initialize(false)
			defer log.Close()

			cfg := config.LoadConfig()
			
			// Program flag overrides config
			program := cfg.DefaultProgram
			if programFlag != "" {
				program = programFlag
			}
			// AutoYes flag overrides config  
			autoYes := cfg.AutoYes
			if autoYesFlag {
				autoYes = true
			}

			return app.Run(ctx, program, autoYes)
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
	rootCmd.AddCommand(daemonCmd)
	rootCmd.AddCommand(connectCmd)
	rootCmd.AddCommand(commands.SyncCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
	}
}
