package update

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"orzbob/log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var (
	// CurrentVersion is the current version of the application
	CurrentVersion string

	// GitHubRepo is the GitHub repository to check for updates
	GitHubRepo = "carnivoroustoad/orzbob"
)

// ReleaseInfo represents the GitHub API response for a release
type ReleaseInfo struct {
	TagName string    `json:"tag_name"`
	Name    string    `json:"name"`
	Assets  []Asset   `json:"assets"`
	URL     string    `json:"html_url"`
	Date    time.Time `json:"published_at"`
}

// Asset represents a release asset
type Asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
	Size        int    `json:"size"`
}

// CheckForUpdates checks if there is a newer version available
func CheckForUpdates() (*ReleaseInfo, bool, error) {
	// Don't log if InfoLog is nil (happens during initial check)
	if log.InfoLog != nil {
		log.InfoLog.Println("Checking for updates...")
	}
	
	latestRelease, err := getLatestRelease()
	if err != nil {
		return nil, false, fmt.Errorf("failed to get latest release: %w", err)
	}

	// Extract version from TagName by removing 'v' prefix
	latestVersion := strings.TrimPrefix(latestRelease.TagName, "v")
	
	// Compare versions
	hasUpdate := isNewerVersion(CurrentVersion, latestVersion)
	
	if hasUpdate {
		if log.InfoLog != nil {
			log.InfoLog.Printf("Found update: v%s -> v%s", CurrentVersion, latestVersion)
		}
		return latestRelease, true, nil
	}
	
	if log.InfoLog != nil {
		log.InfoLog.Printf("No updates found. Current version: v%s is latest.", CurrentVersion)
	}
	return nil, false, nil
}

// getLatestRelease fetches the latest release from GitHub API
func getLatestRelease() (*ReleaseInfo, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", GitHubRepo)
	
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from GitHub API: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned non-OK status: %s", resp.Status)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read API response: %w", err)
	}
	
	var release ReleaseInfo
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}
	
	return &release, nil
}

// isNewerVersion determines if available version is newer than current
func isNewerVersion(currentVersion, availableVersion string) bool {
	// Simple version comparison - can be replaced with semver for more robust handling
	return availableVersion != currentVersion
}

// DownloadAndInstall downloads and installs the latest version
func DownloadAndInstall(release *ReleaseInfo) error {
	log.InfoLog.Println("Starting update process...")
	
	// Get platform and architecture
	platform := runtime.GOOS
	architecture := runtime.GOARCH
	
	// Normalize architecture naming to match release asset naming
	if architecture == "amd64" {
		architecture = "amd64"
	} else if architecture == "arm64" {
		architecture = "arm64"
	}
	
	// Determine archive extension
	archiveExt := ".tar.gz"
	if platform == "windows" {
		archiveExt = ".zip"
	}
	
	// Extract version without 'v' prefix
	version := strings.TrimPrefix(release.TagName, "v")
	
	// Determine archive name using the same format as in install.sh
	archiveName := fmt.Sprintf("orzbob_%s_%s_%s%s", version, platform, architecture, archiveExt)
	
	// Find the correct asset
	var downloadURL string
	for _, asset := range release.Assets {
		if asset.Name == archiveName {
			downloadURL = asset.DownloadURL
			break
		}
	}
	
	if downloadURL == "" {
		return fmt.Errorf("could not find download URL for %s", archiveName)
	}
	
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "orzbob-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)
	
	// Download the archive
	archivePath := filepath.Join(tmpDir, archiveName)
	if err := downloadFile(downloadURL, archivePath); err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}
	
	// Extract the archive based on platform
	if platform == "windows" {
		// Use external unzip command
		cmd := exec.Command("unzip", archivePath, "-d", tmpDir)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to extract archive: %w", err)
		}
	} else {
		// Use external tar command
		cmd := exec.Command("tar", "xzf", archivePath, "-C", tmpDir)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to extract archive: %w", err)
		}
	}
	
	// Get executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	
	// Create backup of current executable
	backupPath := execPath + ".backup"
	if err := copyFile(execPath, backupPath); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	
	// Binary name with platform-specific extension
	binaryName := "orzbob"
	if platform == "windows" {
		binaryName += ".exe"
	}
	
	// Replace current executable with new one
	extractedBinaryPath := filepath.Join(tmpDir, binaryName)
	if err := copyFile(extractedBinaryPath, execPath); err != nil {
		// Restore from backup if update fails
		restoreErr := copyFile(backupPath, execPath)
		if restoreErr != nil {
			log.ErrorLog.Printf("Failed to restore from backup: %v", restoreErr)
		}
		return fmt.Errorf("failed to install update: %w", err)
	}
	
	// Set executable permissions
	if platform != "windows" {
		if err := os.Chmod(execPath, 0755); err != nil {
			return fmt.Errorf("failed to set executable permissions: %w", err)
		}
	}
	
	log.InfoLog.Printf("Successfully updated to version %s", release.TagName)
	return nil
}

// downloadFile downloads a file from URL to a local path
func downloadFile(url, destPath string) error {
	// Create output file
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()
	
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	// Check server response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}
	
	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()
	
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	
	_, err = io.Copy(dstFile, sourceFile)
	return err
}