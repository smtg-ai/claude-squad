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
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

var (
	version       = "1.0.17"
	programFlag   string
	autoYesFlag   bool
	daemonFlag    bool
	workspaceFlag string
	rootCmd       = &cobra.Command{
		Use:   "claude-squad",
		Short: "Claude Squad - Manage multiple AI agents like Claude Code, Aider, Codex, and Amp.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			log.Initialize(daemonFlag)
			defer log.Close()

			if daemonFlag {
				cfg := config.LoadConfig()
				wsID := os.Getenv(daemon.DaemonWorkspaceEnv)
				err := daemon.RunDaemon(cfg, wsID)
				log.ErrorLog.Printf("failed to start daemon %v", err)
				return err
			}

			reg := config.LoadWorkspaceRegistry()

			var workspace *config.Workspace
			switch {
			case workspaceFlag != "":
				workspace = reg.Get(workspaceFlag)
				if workspace == nil {
					workspace = reg.FindByName(workspaceFlag)
				}
				if workspace == nil {
					return fmt.Errorf("workspace not found: %s (use `cs workspace ls`)", workspaceFlag)
				}
				_ = reg.Touch(workspace.ID)
			default:
				currentDir, err := filepath.Abs(".")
				if err != nil {
					return fmt.Errorf("failed to get current directory: %w", err)
				}
				if git.IsGitRepo(currentDir) {
					workspace, err = resolveOrRegisterWorkspace(reg, currentDir)
					if err != nil {
						return fmt.Errorf("workspace auto-register: %w", err)
					}
				} else {
					// Not in a git repo — fall back to the most-recently-used
					// workspace if any exist. If none do, run with no active
					// workspace; the TUI surfaces a hint to add one with `A`.
					workspace = reg.MostRecentlyUsed()
				}
			}

			if err := migrateInstancesToWorkspaces(reg); err != nil {
				log.WarningLog.Printf("instance migration: %v", err)
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
					if err := daemon.LaunchDaemon(workspace.ID); err != nil {
						log.ErrorLog.Printf("failed to launch daemon: %v", err)
					}
				}()
			}
			// Kill this workspace's daemon if one is running, plus any legacy
			// pre-workspace daemon left over from before the upgrade.
			if err := daemon.StopDaemon(workspace.ID); err != nil {
				log.ErrorLog.Printf("failed to stop daemon: %v", err)
			}
			if err := daemon.StopLegacyDaemon(); err != nil {
				log.WarningLog.Printf("failed to stop legacy daemon: %v", err)
			}

			var wsID string
			if workspace != nil {
				wsID = workspace.ID
			}
			return app.Run(ctx, program, autoYes, wsID)
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

			// Kill all per-workspace daemons and any legacy daemon.
			reg := config.LoadWorkspaceRegistry()
			for _, w := range reg.Workspaces {
				if err := daemon.StopDaemon(w.ID); err != nil {
					log.WarningLog.Printf("failed to stop daemon for workspace %s: %v", w.ID, err)
				}
			}
			if err := daemon.StopLegacyDaemon(); err != nil {
				log.WarningLog.Printf("failed to stop legacy daemon: %v", err)
			}
			fmt.Println("daemons have been stopped")

			return nil
		},
	}

	debugCmd = &cobra.Command{
		Use:   "debug",
		Short: "Print debug information like config paths, registered workspaces, and the effective env for each profile in the resolved workspace.",
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

			reg := config.LoadWorkspaceRegistry()
			fmt.Printf("\nWorkspaces: %s\n", filepath.Join(configDir, config.WorkspacesFileName))
			for _, w := range reg.Workspaces {
				fmt.Printf("  %s\n", w.String())
			}

			currentDir, err := filepath.Abs(".")
			if err == nil && git.IsGitRepo(currentDir) {
				ws, err := resolveOrRegisterWorkspace(reg, currentDir)
				if err == nil {
					fmt.Printf("\nResolved workspace for cwd: %s (%s)\n", ws.DisplayName, ws.ID)
					wsDir, _ := ws.Dir()
					wtRoot, _ := ws.WorktreeRoot()
					fmt.Printf("  dir:           %s\n", wsDir)
					fmt.Printf("  worktree root: %s\n", wtRoot)
					if ws.Hooks.PostWorktree != "" {
						fmt.Printf("  post_worktree: %s\n", ws.Hooks.PostWorktree)
					}
					for _, p := range ws.Profiles {
						fmt.Printf("\n  Profile %q -> %s\n", p.Name, p.Program)
						env, err := ws.ResolveEnv(&p)
						if err != nil {
							fmt.Printf("    (env resolution failed: %v)\n", err)
							continue
						}
						for _, kv := range env {
							fmt.Printf("    %s\n", kv)
						}
					}
				}
			}

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

	workspaceCmd = &cobra.Command{
		Use:   "workspace",
		Short: "Manage claude-squad workspaces",
	}

	workspaceLsCmd = &cobra.Command{
		Use:   "ls",
		Short: "List registered workspaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Initialize(false)
			defer log.Close()
			reg := config.LoadWorkspaceRegistry()
			for _, w := range reg.Workspaces {
				fmt.Println(w.String())
			}
			return nil
		},
	}

	workspaceAddCmd = &cobra.Command{
		Use:   "add <path>",
		Short: "Register a git repo as a workspace (idempotent)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Initialize(false)
			defer log.Close()
			abs, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			reg := config.LoadWorkspaceRegistry()
			ws, err := resolveOrRegisterWorkspace(reg, abs)
			if err != nil {
				return err
			}
			fmt.Println(ws.String())
			return nil
		},
	}

	workspaceEditCmd = &cobra.Command{
		Use:   "edit",
		Short: "Open the workspace registry (workspaces.json) in $EDITOR",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Initialize(false)
			defer log.Close()
			dir, err := config.GetConfigDir()
			if err != nil {
				return err
			}
			path := filepath.Join(dir, config.WorkspacesFileName)
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}
			editCmd := exec.Command(editor, path)
			editCmd.Stdin = os.Stdin
			editCmd.Stdout = os.Stdout
			editCmd.Stderr = os.Stderr
			if err := editCmd.Run(); err != nil {
				return fmt.Errorf("editor exited with error: %w", err)
			}
			// Verify the file still parses; warn loudly if not so the user knows.
			if _, err := os.Stat(path); err == nil {
				if data, err := os.ReadFile(path); err == nil {
					var reg config.WorkspaceRegistry
					if err := json.Unmarshal(data, &reg); err != nil {
						return fmt.Errorf("workspaces.json no longer parses: %w (edits saved, but cs will load an empty registry until fixed)", err)
					}
				}
			}
			return nil
		},
	}

	workspaceRmCmd = &cobra.Command{
		Use:   "rm <id-or-name>",
		Short: "Remove a workspace from the registry (does not touch the repo or worktrees)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Initialize(false)
			defer log.Close()
			reg := config.LoadWorkspaceRegistry()
			ws := reg.Get(args[0])
			if ws == nil {
				ws = reg.FindByName(args[0])
			}
			if ws == nil {
				return fmt.Errorf("workspace not found: %s", args[0])
			}
			return reg.Remove(ws.ID)
		},
	}
)

// resolveOrRegisterWorkspace finds the workspace for the git repo containing
// dirOrRepo, registering it silently if it doesn't exist.
func resolveOrRegisterWorkspace(reg *config.WorkspaceRegistry, dirOrRepo string) (*config.Workspace, error) {
	root, err := git.FindGitRepoRoot(dirOrRepo)
	if err != nil {
		return nil, err
	}
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		canonical = root
	}
	return reg.EnsureWorkspace(canonical, git.FirstRemoteURL(canonical))
}

// migrateInstancesToWorkspaces backfills WorkspaceID on any existing instances
// in state.json by deriving a workspace from each instance's worktree repo path.
// Operates on the raw JSON so it runs before instances are loaded/started.
func migrateInstancesToWorkspaces(reg *config.WorkspaceRegistry) error {
	state := config.LoadState()
	raw := state.GetInstances()
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var data []session.InstanceData
	if err := json.Unmarshal(raw, &data); err != nil {
		return err
	}
	changed := false
	for i := range data {
		if data[i].WorkspaceID != "" {
			continue
		}
		repoPath := data[i].Worktree.RepoPath
		if repoPath == "" {
			continue
		}
		canonical, err := filepath.EvalSymlinks(repoPath)
		if err != nil {
			canonical = repoPath
		}
		remote := git.FirstRemoteURL(canonical)
		id := config.WorkspaceID(canonical, remote)
		if reg.Get(id) == nil {
			now := time.Now()
			_ = reg.Upsert(config.Workspace{
				ID:          id,
				DisplayName: filepath.Base(canonical),
				RepoPath:    canonical,
				RemoteURL:   remote,
				CreatedAt:   now,
				LastUsedAt:  now,
			})
		}
		data[i].WorkspaceID = id

		// Best-effort: rename any pre-existing tmux session from the legacy
		// (unprefixed) name to the new workspace-scoped name. Idempotent: only
		// fires when the legacy session exists and the new name is unused.
		oldName := tmux.SessionName(data[i].Title, "")
		newName := tmux.SessionName(data[i].Title, id)
		if oldName != newName {
			if exec.Command("tmux", "has-session", "-t", oldName).Run() == nil &&
				exec.Command("tmux", "has-session", "-t", newName).Run() != nil {
				_ = exec.Command("tmux", "rename-session", "-t", oldName, newName).Run()
			}
		}

		changed = true
	}
	if !changed {
		return nil
	}
	out, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return state.SaveInstances(out)
}

func init() {
	rootCmd.Flags().StringVarP(&programFlag, "program", "p", "",
		"Program to run in new instances (e.g. 'aider --model ollama_chat/gemma3:1b')")
	rootCmd.Flags().BoolVarP(&autoYesFlag, "autoyes", "y", false,
		"[experimental] If enabled, all instances will automatically accept prompts")
	rootCmd.Flags().BoolVar(&daemonFlag, "daemon", false, "Run a program that loads all sessions"+
		" and runs autoyes mode on them.")
	rootCmd.Flags().StringVar(&workspaceFlag, "workspace", "",
		"Workspace id or display name. If set, use that workspace instead of auto-resolving from the current directory.")

	// Hide the daemonFlag as it's only for internal use
	err := rootCmd.Flags().MarkHidden("daemon")
	if err != nil {
		panic(err)
	}

	rootCmd.AddCommand(debugCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(resetCmd)

	workspaceCmd.AddCommand(workspaceLsCmd)
	workspaceCmd.AddCommand(workspaceAddCmd)
	workspaceCmd.AddCommand(workspaceRmCmd)
	workspaceCmd.AddCommand(workspaceEditCmd)
	rootCmd.AddCommand(workspaceCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
	}
}
