# GUI Session Creation — Branch Picker & In-Place Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add branch selection and in-place session toggle to the GUI's new session dialog, and implement the core in-place session data model with lifecycle guards.

**Architecture:** The `Instance` struct gains an `inPlace` field that, when true, skips all git worktree creation and lifecycle operations. The GUI dialog gains a searchable branch picker (defaulting to "New branch from origin default") and an in-place checkbox that hides the branch picker. A new `GetDefaultBranch()` git helper detects the remote default branch with local fallback. A new `NewGitWorktreeFromRef()` function creates worktrees based on a specific remote ref.

**Tech Stack:** Go, Fyne v2 GUI framework, tmux, git CLI

**Spec:** `docs/superpowers/specs/2026-03-24-gui-session-creation-design.md`

---

## File Structure

| File | Changes | Purpose |
|------|---------|---------|
| `session/git/worktree_git.go` | Modify | Add `GetDefaultBranch()`, `GetCurrentBranch()` |
| `session/git/worktree_git_test.go` | Create | Tests for `GetDefaultBranch` |
| `session/git/worktree.go` | Modify | Add `NewGitWorktreeFromRef()` |
| `session/git/worktree_ops.go` | Modify | Add `setupFromRef()` for worktree creation from a ref |
| `session/storage.go` | Modify | Add `InPlace` to `InstanceData` |
| `session/storage_test.go` | Create | Serialization round-trip and backward compat tests |
| `session/instance.go` | Modify | Add `inPlace` field, `IsInPlace()`, modify `Start()`, `Pause()`, `Resume()`, `UpdateDiffStats()`, `RepoName()`, `FromInstanceData()` |
| `session/instance_test.go` | Create | Tests for in-place lifecycle guards |
| `gui/dialogs/new_session.go` | Modify | Add branch picker and in-place toggle |
| `gui/app.go` | Modify | Wire new dialog options, add nil guard in `pushSession` |
| `app/app.go` | Modify | Add in-place guards to TUI kill and push handlers |

---

## Task 1: GetDefaultBranch and GetCurrentBranch Git Helpers

**Files:**
- Modify: `session/git/worktree_git.go`
- Create: `session/git/worktree_git_test.go`

- [ ] **Step 1: Write tests for GetDefaultBranch**

```go
// session/git/worktree_git_test.go
package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// createTestRepo creates a temporary git repo with an initial commit and returns its path.
func createTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmds := [][]string{
		{"git", "init"},
		{"git", "commit", "--allow-empty", "-m", "initial"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup %v: %s (%v)", args, out, err)
		}
	}
	return dir
}

func TestGetDefaultBranch(t *testing.T) {
	repo := createTestRepo(t)

	// Should fall back to current branch (no origin)
	branch := GetDefaultBranch(repo)
	if branch == "" {
		t.Fatal("expected non-empty branch name")
	}
	// Should be "main" or "master" depending on git config
	if branch != "main" && branch != "master" {
		t.Fatalf("unexpected default branch: %s", branch)
	}
}

func TestGetDefaultBranchWithOrigin(t *testing.T) {
	// Create a "remote" repo
	remote := createTestRepo(t)
	// Create local clone
	local := t.TempDir()
	cmd := exec.Command("git", "clone", remote, local)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone: %s (%v)", out, err)
	}

	branch := GetDefaultBranch(local)
	if branch == "" {
		t.Fatal("expected non-empty branch name")
	}
}

func TestGetDefaultBranchNonGitDir(t *testing.T) {
	dir := t.TempDir()
	branch := GetDefaultBranch(dir)
	if branch != "main" {
		t.Fatalf("expected 'main' fallback for non-git dir, got: %s", branch)
	}
}

func TestGetCurrentBranch(t *testing.T) {
	repo := createTestRepo(t)
	branch, err := GetCurrentBranch(repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch == "" {
		t.Fatal("expected non-empty branch name")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./session/git/ -run "TestGetDefaultBranch|TestGetCurrentBranch" -v`
Expected: FAIL — functions not defined

- [ ] **Step 3: Implement GetDefaultBranch and GetCurrentBranch**

Add to `session/git/worktree_git.go`:

```go
// GetCurrentBranch returns the current branch name for the given repo path.
func GetCurrentBranch(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "branch", "--show-current")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %s (%w)", output, err)
	}
	return strings.TrimSpace(string(output)), nil
}

// GetDefaultBranch returns the default branch name for the remote origin.
// Falls back to the current branch, then to "main" if both fail.
func GetDefaultBranch(repoPath string) string {
	// Try to get the remote default branch
	cmd := exec.Command("git", "-C", repoPath, "symbolic-ref", "refs/remotes/origin/HEAD")
	if output, err := cmd.CombinedOutput(); err == nil {
		ref := strings.TrimSpace(string(output))
		// Strip "refs/remotes/origin/" prefix
		if name := strings.TrimPrefix(ref, "refs/remotes/origin/"); name != ref {
			return name
		}
	}

	// Fall back to current branch
	if branch, err := GetCurrentBranch(repoPath); err == nil && branch != "" {
		return branch
	}

	// Final fallback
	return "main"
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./session/git/ -run "TestGetDefaultBranch|TestGetCurrentBranch" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add session/git/worktree_git.go session/git/worktree_git_test.go
git commit -m "feat(git): add GetDefaultBranch and GetCurrentBranch helpers"
```

---

## Task 2: NewGitWorktreeFromRef

**Files:**
- Modify: `session/git/worktree.go`
- Modify: `session/git/worktree_ops.go`

- [ ] **Step 1: Add NewGitWorktreeFromRef to worktree.go**

Add after `NewGitWorktreeFromBranch` in `session/git/worktree.go`:

```go
// NewGitWorktreeFromRef creates a new GitWorktree with a new branch based on a specific ref
// (e.g., "origin/main"). The new branch is named using the configured branch prefix + session name.
func NewGitWorktreeFromRef(repoPath string, baseRef string, sessionName string) (tree *GitWorktree, branchName string, err error) {
	cfg := config.LoadConfig()
	branchName = fmt.Sprintf("%s%s", cfg.BranchPrefix, sessionName)
	branchName = sanitizeBranchName(branchName)

	repoPath, worktreePath, err := resolveWorktreePaths(repoPath, branchName)
	if err != nil {
		return nil, "", err
	}

	return &GitWorktree{
		repoPath:     repoPath,
		sessionName:  sessionName,
		branchName:   branchName,
		worktreePath: worktreePath,
		baseRef:      baseRef,
	}, branchName, nil
}
```

- [ ] **Step 2: Add baseRef field to GitWorktree struct**

In `session/git/worktree.go`, add to the `GitWorktree` struct after `isExistingBranch`:

```go
	// baseRef is the ref to base a new branch on (e.g., "origin/main").
	// Only used during Setup for new worktrees. Empty means use HEAD.
	baseRef string
```

- [ ] **Step 3: Add setupFromRef to worktree_ops.go**

Add after `setupNewWorktree` in `session/git/worktree_ops.go`:

```go
// setupFromRef creates a new worktree with a new branch based on a specific ref.
func (g *GitWorktree) setupFromRef() error {
	// Clean up any existing worktree first
	_, _ = g.runGitCommand(g.repoPath, "worktree", "remove", "-f", g.worktreePath)

	// Clean up any existing branch
	_, _ = g.runGitCommand(g.repoPath, "branch", "-D", g.branchName)

	// Resolve the ref to a commit SHA for baseCommitSHA
	output, err := g.runGitCommand(g.repoPath, "rev-parse", g.baseRef)
	if err != nil {
		return fmt.Errorf("failed to resolve ref %s: %w", g.baseRef, err)
	}
	g.baseCommitSHA = strings.TrimSpace(string(output))

	// Create worktree with new branch based on the ref
	if _, err := g.runGitCommand(g.repoPath, "worktree", "add", "-b", g.branchName, g.worktreePath, g.baseRef); err != nil {
		return fmt.Errorf("failed to create worktree from ref %s: %w", g.baseRef, err)
	}

	return nil
}
```

- [ ] **Step 4: Modify Setup() to use setupFromRef when baseRef is set**

In `session/git/worktree_ops.go`, modify `Setup()` to check `baseRef` before the existing branch check. Add after the `isExistingBranch` check (after line 28):

```go
	// If a base ref is specified, create a new branch from that ref
	if g.baseRef != "" {
		return g.setupFromRef()
	}
```

- [ ] **Step 5: Run existing tests to ensure no regression**

Run: `go test ./session/git/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add session/git/worktree.go session/git/worktree_ops.go
git commit -m "feat(git): add NewGitWorktreeFromRef for creating worktrees from a specific ref"
```

---

## Task 3: In-Place Data Model — Storage and Instance

**Files:**
- Modify: `session/storage.go:11-25`
- Create: `session/storage_test.go`
- Modify: `session/instance.go:31-68` (struct), `session/instance.go:148-160` (InstanceOptions), `session/instance.go:162-183` (NewInstance)

- [ ] **Step 1: Write serialization tests**

```go
// session/storage_test.go
package session

import (
	"encoding/json"
	"testing"
)

func TestInPlaceSessionSerialization(t *testing.T) {
	data := InstanceData{
		Title:   "test-inplace",
		Path:    "/some/path",
		InPlace: true,
		Program: "claude",
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var restored InstanceData
	if err := json.Unmarshal(jsonBytes, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !restored.InPlace {
		t.Error("expected InPlace to be true")
	}
	if restored.Worktree.RepoPath != "" {
		t.Error("expected empty worktree for in-place session")
	}
}

func TestInPlaceBackwardCompatibility(t *testing.T) {
	// Old JSON without in_place field
	oldJSON := `{"title":"old","path":"/old","status":0,"program":"claude","worktree":{"repo_path":"/r","worktree_path":"/w","session_name":"s","branch_name":"b","base_commit_sha":"c"}}`

	var data InstanceData
	if err := json.Unmarshal([]byte(oldJSON), &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if data.InPlace {
		t.Error("old sessions should not be in-place")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./session/ -run "TestInPlace" -v`
Expected: FAIL — `InPlace` field not found on `InstanceData`

- [ ] **Step 3: Add InPlace to InstanceData**

In `session/storage.go`, add after the `AutoYes` field (line 20):

```go
	InPlace   bool      `json:"in_place,omitempty"`
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./session/ -run "TestInPlace" -v`
Expected: PASS

- [ ] **Step 5: Add inPlace to Instance struct and InstanceOptions**

In `session/instance.go`, add to `Instance` struct after `selectedBranch` (after line 59):

```go
	// inPlace is true when the session runs directly in the working directory
	// without git worktree isolation. gitWorktree will be nil.
	inPlace bool
```

Add `IsInPlace()` accessor after `SetSelectedBranch` (after line 199):

```go
// IsInPlace returns whether this session runs in-place without git isolation.
func (i *Instance) IsInPlace() bool {
	return i.inPlace
}
```

Add `InPlace` to `InstanceOptions` after `Branch` (after line 159):

```go
	// InPlace runs the session directly in the working directory without git isolation.
	InPlace bool
```

Wire it in `NewInstance()` — add `inPlace: opts.InPlace,` to the struct literal (after `selectedBranch`).

- [ ] **Step 6: Wire InPlace through ToInstanceData and FromInstanceData**

In `ToInstanceData()`, add after `AutoYes` (line 82):

```go
		InPlace:   i.inPlace,
```

In `FromInstanceData()`, modify the instance creation (lines 110-134). Add `inPlace` field and conditionally skip worktree creation:

```go
func FromInstanceData(data InstanceData) (*Instance, error) {
	instance := &Instance{
		Title:     data.Title,
		Path:      data.Path,
		Branch:    data.Branch,
		Status:    data.Status,
		Height:    data.Height,
		Width:     data.Width,
		CreatedAt: data.CreatedAt,
		UpdatedAt: data.UpdatedAt,
		Program:   data.Program,
		inPlace:   data.InPlace,
		diffStats: &git.DiffStats{
			Added:   data.DiffStats.Added,
			Removed: data.DiffStats.Removed,
			Content: data.DiffStats.Content,
		},
	}

	// Only create git worktree from storage if not an in-place session
	if !data.InPlace {
		instance.gitWorktree = git.NewGitWorktreeFromStorage(
			data.Worktree.RepoPath,
			data.Worktree.WorktreePath,
			data.Worktree.SessionName,
			data.Worktree.BranchName,
			data.Worktree.BaseCommitSHA,
			data.Worktree.IsExistingBranch,
		)
	}

	if instance.Paused() {
		instance.started = true
		instance.tmuxSession = tmux.NewTmuxSession(instance.Title, instance.Program)
	} else {
		if err := instance.Start(false); err != nil {
			return nil, err
		}
	}

	return instance, nil
}
```

- [ ] **Step 7: Run all session tests**

Run: `go test ./session/ -v`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add session/storage.go session/storage_test.go session/instance.go
git commit -m "feat(session): add inPlace field to Instance and InstanceData"
```

---

## Task 4: In-Place Lifecycle Guards

**Files:**
- Modify: `session/instance.go:202-274` (Start), `session/instance.go:185-190` (RepoName), `session/instance.go:372-378` (GetGitWorktree), `session/instance.go:415-475` (Pause), `session/instance.go:478-534` (Resume), `session/instance.go:537-560` (UpdateDiffStats)
- Create: `session/instance_test.go`

- [ ] **Step 1: Write tests for in-place lifecycle**

```go
// session/instance_test.go
package session

import (
	"claude-squad/log"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	log.Initialize(false)
	defer log.Close()
	os.Exit(m.Run())
}

func TestInPlaceRepoName(t *testing.T) {
	inst := &Instance{
		Path:    "/some/path/myproject",
		started: true,
		inPlace: true,
	}
	name, err := inst.RepoName()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "myproject" {
		t.Fatalf("expected 'myproject', got '%s'", name)
	}
}

func TestInPlaceUpdateDiffStatsNoOp(t *testing.T) {
	inst := &Instance{
		started: true,
		inPlace: true,
	}
	if err := inst.UpdateDiffStats(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.diffStats != nil {
		t.Error("expected nil diffStats for in-place session")
	}
}

func TestInPlaceGetGitWorktreeReturnsNil(t *testing.T) {
	inst := &Instance{
		started: true,
		inPlace: true,
	}
	wt, err := inst.GetGitWorktree()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wt != nil {
		t.Error("expected nil worktree for in-place session")
	}
}

func TestInPlacePauseDoesNotPanic(t *testing.T) {
	inst := &Instance{
		started: true,
		inPlace: true,
		Status:  Running,
	}
	// Pause will fail because tmuxSession is nil, but it must NOT panic
	// on nil gitWorktree access
	err := inst.Pause()
	if err == nil {
		t.Error("expected error due to nil tmux session")
	}
	if err.Error() != "tmux session is nil" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInPlaceResumeDoesNotPanic(t *testing.T) {
	inst := &Instance{
		started: true,
		inPlace: true,
		Status:  Paused,
	}
	// Resume will fail because tmuxSession is nil, but it must NOT panic
	// on nil gitWorktree access
	err := inst.Resume()
	if err == nil {
		t.Error("expected error due to nil tmux session")
	}
	if err.Error() != "tmux session is nil" {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./session/ -run "TestInPlace" -v`
Expected: FAIL — RepoName panics on nil gitWorktree, UpdateDiffStats panics on nil gitWorktree

- [ ] **Step 3: Modify Start() for in-place sessions**

In `session/instance.go`, modify `Start()`. Replace the worktree creation block (lines 217-233, the `if firstTimeSetup {` body) with the code below. **Do NOT touch the `!firstTimeSetup` restore path (lines 247-252) — it stays as-is.**

```go
	if firstTimeSetup {
		if i.inPlace {
			// In-place: no worktree, set branch from current working directory
			if branch, err := git.GetCurrentBranch(i.Path); err == nil && branch != "" {
				i.Branch = branch
			}
		} else if i.selectedBranch != "" {
			gitWorktree, err := git.NewGitWorktreeFromBranch(i.Path, i.selectedBranch, i.Title)
			if err != nil {
				return fmt.Errorf("failed to create git worktree from branch: %w", err)
			}
			i.gitWorktree = gitWorktree
			i.Branch = i.selectedBranch
		} else {
			// Default: fetch origin and create worktree from remote default branch
			git.FetchBranches(i.Path)
			defaultBranch := git.GetDefaultBranch(i.Path)
			baseRef := fmt.Sprintf("origin/%s", defaultBranch)
			gitWorktree, branchName, err := git.NewGitWorktreeFromRef(i.Path, baseRef, i.Title)
			if err != nil {
				// Fall back to HEAD if origin ref fails
				gitWorktree, branchName, err = git.NewGitWorktree(i.Path, i.Title)
				if err != nil {
					return fmt.Errorf("failed to create git worktree: %w", err)
				}
			}
			i.gitWorktree = gitWorktree
			i.Branch = branchName
		}
	}
```

Replace **only** the `} else {` block for `firstTimeSetup == true` (lines 253-268, the worktree setup + tmux start). The `!firstTimeSetup` path at lines 247-252 (`tmuxSession.Restore()`) is unchanged:

```go
	} else {
		if i.inPlace {
			// In-place: just start tmux in working directory
			if err := i.tmuxSession.Start(i.Path); err != nil {
				setupErr = fmt.Errorf("failed to start new session: %w", err)
				return setupErr
			}
		} else {
			// Setup git worktree first
			if err := i.gitWorktree.Setup(); err != nil {
				setupErr = fmt.Errorf("failed to setup git worktree: %w", err)
				return setupErr
			}

			// Create new session
			if err := i.tmuxSession.Start(i.gitWorktree.GetWorktreePath()); err != nil {
				// Cleanup git worktree if tmux session creation fails
				if cleanupErr := i.gitWorktree.Cleanup(); cleanupErr != nil {
					err = fmt.Errorf("%v (cleanup error: %v)", err, cleanupErr)
				}
				setupErr = fmt.Errorf("failed to start new session: %w", err)
				return setupErr
			}
		}
	}
```

- [ ] **Step 4: Modify RepoName() for in-place sessions**

Replace `RepoName()` (lines 185-190):

```go
func (i *Instance) RepoName() (string, error) {
	if !i.started {
		return "", fmt.Errorf("cannot get repo name for instance that has not been started")
	}
	if i.gitWorktree == nil {
		return filepath.Base(i.Path), nil
	}
	return i.gitWorktree.GetRepoName(), nil
}
```

- [ ] **Step 5: Modify UpdateDiffStats() for in-place sessions**

Replace `UpdateDiffStats()` (lines 537-560). Add guard after the `!i.started` check:

```go
func (i *Instance) UpdateDiffStats() error {
	if !i.started {
		i.diffStats = nil
		return nil
	}

	if i.Status == Paused {
		return nil
	}

	if i.gitWorktree == nil {
		i.diffStats = nil
		return nil
	}

	stats := i.gitWorktree.Diff()
	if stats.Error != nil {
		if strings.Contains(stats.Error.Error(), "base commit SHA not set") {
			i.diffStats = nil
			return nil
		}
		return fmt.Errorf("failed to get diff stats: %w", stats.Error)
	}

	i.diffStats = stats
	return nil
}
```

- [ ] **Step 6: Modify Pause() for in-place sessions**

In `Pause()` (lines 415-475), add in-place guard after the `already paused` check (after line 421):

```go
	// In-place sessions: just detach tmux, no git operations
	if i.inPlace {
		if i.tmuxSession == nil {
			return fmt.Errorf("tmux session is nil")
		}
		if err := i.tmuxSession.DetachSafely(); err != nil {
			return fmt.Errorf("failed to detach tmux session: %w", err)
		}
		i.SetStatus(Paused)
		return nil
	}
```

- [ ] **Step 7: Modify Resume() for in-place sessions**

In `Resume()` (lines 478-534), add in-place guard after the `not paused` check (after line 484):

```go
	// In-place sessions: just restart tmux in working directory
	if i.inPlace {
		if i.tmuxSession == nil {
			return fmt.Errorf("tmux session is nil")
		}
		if i.tmuxSession.DoesSessionExist() {
			if err := i.tmuxSession.Restore(); err != nil {
				if err := i.tmuxSession.Start(i.Path); err != nil {
					return fmt.Errorf("failed to start new session: %w", err)
				}
			}
		} else {
			if err := i.tmuxSession.Start(i.Path); err != nil {
				return fmt.Errorf("failed to start new session: %w", err)
			}
		}
		i.SetStatus(Running)
		return nil
	}
```

- [ ] **Step 8: Run tests**

Run: `go test ./session/ -v`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add session/instance.go session/instance_test.go
git commit -m "feat(session): add in-place lifecycle guards for Start, Pause, Resume, RepoName, UpdateDiffStats"
```

---

## Task 5: GUI Dialog — Branch Picker and In-Place Toggle

**Files:**
- Modify: `gui/dialogs/new_session.go`

- [ ] **Step 1: Add Branch and InPlace to SessionOptions**

Update the struct at the top of `gui/dialogs/new_session.go`:

```go
type SessionOptions struct {
	Name    string
	Prompt  string
	Program string
	Branch  string // empty = new branch from default
	InPlace bool
}
```

- [ ] **Step 2: Rewrite ShowNewSession with branch picker and in-place toggle**

Replace the entire `ShowNewSession` function:

```go
func ShowNewSession(profiles []config.Profile, defaultBranch string, branches []string, parent fyne.Window, onBranchSearch func(filter string) []string, onSubmit func(SessionOptions)) {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Session name")

	promptEntry := widget.NewMultiLineEntry()
	promptEntry.SetPlaceHolder("Initial prompt (optional)")
	promptEntry.SetMinRowsVisible(3)

	// In-place toggle
	inPlaceCheck := widget.NewCheck("Run in-place (no git isolation)", nil)

	// Branch picker
	newBranchLabel := fmt.Sprintf("New branch (from %s)", defaultBranch)
	branchOptions := append([]string{newBranchLabel}, branches...)
	branchSelect := widget.NewSelect(branchOptions, nil)
	branchSelect.SetSelected(newBranchLabel)

	// Search entry for filtering branches
	branchSearch := widget.NewEntry()
	branchSearch.SetPlaceHolder("Search branches...")
	branchSearch.OnChanged = func(filter string) {
		if onBranchSearch == nil {
			return
		}
		filtered := onBranchSearch(filter)
		newOptions := append([]string{newBranchLabel}, filtered...)
		branchSelect.Options = newOptions
		branchSelect.Refresh()
	}

	branchContainer := container.NewVBox(branchSearch, branchSelect)
	branchFormItem := widget.NewFormItem("Branch", branchContainer)

	// Toggle branch picker visibility based on in-place checkbox
	inPlaceCheck.OnChanged = func(checked bool) {
		if checked {
			branchFormItem.Widget = widget.NewLabel("(disabled for in-place sessions)")
		} else {
			branchFormItem.Widget = branchContainer
		}
		// Force form to re-layout
		parent.Canvas().Content().Refresh()
	}

	// Program/profile selector
	profileNames := make([]string, len(profiles))
	for i, p := range profiles {
		profileNames[i] = p.Name
	}
	programSelect := widget.NewSelect(profileNames, nil)
	if len(profileNames) > 0 {
		programSelect.SetSelected(profileNames[0])
	}

	items := []*widget.FormItem{
		widget.NewFormItem("Name", nameEntry),
		widget.NewFormItem("In-place", inPlaceCheck),
		branchFormItem,
		widget.NewFormItem("Prompt", promptEntry),
	}
	if len(profiles) > 1 {
		items = append(items, widget.NewFormItem("Program", programSelect))
	}

	d := dialog.NewForm("New Session", "Create", "Cancel", items, func(confirmed bool) {
		if !confirmed {
			return
		}
		opts := SessionOptions{
			Name:    nameEntry.Text,
			Prompt:  promptEntry.Text,
			InPlace: inPlaceCheck.Checked,
		}

		// Resolve branch selection
		if !inPlaceCheck.Checked && branchSelect.Selected != newBranchLabel {
			opts.Branch = branchSelect.Selected
		}

		// Resolve program from profile
		selected := programSelect.Selected
		for _, p := range profiles {
			if p.Name == selected {
				opts.Program = p.Program
				break
			}
		}
		if onSubmit != nil {
			onSubmit(opts)
		}
	}, parent)
	d.Resize(fyne.NewSize(500, 500))
	d.Show()
}
```

- [ ] **Step 3: Add container import**

Add `"fyne.io/fyne/v2/container"` and `"fmt"` to the imports in `gui/dialogs/new_session.go`.

- [ ] **Step 4: Verify it compiles**

Run: `go vet ./gui/dialogs/...`
Expected: PASS (only verify the dialogs package — `gui/app.go` will not compile until Task 6 updates the caller).

- [ ] **Step 5: Commit**

```bash
git add gui/dialogs/new_session.go
git commit -m "feat(gui): add branch picker and in-place toggle to new session dialog"
```

---

## Task 6: Wire GUI Dialog to Instance Creation

**Files:**
- Modify: `gui/app.go:310-353` (showNewSessionDialog), `gui/app.go:375-385` (pushSession)

- [ ] **Step 1: Update showNewSessionDialog**

Replace the function in `gui/app.go`:

```go
func showNewSessionDialog(w fyne.Window, cfg *config.Config, defaultProgram string, state *guiState, sb *sidebar.Sidebar, pm *panes.Manager, autoYes bool) {
	defaultBranch := git.GetDefaultBranch(".")
	branches, _ := git.SearchBranches(".", "")

	dialogs.ShowNewSession(cfg.GetProfiles(), defaultBranch, branches, w,
		func(filter string) []string {
			results, _ := git.SearchBranches(".", filter)
			return results
		},
		func(opts dialogs.SessionOptions) {
			if opts.Name == "" {
				return
			}
			prog := opts.Program
			if prog == "" {
				prog = defaultProgram
			}
			inst, err := session.NewInstance(session.InstanceOptions{
				Title:   opts.Name,
				Path:    ".",
				Program: prog,
				InPlace: opts.InPlace,
			})
			if err != nil {
				log.ErrorLog.Printf("failed to create instance: %v", err)
				return
			}
			inst.AutoYes = autoYes
			inst.Prompt = opts.Prompt
			if opts.Branch != "" {
				inst.SetSelectedBranch(opts.Branch)
			}
			inst.SetStatus(session.Loading)
			state.addInstance(inst)
			sb.Update(state.getInstances())

			go func() {
				if err := inst.Start(true); err != nil {
					log.ErrorLog.Printf("failed to start instance: %v", err)
					return
				}
				if opts.Prompt != "" {
					if err := inst.SendPrompt(opts.Prompt); err != nil {
						log.ErrorLog.Printf("failed to send prompt: %v", err)
					}
					inst.Prompt = ""
				}
				fyne.Do(func() {
					sb.Update(state.getInstances())
				})
				if err := state.storage.SaveInstances(state.getInstances()); err != nil {
					log.ErrorLog.Printf("failed to save instances: %v", err)
				}
			}()
		})
}
```

- [ ] **Step 2: Add git import to gui/app.go**

Add `"claude-squad/session/git"` to imports in `gui/app.go`.

- [ ] **Step 3: Add nil guard to pushSession**

Replace `pushSession` in `gui/app.go`:

```go
func pushSession(inst *session.Instance) {
	if inst.IsInPlace() {
		log.WarningLog.Printf("cannot push in-place session '%s'", inst.Title)
		return
	}
	worktree, err := inst.GetGitWorktree()
	if err != nil {
		log.ErrorLog.Printf("failed to get worktree: %v", err)
		return
	}
	if worktree == nil {
		log.ErrorLog.Printf("no worktree for session '%s'", inst.Title)
		return
	}
	commitMsg := fmt.Sprintf("[claudesquad] update from '%s' on %s", inst.Title, time.Now().Format(time.RFC822))
	if err := worktree.PushChanges(commitMsg, true); err != nil {
		log.ErrorLog.Printf("failed to push changes: %v", err)
	}
}
```

- [ ] **Step 4: Verify it compiles**

Run: `go build ./gui/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add gui/app.go
git commit -m "feat(gui): wire branch picker and in-place toggle to session creation"
```

---

## Task 7: TUI In-Place Guards

**Files:**
- Modify: `app/app.go:658-665` (kill handler), `app/app.go:698-706` (push handler)

- [ ] **Step 1: Add in-place guard to TUI kill handler**

In `app/app.go`, find the kill handler around line 658. Currently:

```go
worktree, err := selected.GetGitWorktree()
if err != nil {
    return err
}

checkedOut, err := worktree.IsBranchCheckedOut()
```

Add a nil check after getting the worktree:

```go
worktree, err := selected.GetGitWorktree()
if err != nil {
    return err
}

// Skip branch-checked-out warning for in-place sessions (no worktree)
if worktree == nil {
    // Proceed directly to kill confirmation
} else {
    checkedOut, err := worktree.IsBranchCheckedOut()
```

Adjust the surrounding code so the `IsBranchCheckedOut` logic is only run when `worktree != nil`.

- [ ] **Step 2: Add in-place guard to TUI push handler**

In `app/app.go`, find the push handler around line 698. Currently:

```go
worktree, err := selected.GetGitWorktree()
if err != nil {
    return err
}
if err = worktree.PushChanges(commitMsg, true); err != nil {
```

Add an `IsInPlace()` check before the push:

```go
if selected.IsInPlace() {
    return fmt.Errorf("cannot push in-place session")
}
worktree, err := selected.GetGitWorktree()
if err != nil {
    return err
}
if worktree == nil {
    return fmt.Errorf("no worktree for session")
}
if err = worktree.PushChanges(commitMsg, true); err != nil {
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./app/...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add app/app.go
git commit -m "fix(tui): add in-place guards to kill and push handlers"
```

---

## Task 8: Build and Smoke Test

- [ ] **Step 1: Run all tests**

Run: `go test ./... 2>&1 | head -50`
Expected: All tests pass

- [ ] **Step 2: Build the binary**

Run: `go build -o cs .`
Expected: Clean build

- [ ] **Step 3: Commit any remaining fixes**

If any compilation or test issues were found, fix and commit them.
