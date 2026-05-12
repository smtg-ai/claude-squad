package config

import (
	"claude-squad/log"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const WorkspacesFileName = "workspaces.json"

// WorkspaceProfile is a per-workspace agent launch recipe.
// Program is the command line; Env is exported into the tmux session; EnvFiles
// is an env-var → file-path map resolved at session-start time so secrets don't
// live in JSON; AgentHome redirects per-tool config dirs (CLAUDE_CONFIG_DIR,
// GH_CONFIG_DIR, GIT_CONFIG_GLOBAL, ...).
type WorkspaceProfile struct {
	Name      string            `json:"name"`
	Program   string            `json:"program"`
	Env       map[string]string `json:"env,omitempty"`
	EnvFiles  map[string]string `json:"env_files,omitempty"`
	AgentHome map[string]string `json:"agent_home,omitempty"`
}

type Hooks struct {
	PostWorktree string `json:"post_worktree,omitempty"`
}

type Workspace struct {
	ID          string             `json:"id"`
	DisplayName string             `json:"display_name"`
	RepoPath    string             `json:"repo_path"`
	RemoteURL   string             `json:"remote_url,omitempty"`
	Profiles    []WorkspaceProfile `json:"profiles,omitempty"`
	WorktreeDir string             `json:"worktree_dir,omitempty"`
	Hooks       Hooks              `json:"hooks,omitempty"`
	CreatedAt   time.Time          `json:"created_at"`
	LastUsedAt  time.Time          `json:"last_used_at"`
}

// WorkspaceID derives a stable identifier from a (canonical repo path, remote URL) pair.
func WorkspaceID(canonicalRepoPath, remoteURL string) string {
	h := sha256.Sum256([]byte(canonicalRepoPath + "\x00" + remoteURL))
	return hex.EncodeToString(h[:6])
}

type WorkspaceRegistry struct {
	Workspaces []Workspace `json:"workspaces"`

	mu sync.Mutex
}

func workspaceRegistryPath() (string, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, WorkspacesFileName), nil
}

func LoadWorkspaceRegistry() *WorkspaceRegistry {
	p, err := workspaceRegistryPath()
	if err != nil {
		log.ErrorLog.Printf("workspace registry path: %v", err)
		return &WorkspaceRegistry{}
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if !os.IsNotExist(err) {
			log.WarningLog.Printf("read workspace registry: %v", err)
		}
		return &WorkspaceRegistry{}
	}
	r := &WorkspaceRegistry{}
	if err := json.Unmarshal(data, r); err != nil {
		log.ErrorLog.Printf("parse workspace registry: %v", err)
		return &WorkspaceRegistry{}
	}
	return r
}

func (r *WorkspaceRegistry) Save() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.saveLocked()
}

func (r *WorkspaceRegistry) saveLocked() error {
	p, err := workspaceRegistryPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

func (r *WorkspaceRegistry) Get(id string) *Workspace {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.Workspaces {
		if r.Workspaces[i].ID == id {
			return &r.Workspaces[i]
		}
	}
	return nil
}

func (r *WorkspaceRegistry) FindByName(name string) *Workspace {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.Workspaces {
		if strings.EqualFold(r.Workspaces[i].DisplayName, name) {
			return &r.Workspaces[i]
		}
	}
	return nil
}

func (r *WorkspaceRegistry) FindByRepoPath(canonicalPath string) *Workspace {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.Workspaces {
		if r.Workspaces[i].RepoPath == canonicalPath {
			return &r.Workspaces[i]
		}
	}
	return nil
}

// Upsert inserts or updates a workspace by ID. Idempotent.
func (r *WorkspaceRegistry) Upsert(ws Workspace) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.Workspaces {
		if r.Workspaces[i].ID == ws.ID {
			r.Workspaces[i] = ws
			return r.saveLocked()
		}
	}
	r.Workspaces = append(r.Workspaces, ws)
	return r.saveLocked()
}

func (r *WorkspaceRegistry) Remove(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.Workspaces[:0]
	for _, w := range r.Workspaces {
		if w.ID != id {
			out = append(out, w)
		}
	}
	r.Workspaces = out
	return r.saveLocked()
}

// Touch updates LastUsedAt for a workspace by ID. Best-effort; ignores not-found.
func (r *WorkspaceRegistry) Touch(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.Workspaces {
		if r.Workspaces[i].ID == id {
			r.Workspaces[i].LastUsedAt = time.Now()
			return r.saveLocked()
		}
	}
	return nil
}

// EnsureWorkspace returns the workspace for (canonicalPath, remoteURL), registering
// a fresh one with sensible defaults if none exists. Idempotent. Callers compute
// the canonical path and remote URL themselves so this package stays free of any
// direct git/exec dependency.
func (r *WorkspaceRegistry) EnsureWorkspace(canonicalPath, remoteURL string) (*Workspace, error) {
	id := WorkspaceID(canonicalPath, remoteURL)
	if existing := r.Get(id); existing != nil {
		_ = r.Touch(id)
		return existing, nil
	}
	now := time.Now()
	ws := Workspace{
		ID:          id,
		DisplayName: filepath.Base(canonicalPath),
		RepoPath:    canonicalPath,
		RemoteURL:   remoteURL,
		CreatedAt:   now,
		LastUsedAt:  now,
	}
	if err := r.Upsert(ws); err != nil {
		return nil, fmt.Errorf("register workspace: %w", err)
	}
	return r.Get(id), nil
}

// MostRecentlyUsed returns the workspace whose LastUsedAt is most recent, or
// nil if no workspaces are registered. Used as a fallback when cs is launched
// outside any git repo.
func (r *WorkspaceRegistry) MostRecentlyUsed() *Workspace {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.Workspaces) == 0 {
		return nil
	}
	bestIdx := 0
	for i := 1; i < len(r.Workspaces); i++ {
		if r.Workspaces[i].LastUsedAt.After(r.Workspaces[bestIdx].LastUsedAt) {
			bestIdx = i
		}
	}
	w := r.Workspaces[bestIdx]
	return &w
}

// Dir returns the on-disk directory for this workspace's per-workspace state.
func (w *Workspace) Dir() (string, error) {
	root, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "workspaces", w.ID), nil
}

// WorktreeRoot returns the directory where this workspace's worktrees live.
func (w *Workspace) WorktreeRoot() (string, error) {
	if w.WorktreeDir != "" {
		return w.WorktreeDir, nil
	}
	dir, err := w.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "worktrees"), nil
}

func (w *Workspace) FindProfile(name string) *WorkspaceProfile {
	for i := range w.Profiles {
		if w.Profiles[i].Name == name {
			return &w.Profiles[i]
		}
	}
	return nil
}

// ResolveEnv builds the env that a session in this workspace + profile would
// inherit: Env merged with EnvFiles (read from disk) and AgentHome entries
// (resolved to absolute paths). Returned as a sorted "KEY=VALUE" slice so the
// output is stable for debug listings. Does not mkdir AgentHome targets —
// that's the runtime path's job.
func (w *Workspace) ResolveEnv(profile *WorkspaceProfile) ([]string, error) {
	if profile == nil {
		return nil, nil
	}
	wsDir, err := w.Dir()
	if err != nil {
		return nil, err
	}
	merged := map[string]string{}
	for k, v := range profile.Env {
		merged[k] = v
	}
	for k, rel := range profile.EnvFiles {
		full := rel
		if !filepath.IsAbs(full) {
			full = filepath.Join(wsDir, full)
		}
		b, err := os.ReadFile(full)
		if err != nil {
			return nil, fmt.Errorf("read env file %s for %s: %w", full, k, err)
		}
		merged[k] = strings.TrimRight(string(b), "\n")
	}
	for varName, rel := range profile.AgentHome {
		full := rel
		if !filepath.IsAbs(full) {
			full = filepath.Join(wsDir, full)
		}
		merged[varName] = full
	}
	out := make([]string, 0, len(merged))
	for k, v := range merged {
		out = append(out, k+"="+v)
	}
	sort.Strings(out)
	return out, nil
}

// String returns "<id>  <name>  <path>" for CLI listing.
func (w *Workspace) String() string {
	return fmt.Sprintf("%s\t%s\t%s", w.ID, w.DisplayName, w.RepoPath)
}
