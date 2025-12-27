// Package agent5 implements per-agent sandboxed work directories with poka-yoke (mistake-proof) isolation.
//
// Poka-yoke design principle: Make undeclared I/O unrepresentable, not just unlikely.
// All file operations must go through the Workspace API, which enforces declared allowlists.
package agent5

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	// ErrUndeclaredRead is returned when attempting to read a file not in InputFiles allowlist
	ErrUndeclaredRead = errors.New("poka-yoke violation: undeclared read operation")

	// ErrUndeclaredWrite is returned when attempting to write a file not in OutputFiles allowlist
	ErrUndeclaredWrite = errors.New("poka-yoke violation: undeclared write operation")

	// ErrPathTraversal is returned when a path attempts to escape the workspace
	ErrPathTraversal = errors.New("security violation: path traversal detected")

	// ErrWorkspaceNotFound is returned when workspace doesn't exist
	ErrWorkspaceNotFound = errors.New("workspace not found")
)

// WorkspaceConfig declares the I/O contract for an agent workspace.
// All file operations must be declared upfront in these allowlists.
type WorkspaceConfig struct {
	// InputFiles: Paths that this workspace is allowed to read from
	// Paths are relative to workspace base directory
	InputFiles []string `json:"input_files"`

	// OutputFiles: Paths that this workspace is allowed to write to
	// Paths are relative to workspace base directory
	OutputFiles []string `json:"output_files"`

	// AllowTempFiles: If true, allows writing to a temp/ subdirectory
	// without explicit declaration (useful for intermediate artifacts)
	AllowTempFiles bool `json:"allow_temp_files"`
}

// Workspace represents an isolated work directory with enforced I/O constraints.
//
// Design invariant: All I/O operations MUST go through IsolatedRead/IsolatedWrite.
// Direct filesystem access bypasses the poka-yoke guarantees.
type Workspace struct {
	// ID uniquely identifies this workspace
	ID string

	// BaseDir is the absolute path to the workspace root
	BaseDir string

	// config holds the declared I/O contract
	config WorkspaceConfig

	// allowedIn provides O(1) lookup for allowed input paths
	allowedIn map[string]bool

	// allowedOut provides O(1) lookup for allowed output paths
	allowedOut map[string]bool

	// mu protects concurrent access to allowlist maps
	mu sync.RWMutex

	// metrics track isolation overhead
	metrics WorkspaceMetrics
}

// WorkspaceMetrics tracks performance of isolation operations
type WorkspaceMetrics struct {
	ReadCount      int64
	WriteCount     int64
	ReadDenied     int64
	WriteDenied    int64
	AvgReadLatency time.Duration
	AvgWriteLatency time.Duration
	mu             sync.Mutex
}

// CreateWorkspace initializes a new isolated workspace with the given configuration.
//
// O: agentID, config (declared I/O allowlists)
// A = μ(O): Create workspace directory, initialize allowlist maps
// H: No undeclared I/O is possible through this workspace
//
// Returns:
//   - *Workspace: Initialized workspace with enforced isolation
//   - error: If workspace creation fails
func CreateWorkspace(agentID string, config WorkspaceConfig) (*Workspace, error) {
	if agentID == "" {
		return nil, errors.New("agentID cannot be empty")
	}

	// Create workspace base directory
	baseDir := filepath.Join(os.TempDir(), "kgc-workspaces", agentID)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspace directory: %w", err)
	}

	// Initialize allowlist maps for O(1) lookup
	allowedIn := make(map[string]bool, len(config.InputFiles))
	for _, path := range config.InputFiles {
		// Normalize path to prevent duplicates
		normalized := filepath.Clean(path)
		allowedIn[normalized] = true
	}

	allowedOut := make(map[string]bool, len(config.OutputFiles))
	for _, path := range config.OutputFiles {
		normalized := filepath.Clean(path)
		allowedOut[normalized] = true
	}

	// Create temp directory if allowed
	if config.AllowTempFiles {
		tempDir := filepath.Join(baseDir, "temp")
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create temp directory: %w", err)
		}
	}

	ws := &Workspace{
		ID:         agentID,
		BaseDir:    baseDir,
		config:     config,
		allowedIn:  allowedIn,
		allowedOut: allowedOut,
	}

	return ws, nil
}

// IsolatedRead reads a file from the workspace, enforcing input allowlist.
//
// Poka-yoke guarantee: If path is not in InputFiles, operation is REJECTED.
//
// Parameters:
//   - path: Relative path within workspace (must be in InputFiles allowlist)
//
// Returns:
//   - []byte: File contents
//   - error: ErrUndeclaredRead if path not in allowlist, or other I/O errors
func (w *Workspace) IsolatedRead(path string) ([]byte, error) {
	start := time.Now()
	defer func() {
		w.metrics.mu.Lock()
		w.metrics.ReadCount++
		elapsed := time.Since(start)
		// Exponential moving average
		if w.metrics.AvgReadLatency == 0 {
			w.metrics.AvgReadLatency = elapsed
		} else {
			w.metrics.AvgReadLatency = (w.metrics.AvgReadLatency*9 + elapsed) / 10
		}
		w.metrics.mu.Unlock()
	}()

	// Normalize and validate path
	cleanPath := filepath.Clean(path)
	if err := w.validatePath(cleanPath); err != nil {
		w.metrics.mu.Lock()
		w.metrics.ReadDenied++
		w.metrics.mu.Unlock()
		return nil, err
	}

	// Check allowlist (poka-yoke enforcement)
	w.mu.RLock()
	allowed := w.allowedIn[cleanPath]
	w.mu.RUnlock()

	if !allowed {
		w.metrics.mu.Lock()
		w.metrics.ReadDenied++
		w.metrics.mu.Unlock()
		return nil, fmt.Errorf("%w: path=%s", ErrUndeclaredRead, cleanPath)
	}

	// Construct absolute path
	absPath := filepath.Join(w.BaseDir, cleanPath)

	// Read file
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read failed: %w", err)
	}

	return data, nil
}

// IsolatedWrite writes data to a file in the workspace, enforcing output allowlist.
//
// Poka-yoke guarantee: If path is not in OutputFiles (or temp/ if AllowTempFiles), operation is REJECTED.
//
// Parameters:
//   - path: Relative path within workspace
//   - data: Data to write
//
// Returns:
//   - error: ErrUndeclaredWrite if path not in allowlist, or other I/O errors
func (w *Workspace) IsolatedWrite(path string, data []byte) error {
	start := time.Now()
	defer func() {
		w.metrics.mu.Lock()
		w.metrics.WriteCount++
		elapsed := time.Since(start)
		if w.metrics.AvgWriteLatency == 0 {
			w.metrics.AvgWriteLatency = elapsed
		} else {
			w.metrics.AvgWriteLatency = (w.metrics.AvgWriteLatency*9 + elapsed) / 10
		}
		w.metrics.mu.Unlock()
	}()

	// Normalize and validate path
	cleanPath := filepath.Clean(path)
	if err := w.validatePath(cleanPath); err != nil {
		w.metrics.mu.Lock()
		w.metrics.WriteDenied++
		w.metrics.mu.Unlock()
		return err
	}

	// Check if path is in temp/ directory (if allowed)
	if w.config.AllowTempFiles && strings.HasPrefix(cleanPath, "temp/") {
		// Temp files are always allowed if AllowTempFiles is true
		return w.writeFile(cleanPath, data)
	}

	// Check allowlist (poka-yoke enforcement)
	w.mu.RLock()
	allowed := w.allowedOut[cleanPath]
	w.mu.RUnlock()

	if !allowed {
		w.metrics.mu.Lock()
		w.metrics.WriteDenied++
		w.metrics.mu.Unlock()
		return fmt.Errorf("%w: path=%s", ErrUndeclaredWrite, cleanPath)
	}

	return w.writeFile(cleanPath, data)
}

// writeFile performs the actual file write operation
func (w *Workspace) writeFile(cleanPath string, data []byte) error {
	absPath := filepath.Join(w.BaseDir, cleanPath)

	// Ensure parent directory exists
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file atomically (write to temp, then rename)
	tempPath := absPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	if err := os.Rename(tempPath, absPath); err != nil {
		os.Remove(tempPath) // Clean up temp file
		return fmt.Errorf("atomic rename failed: %w", err)
	}

	return nil
}

// validatePath checks for path traversal attacks
func (w *Workspace) validatePath(path string) error {
	// Check for absolute paths
	if filepath.IsAbs(path) {
		return ErrPathTraversal
	}

	// Check for ".." components (path traversal)
	if strings.Contains(path, "..") {
		return ErrPathTraversal
	}

	// Check for suspicious characters
	if strings.ContainsAny(path, "\x00") {
		return ErrPathTraversal
	}

	return nil
}

// GetMetrics returns a snapshot of workspace metrics
func (w *Workspace) GetMetrics() WorkspaceMetrics {
	w.metrics.mu.Lock()
	defer w.metrics.mu.Unlock()
	return w.metrics
}

// Snapshot creates a deterministic hash of all output files
//
// Returns:
//   - hash: SHA256 hex digest of all output files (sorted by path)
//   - files: Map of path → file hash
//   - error: If snapshot creation fails
func (w *Workspace) Snapshot() (hash string, files map[string]string, err error) {
	files = make(map[string]string)
	h := sha256.New()

	// Collect all output files
	var paths []string
	w.mu.RLock()
	for path := range w.allowedOut {
		paths = append(paths, path)
	}
	w.mu.RUnlock()

	// Sort paths for determinism
	// Using simple bubble sort for deterministic ordering
	for i := 0; i < len(paths); i++ {
		for j := i + 1; j < len(paths); j++ {
			if paths[i] > paths[j] {
				paths[i], paths[j] = paths[j], paths[i]
			}
		}
	}

	// Hash each file in sorted order
	for _, path := range paths {
		absPath := filepath.Join(w.BaseDir, path)
		data, err := os.ReadFile(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				// File not yet created, use empty hash
				files[path] = "empty"
				h.Write([]byte(path + ":empty\n"))
				continue
			}
			return "", nil, fmt.Errorf("failed to read %s: %w", path, err)
		}

		// Hash individual file
		fileHash := sha256.Sum256(data)
		fileHashHex := hex.EncodeToString(fileHash[:])
		files[path] = fileHashHex

		// Contribute to global hash
		h.Write([]byte(path + ":" + fileHashHex + "\n"))
	}

	globalHash := hex.EncodeToString(h.Sum(nil))
	return globalHash, files, nil
}

// Cleanup removes the workspace directory and all contents
func (w *Workspace) Cleanup() error {
	return os.RemoveAll(w.BaseDir)
}

// Config returns a copy of the workspace configuration
func (w *Workspace) Config() WorkspaceConfig {
	return w.config
}

// GetBaseDir returns the absolute path to the workspace base directory
func (w *Workspace) GetBaseDir() string {
	return w.BaseDir
}
