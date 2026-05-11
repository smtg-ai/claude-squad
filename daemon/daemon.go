package daemon

import (
	"claude-squad/config"
	"claude-squad/log"
	"claude-squad/session"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// DaemonWorkspaceEnv is the env var the parent process sets to tell the daemon
// which workspace it's scoped to.
const DaemonWorkspaceEnv = "CS_DAEMON_WORKSPACE"

// RunDaemon runs the daemon process which iterates over sessions and runs AutoYes mode on them.
// If workspaceID is non-empty, the daemon only handles instances belonging to that workspace.
// It's expected that the main process kills the daemon when the main process starts.
func RunDaemon(cfg *config.Config, workspaceID string) error {
	log.InfoLog.Printf("starting daemon (workspace=%q)", workspaceID)
	state := config.LoadState()
	storage, err := session.NewStorage(state)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	allInstances, err := storage.LoadInstances()
	if err != nil {
		return fmt.Errorf("failed to load instacnes: %w", err)
	}
	var instances []*session.Instance
	for _, inst := range allInstances {
		if workspaceID != "" && inst.WorkspaceID != workspaceID {
			continue
		}
		// Assume AutoYes is true if the daemon is running.
		inst.AutoYes = true
		instances = append(instances, inst)
	}

	pollInterval := time.Duration(cfg.DaemonPollInterval) * time.Millisecond

	// If we get an error for a session, it's likely that we'll keep getting the error. Log every 30 seconds.
	everyN := log.NewEvery(60 * time.Second)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	stopCh := make(chan struct{})
	go func() {
		defer wg.Done()
		ticker := time.NewTimer(pollInterval)
		for {
			for _, instance := range instances {
				// We only store started instances, but check anyway.
				if instance.Started() && !instance.Paused() {
					if _, hasPrompt := instance.HasUpdated(); hasPrompt {
						instance.TapEnter()
						if err := instance.UpdateDiffStats(); err != nil {
							if everyN.ShouldLog() {
								log.WarningLog.Printf("could not update diff stats for %s: %v", instance.Title, err)
							}
						}
					}
				}
			}

			// Handle stop before ticker.
			select {
			case <-stopCh:
				return
			default:
			}

			<-ticker.C
			ticker.Reset(pollInterval)
		}
	}()

	// Notify on SIGINT (Ctrl+C) and SIGTERM. Save instances before
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	log.InfoLog.Printf("received signal %s", sig.String())

	// Stop the goroutine so we don't race.
	close(stopCh)
	wg.Wait()

	// Save back, merging our (workspace-filtered) slice into the full list so
	// we don't clobber instances owned by other workspaces' daemons.
	if err := saveMerged(storage, allInstances, instances); err != nil {
		log.ErrorLog.Printf("failed to save instances when terminating daemon: %v", err)
	}
	return nil
}

func saveMerged(storage *session.Storage, all, scoped []*session.Instance) error {
	byTitle := make(map[string]*session.Instance, len(scoped))
	for _, inst := range scoped {
		byTitle[inst.Title] = inst
	}
	out := make([]*session.Instance, len(all))
	for i, inst := range all {
		if updated, ok := byTitle[inst.Title]; ok {
			out[i] = updated
		} else {
			out[i] = inst
		}
	}
	return storage.SaveInstances(out)
}

// pidFilePath returns the PID file path for a given workspace. Empty workspaceID
// uses the legacy global location for back-compat.
func pidFilePath(workspaceID string) (string, error) {
	if workspaceID == "" {
		dir, err := config.GetConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "daemon.pid"), nil
	}
	reg := config.LoadWorkspaceRegistry()
	ws := reg.Get(workspaceID)
	if ws == nil {
		return "", fmt.Errorf("workspace %s not found", workspaceID)
	}
	wsDir, err := ws.Dir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(wsDir, "daemon.pid"), nil
}

// LaunchDaemon launches a daemon process scoped to the given workspace.
func LaunchDaemon(workspaceID string) error {
	// Find the claude squad binary.
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	cmd := exec.Command(execPath, "--daemon")
	// Pass workspace id to the child via env so RunDaemon can scope itself.
	cmd.Env = append(os.Environ(), DaemonWorkspaceEnv+"="+workspaceID)

	// Detach the process from the parent
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	// Set process group to prevent signals from propagating
	cmd.SysProcAttr = getSysProcAttr()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start child process: %w", err)
	}

	log.InfoLog.Printf("started daemon child process (workspace=%q) with PID: %d", workspaceID, cmd.Process.Pid)

	pidFile, err := pidFilePath(workspaceID)
	if err != nil {
		return err
	}
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	return nil
}

// StopDaemon stops the workspace's daemon if it exists. No error if the PID file is missing.
func StopDaemon(workspaceID string) error {
	pidFile, err := pidFilePath(workspaceID)
	if err != nil {
		return err
	}
	return stopAtPidFile(pidFile)
}

// StopLegacyDaemon stops a pre-workspace daemon (pid file at ~/.claude-squad/daemon.pid).
// Called once during upgrade so we don't leave the old daemon orphaned.
func StopLegacyDaemon() error {
	return StopDaemon("")
}

func stopAtPidFile(pidFile string) error {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read PID file: %w", err)
	}

	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return fmt.Errorf("invalid PID file format: %w", err)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find daemon process: %w", err)
	}

	if err := proc.Kill(); err != nil {
		return fmt.Errorf("failed to stop daemon process: %w", err)
	}

	if err := os.Remove(pidFile); err != nil {
		return fmt.Errorf("failed to remove PID file: %w", err)
	}

	log.InfoLog.Printf("daemon process (PID: %d) stopped successfully", pid)
	return nil
}
