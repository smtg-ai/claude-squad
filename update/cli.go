package update

import (
	"fmt"
	"orzbob/config"
	"orzbob/log"
	"os"
	"os/exec"
	"time"
	
	"github.com/spf13/cobra"
)

var (
	forceFlag bool
	
	// UpdateCmd is the command for checking and applying updates
	UpdateCmd = &cobra.Command{
		Use:   "update",
		Short: "Check for and apply updates",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Initialize(false)
			defer log.Close()
			
			fmt.Println("Checking for updates...")
			
			release, hasUpdate, err := CheckForUpdates()
			if err != nil {
				return fmt.Errorf("failed to check for updates: %w", err)
			}
			
			cfg := config.LoadConfig()
			config.UpdateLastUpdateCheck(cfg)
			
			if !hasUpdate {
				fmt.Printf("You're already running the latest version (v%s).\n", CurrentVersion)
				return nil
			}
			
			fmt.Printf("Update available: v%s → v%s\n", CurrentVersion, release.TagName[1:])
			fmt.Printf("Release URL: %s\n", release.URL)
			
			if forceFlag || cfg.AutoInstallUpdates {
				fmt.Println("Installing update...")
				if err := DownloadAndInstall(release); err != nil {
					return fmt.Errorf("failed to install update: %w", err)
				}
				fmt.Println("Update successfully installed. Please restart the application.")
				return nil
			}
			
			fmt.Print("Do you want to install this update? [y/N]: ")
			var response string
			fmt.Scanln(&response)
			
			if response == "y" || response == "Y" {
				fmt.Println("Installing update...")
				if err := DownloadAndInstall(release); err != nil {
					return fmt.Errorf("failed to install update: %w", err)
				}
				fmt.Println("Update successfully installed. Please restart the application.")
			} else {
				fmt.Println("Update skipped.")
			}
			
			return nil
		},
	}
	
	// AutoUpdateCmd is a hidden command for automated update checks
	AutoUpdateCmd = &cobra.Command{
		Use:    "auto-update",
		Short:  "Automatically check for updates (hidden command)",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.LoadConfig()
			
			if !cfg.EnableAutoUpdate {
				return nil
			}
			
			// If last check was within 24 hours, skip
			if cfg.LastUpdateCheck > 0 {
				lastCheck := time.Unix(cfg.LastUpdateCheck, 0)
				if time.Since(lastCheck) < 24*time.Hour {
					return nil
				}
			}
			
			// Update the last check timestamp regardless of outcome
			defer config.UpdateLastUpdateCheck(cfg)
			
			release, hasUpdate, err := CheckForUpdates()
			if err != nil {
				log.ErrorLog.Printf("Auto-update check failed: %v", err)
				return nil // Don't propagate auto-update errors
			}
			
			if !hasUpdate {
				return nil
			}
			
			// If auto-install is enabled, install the update
			if cfg.AutoInstallUpdates {
				if err := DownloadAndInstall(release); err != nil {
					log.ErrorLog.Printf("Auto-update installation failed: %v", err)
					return nil
				}
				
				// Restart the application after successful update
				executable, err := os.Executable()
				if err != nil {
					log.ErrorLog.Printf("Failed to get executable path: %v", err)
					return nil
				}
				
				// Execute the new binary with the same arguments
				cmd := exec.Command(executable, os.Args[1:]...)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				cmd.Stdin = os.Stdin
				
				if err := cmd.Start(); err != nil {
					log.ErrorLog.Printf("Failed to restart after update: %v", err)
					return nil
				}
				
				// Exit the current process
				os.Exit(0)
			} else {
				// Just notify the user about the update
				fmt.Printf("\nUpdate available: v%s → v%s\n", CurrentVersion, release.TagName[1:])
				fmt.Printf("Run 'orzbob update' to install the update.\n\n")
			}
			
			return nil
		},
	}
)

func init() {
	UpdateCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Force update without confirmation")
}