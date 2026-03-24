# In-Place Sessions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an "in-place" session mode that runs the AI agent directly in the current working directory without creating git branches or worktrees.

**Architecture:** The `Instance` struct gains an `inPlace bool` field. When true, `gitWorktree` is nil — all lifecycle methods (Start, Kill, Pause, Resume, UpdateDiffStats) early-return before any gitWorktree access. The session creation overlay gains a toggle that hides branch/submodule pickers when enabled. A new `i` keybinding opens the overlay with the toggle pre-selected.

**Tech Stack:** Go, bubbletea TUI framework, tmux for session management

**Spec:** `docs/superpowers/specs/2026-03-23-in-place-sessions-design.md`

---

## File Structure

| File | Changes | Purpose |
|------|---------|---------|
| `session/instance.go` | Modify | Add `inPlace` field, `IsInPlace()` accessor, modify lifecycle methods |
| `session/storage.go` | Modify | Add `InPlace` to `InstanceData` |
| `session/storage_test.go` | Modify | Add serialization tests for in-place sessions |
| `keys/keys.go` | Modify | Add `KeyInPlace` iota and `"i"` mapping |
| `ui/overlay/textInput.go` | Modify | Add in-place toggle, dynamic focus order |
| `ui/overlay/textInput_test.go` | Create | Test toggle behavior, focus order changes |
| `ui/list.go` | Modify | Show `[in-place]` for in-place sessions |
| `app/app.go` | Modify | Add `KeyInPlace` handler, guard Kill/Push handlers |

---

## Task 1: Data Model — Instance and Storage

**Files:**
- Modify: `session/instance.go:32-72` (Instance struct), `session/instance.go:176-188` (InstanceOptions), `session/instance.go:190-210` (NewInstance)
- Modify: `session/storage.go:11-25` (InstanceData)
- Modify: `session/storage_test.go`

- [ ] **Step 1: Write serialization tests**

Add to `session/storage_test.go`:

```go
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
	// Worktree should be zero value
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
Expected: FAIL — `InPlace` field not found

- [ ] **Step 3: Add InPlace to InstanceData**

In `session/storage.go`, add to `InstanceData` struct after the `AutoYes` field:

```go
	InPlace   bool      `json:"in_place,omitempty"`
```

- [ ] **Step 4: Add inPlace to Instance struct**

In `session/instance.go`, add to `Instance` struct after `selectedSubmodules`:

```go
	// inPlace is true when this session runs directly in the working directory
	// without creating git branches or worktrees.
	inPlace bool
```

Add accessor methods after the existing `SetSelectedSubmodules`:

```go
// IsInPlace returns true if this is an in-place session (no git isolation).
func (i *Instance) IsInPlace() bool {
	return i.inPlace
}
```

The `inPlace` field is set after construction via `SetInPlace()` rather than through `InstanceOptions`, because the instance is created before the overlay is submitted (user may toggle the in-place checkbox on/off before submitting).

- [ ] **Step 5: Run tests**

Run: `go test ./session/ -run "TestInPlace" -v`
Expected: PASS

- [ ] **Step 6: Verify build**

Run: `go build ./...`
Expected: Success

- [ ] **Step 7: Commit**

```bash
git add session/instance.go session/storage.go session/storage_test.go
git commit -m "feat: add InPlace field to Instance and InstanceData"
```

---

## Task 2: Instance Lifecycle — Start

**Files:**
- Modify: `session/instance.go:235-315` (Start method)
- Modify: `session/instance.go:74-123` (ToInstanceData)
- Modify: `session/instance.go:126-173` (FromInstanceData)
- Modify: `session/instance.go:212-217` (RepoName)

- [ ] **Step 1: Modify Start() for in-place sessions**

In `session/instance.go`, in `Start()` (line 235), after the tmux session creation block (line 248), replace the `firstTimeSetup` block with:

```go
	if firstTimeSetup {
		if i.inPlace {
			// In-place: no worktree, read current branch for display
			branchCmd := exec.Command("git", "-C", i.Path, "branch", "--show-current")
			if out, err := branchCmd.Output(); err == nil {
				i.Branch = strings.TrimSpace(string(out))
			}
		} else if i.selectedBranch != "" {
			gitWorktree, err := git.NewGitWorktreeFromBranch(i.Path, i.selectedBranch, i.Title)
			if err != nil {
				return fmt.Errorf("failed to create git worktree from branch: %w", err)
			}
			i.gitWorktree = gitWorktree
			i.Branch = i.selectedBranch
		} else {
			gitWorktree, branchName, err := git.NewGitWorktree(i.Path, i.Title)
			if err != nil {
				return fmt.Errorf("failed to create git worktree: %w", err)
			}
			i.gitWorktree = gitWorktree
			i.Branch = branchName
		}
	}
```

Add `"os/exec"` to imports.

Then in the `firstTimeSetup` branch of the setup section (line 280-310), add in-place handling:

```go
	if !firstTimeSetup {
		if err := tmuxSession.Restore(); err != nil {
			setupErr = fmt.Errorf("failed to restore existing session: %w", err)
			return setupErr
		}
	} else if i.inPlace {
		// In-place: start tmux in the working directory directly
		if err := i.tmuxSession.Start(i.Path); err != nil {
			setupErr = fmt.Errorf("failed to start new session: %w", err)
			return setupErr
		}
	} else {
		// Normal: setup git worktree first
		if err := i.gitWorktree.Setup(); err != nil {
			setupErr = fmt.Errorf("failed to setup git worktree: %w", err)
			return setupErr
		}

		// Initialize selected submodules
		if len(i.selectedSubmodules) > 0 {
			if err := i.gitWorktree.InitSubmodules(i.Path, i.selectedSubmodules); err != nil {
				setupErr = fmt.Errorf("failed to init submodules: %w", err)
				return setupErr
			}
		}

		// Create new session
		if err := i.tmuxSession.Start(i.gitWorktree.GetWorktreePath()); err != nil {
			if cleanupErr := i.gitWorktree.Cleanup(); cleanupErr != nil {
				err = fmt.Errorf("%v (cleanup error: %v)", err, cleanupErr)
			}
			setupErr = fmt.Errorf("failed to start new session: %w", err)
			return setupErr
		}
	}
```

- [ ] **Step 2: Modify RepoName() for in-place sessions**

Replace the current `RepoName()` method:

```go
func (i *Instance) RepoName() (string, error) {
	if !i.started {
		return "", fmt.Errorf("cannot get repo name for instance that has not been started")
	}
	if i.inPlace {
		return filepath.Base(i.Path), nil
	}
	return i.gitWorktree.GetRepoName(), nil
}
```

- [ ] **Step 3: Modify ToInstanceData() for in-place sessions**

In `ToInstanceData()`, wrap the worktree and diff stats serialization blocks:

```go
func (i *Instance) ToInstanceData() InstanceData {
	data := InstanceData{
		Title:     i.Title,
		Path:      i.Path,
		Branch:    i.Branch,
		Status:    i.Status,
		Height:    i.Height,
		Width:     i.Width,
		CreatedAt: i.CreatedAt,
		UpdatedAt: time.Now(),
		Program:   i.Program,
		AutoYes:   i.AutoYes,
		InPlace:   i.inPlace,
	}

	if !i.inPlace && i.gitWorktree != nil {
		data.Worktree = GitWorktreeData{
			RepoPath:         i.gitWorktree.GetRepoPath(),
			WorktreePath:     i.gitWorktree.GetWorktreePath(),
			SessionName:      i.Title,
			BranchName:       i.gitWorktree.GetBranchName(),
			BaseCommitSHA:    i.gitWorktree.GetBaseCommitSHA(),
			IsExistingBranch: i.gitWorktree.IsExistingBranch(),
		}
		if i.gitWorktree.IsSubmoduleAware() {
			data.Worktree.IsSubmoduleAware = true
			for _, sw := range i.gitWorktree.GetSubmodules() {
				data.Worktree.Submodules = append(data.Worktree.Submodules, SubmoduleWorktreeData{
					SubmodulePath:    sw.GetSubmodulePath(),
					GitDir:           sw.GetGitDir(),
					WorktreePath:     sw.GetWorktreePath(),
					BranchName:       sw.GetBranchName(),
					BaseCommitSHA:    sw.GetBaseCommitSHA(),
					IsExistingBranch: sw.IsExistingBranch(),
				})
			}
		}
	}

	if !i.inPlace && i.diffStats != nil {
		data.DiffStats = DiffStatsData{
			Added:   i.diffStats.Added,
			Removed: i.diffStats.Removed,
			Content: i.diffStats.Content,
		}
	}

	return data
}
```

- [ ] **Step 4: Modify FromInstanceData() for in-place sessions**

Replace `FromInstanceData()`:

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
	}

	if !data.InPlace {
		instance.gitWorktree = git.NewGitWorktreeFromStorage(
			data.Worktree.RepoPath,
			data.Worktree.WorktreePath,
			data.Worktree.SessionName,
			data.Worktree.BranchName,
			data.Worktree.BaseCommitSHA,
			data.Worktree.IsExistingBranch,
		)
		instance.diffStats = &git.DiffStats{
			Added:   data.DiffStats.Added,
			Removed: data.DiffStats.Removed,
			Content: data.DiffStats.Content,
		}

		if data.Worktree.IsSubmoduleAware && len(data.Worktree.Submodules) > 0 {
			subs := make(map[string]*git.SubmoduleWorktree)
			for _, sd := range data.Worktree.Submodules {
				subs[sd.SubmodulePath] = git.NewSubmoduleWorktreeFromStorage(
					sd.SubmodulePath, sd.GitDir, sd.WorktreePath,
					sd.BranchName, sd.BaseCommitSHA, sd.IsExistingBranch,
				)
			}
			instance.gitWorktree.RestoreSubmodules(subs)
		}
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

- [ ] **Step 5: Modify GetGitWorktree() for in-place sessions**

```go
func (i *Instance) GetGitWorktree() (*git.GitWorktree, error) {
	if !i.started {
		return nil, fmt.Errorf("cannot get git worktree for instance that has not been started")
	}
	if i.inPlace {
		return nil, fmt.Errorf("in-place sessions do not have a git worktree")
	}
	return i.gitWorktree, nil
}
```

- [ ] **Step 6: Verify build**

Run: `go build ./...`
Expected: Success

- [ ] **Step 7: Commit**

```bash
git add session/instance.go
git commit -m "feat: in-place session Start, serialization, and RepoName"
```

---

## Task 3: Instance Lifecycle — Kill, Pause, Resume, UpdateDiffStats

**Files:**
- Modify: `session/instance.go:317-342` (Kill), `session/instance.go:453-521` (Pause), `session/instance.go:524-580` (Resume), `session/instance.go:587-633` (UpdateDiffStats)

- [ ] **Step 1: Modify Kill() for in-place sessions**

In `Kill()`, add after the `!i.started` check:

```go
	if i.inPlace {
		// In-place: just close tmux, no git cleanup
		if i.tmuxSession != nil {
			if err := i.tmuxSession.Close(); err != nil {
				return fmt.Errorf("failed to close tmux session: %w", err)
			}
		}
		return nil
	}
```

- [ ] **Step 2: Modify Pause() for in-place sessions**

In `Pause()`, add after the `i.Status == Paused` check (before line 461 which accesses `i.gitWorktree.IsSubmoduleAware()`):

```go
	if i.inPlace {
		// In-place: just detach tmux, no git operations
		if err := i.tmuxSession.DetachSafely(); err != nil {
			return fmt.Errorf("failed to detach tmux session: %w", err)
		}
		i.SetStatus(Paused)
		return nil
	}
```

- [ ] **Step 3: Modify Resume() for in-place sessions**

In `Resume()`, add after the `i.Status != Paused` check (before line 533 which accesses `i.gitWorktree.IsBranchCheckedOut()`):

```go
	if i.inPlace {
		// In-place: just restart tmux in the original directory
		if err := i.tmuxSession.Start(i.Path); err != nil {
			return fmt.Errorf("failed to restart in-place session: %w", err)
		}
		i.SetStatus(Running)
		return nil
	}
```

- [ ] **Step 4: Modify UpdateDiffStats() for in-place sessions**

In `UpdateDiffStats()`, add after the `i.Status == Paused` check (before line 598 which accesses `i.gitWorktree.IsSubmoduleAware()`):

```go
	if i.inPlace {
		i.diffStats = nil
		return nil
	}
```

- [ ] **Step 5: Verify build**

Run: `go build ./...`
Expected: Success

- [ ] **Step 6: Commit**

```bash
git add session/instance.go
git commit -m "feat: in-place session Kill, Pause, Resume, UpdateDiffStats"
```

---

## Task 4: TextInputOverlay — In-Place Toggle

**Files:**
- Modify: `ui/overlay/textInput.go`
- Create: `ui/overlay/textInput_test.go`

This task must come before the app handler changes because Task 5 references `IsInPlace()` and `SetInPlace()`.

The spec requires an interactive in-place toggle as the **first focus stop** in the overlay. When the toggle is on, branch and submodule pickers are hidden. The toggle appears in all overlay variants (`n`, `N`, and `i` all show it) — the `i` key just pre-selects it. The toggle is rendered as a checkbox at the top, toggled with `space` when focused.

- [ ] **Step 1: Write tests for the in-place toggle**

Create `ui/overlay/textInput_test.go`:

```go
package overlay

import (
	tea "github.com/charmbracelet/bubbletea"
	"testing"
)

func TestInPlaceToggle_DefaultOff(t *testing.T) {
	o := NewTextInputOverlayWithBranchPicker("Prompt", "", nil)
	if o.IsInPlace() {
		t.Error("expected in-place toggle to be off by default")
	}
}

func TestInPlaceToggle_SetInPlace(t *testing.T) {
	o := NewTextInputOverlayWithBranchPicker("Prompt", "", nil)

	o.SetInPlace(true)
	if !o.IsInPlace() {
		t.Error("expected in-place after SetInPlace(true)")
	}

	o.SetInPlace(false)
	if o.IsInPlace() {
		t.Error("expected not in-place after SetInPlace(false)")
	}
}

func TestInPlaceToggle_FocusOrderWithToggleOn(t *testing.T) {
	// No profile picker: inPlaceToggle(0) → textarea(1) → enterButton(2)
	o := NewTextInputOverlayWithBranchPicker("Prompt", "", nil)
	o.SetInPlace(true)
	// numStops = 3: toggle + textarea + enter
	if o.numStops != 3 {
		t.Errorf("expected 3 focus stops with toggle on (no profiles), got %d", o.numStops)
	}

	// First stop is the toggle
	if !o.isInPlaceToggle() {
		t.Error("expected first focus stop to be in-place toggle")
	}

	// Tab to textarea
	o.HandleKeyPress(tea.KeyMsg{Type: tea.KeyTab})
	if !o.isTextarea() {
		t.Error("expected textarea after tab from toggle")
	}

	// Tab to enter button
	o.HandleKeyPress(tea.KeyMsg{Type: tea.KeyTab})
	if !o.isEnterButton() {
		t.Error("expected enter button after tab from textarea")
	}
}

func TestInPlaceToggle_FocusOrderWithToggleOff(t *testing.T) {
	// No profile picker: inPlaceToggle(0) → textarea(1) → branchPicker(2) → enterButton(3)
	o := NewTextInputOverlayWithBranchPicker("Prompt", "", nil)
	// numStops = 4: toggle + textarea + branch + enter
	if o.numStops != 4 {
		t.Errorf("expected 4 focus stops with toggle off (no profiles), got %d", o.numStops)
	}

	if !o.isInPlaceToggle() {
		t.Error("expected first focus stop to be in-place toggle")
	}
}

func TestInPlaceToggle_SpaceToggles(t *testing.T) {
	o := NewTextInputOverlayWithBranchPicker("Prompt", "", nil)
	if o.IsInPlace() {
		t.Error("expected toggle off initially")
	}

	// Focus is on toggle (index 0), press space
	o.HandleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	if !o.IsInPlace() {
		t.Error("expected space to toggle in-place on")
	}

	// Press space again to toggle off
	o.HandleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	if o.IsInPlace() {
		t.Error("expected space to toggle in-place off")
	}
}

func TestInPlaceToggle_SpaceOnlyWorksWhenFocused(t *testing.T) {
	o := NewTextInputOverlayWithBranchPicker("Prompt", "", nil)

	// Tab to textarea (index 1)
	o.HandleKeyPress(tea.KeyMsg{Type: tea.KeyTab})
	if !o.isTextarea() {
		t.Error("expected textarea focus")
	}

	// Space on textarea should NOT toggle in-place
	o.HandleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	if o.IsInPlace() {
		t.Error("space on textarea should not toggle in-place")
	}
}

func TestInPlaceToggle_HidesBranchAndSubmodule(t *testing.T) {
	o := NewTextInputOverlayWithBranchPicker("Prompt", "", nil)
	if o.branchPicker == nil {
		t.Error("expected branch picker when toggle is off")
	}

	o.SetInPlace(true)
	// Branch picker still exists but is hidden from focus order
	if o.isBranchPicker() {
		t.Error("branch picker should not be focusable when in-place is on")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./ui/overlay/ -run "TestInPlaceToggle" -v`
Expected: FAIL — `isInPlaceToggle` not defined, `SetInPlace` not defined

- [ ] **Step 3: Add inPlace field and toggle infrastructure**

In `ui/overlay/textInput.go`, add the `inPlace` field to the `TextInputOverlay` struct (line 35-48):

```go
type TextInputOverlay struct {
	textarea        textarea.Model
	Title           string
	FocusIndex      int
	Submitted       bool
	Canceled        bool
	OnSubmit        func()
	width           int
	height          int
	profilePicker   *ProfilePicker
	branchPicker    *BranchPicker
	submodulePicker *SubmodulePicker
	numStops        int
	inPlace         bool // in-place toggle state
}
```

- [ ] **Step 4: Add toggle focus stop and helpers**

Add new focus helper:

```go
// isInPlaceToggle returns true if the current focus is on the in-place toggle.
// The toggle is always the first focus stop (index 0).
func (t *TextInputOverlay) isInPlaceToggle() bool {
	return t.FocusIndex == 0
}
```

**Update ALL existing focus helpers** to account for the toggle being at index 0. The toggle shifts all other indices by 1:

```go
func (t *TextInputOverlay) isProfilePicker() bool {
	if t.profilePicker == nil || !t.profilePicker.HasMultiple() {
		return false
	}
	return t.FocusIndex == 1 // toggle(0), profile(1)
}

func (t *TextInputOverlay) isTextarea() bool {
	offset := 1 // toggle is at 0
	if t.profilePicker != nil && t.profilePicker.HasMultiple() {
		offset = 2 // toggle(0), profile(1), textarea(2)
	}
	return t.FocusIndex == offset
}

func (t *TextInputOverlay) isBranchPicker() bool {
	if t.branchPicker == nil || t.inPlace {
		return false
	}
	offset := 2 // toggle(0), textarea(1), branch(2)
	if t.profilePicker != nil && t.profilePicker.HasMultiple() {
		offset = 3 // toggle(0), profile(1), textarea(2), branch(3)
	}
	return t.FocusIndex == offset
}

func (t *TextInputOverlay) isSubmodulePicker() bool {
	if t.submodulePicker == nil || t.inPlace {
		return false
	}
	offset := 3 // toggle(0), textarea(1), branch(2), submodule(3)
	if t.profilePicker != nil && t.profilePicker.HasMultiple() {
		offset = 4 // toggle(0), profile(1), textarea(2), branch(3), submodule(4)
	}
	return t.FocusIndex == offset
}
```

`isEnterButton()` remains unchanged (it uses `t.numStops - 1`).

- [ ] **Step 5: Add SetInPlace, IsInPlace, and recalcNumStops**

```go
// IsInPlace returns true if the in-place toggle is on.
func (t *TextInputOverlay) IsInPlace() bool {
	return t.inPlace
}

// SetInPlace sets the in-place toggle and recalculates focus order.
func (t *TextInputOverlay) SetInPlace(inPlace bool) {
	t.inPlace = inPlace
	t.recalcNumStops()
	t.FocusIndex = 0 // reset to toggle
	t.updateFocusState()
}

// recalcNumStops recalculates numStops based on current toggle state.
func (t *TextInputOverlay) recalcNumStops() {
	stops := 3 // toggle + textarea + enter button
	if t.profilePicker != nil && t.profilePicker.HasMultiple() {
		stops++
	}
	if !t.inPlace {
		if t.branchPicker != nil {
			stops++
		}
		if t.submodulePicker != nil {
			stops++
		}
	}
	t.numStops = stops
}
```

- [ ] **Step 6: Update constructors to include the toggle**

Update all constructors to include the toggle as a focus stop. The toggle adds 1 to `numStops`.

In `NewTextInputOverlay` (line 51-58):
```go
func NewTextInputOverlay(title string, initialValue string) *TextInputOverlay {
	ti := newTextarea(initialValue)
	return &TextInputOverlay{
		textarea: ti,
		Title:    title,
		numStops: 3, // toggle + textarea + enter button
	}
}
```

In `NewTextInputOverlayWithBranchPicker` (line 62-85), update `numStops`:
```go
	numStops := 4 // toggle + textarea + branch picker + enter button
	if pp != nil && pp.HasMultiple() {
		numStops = 5 // toggle + profile picker + textarea + branch picker + enter button
	}
```

In `NewTextInputOverlayWithSubmodules` (line 87-96), `numStops++` still works since it adds to the base.

- [ ] **Step 7: Add space handler for toggle in HandleKeyPress**

In `HandleKeyPress()` (line 213), add a `tea.KeySpace` case in the `default:` block (line 254), before the existing focus-specific handling:

```go
	default:
		// Space toggles in-place when toggle is focused
		if t.isInPlaceToggle() && msg.Type == tea.KeySpace {
			t.inPlace = !t.inPlace
			t.recalcNumStops()
			t.updateFocusState()
			return false, false
		}
		if t.isTextarea() {
			// ... existing code
```

Note: `tea.KeySpace` is not a named type — space is handled as a `tea.KeyRunes` with rune ' '. Check the bubbletea key types. The space key comes through as `msg.String() == " "` in the `default` branch. So the check should be:

```go
		if t.isInPlaceToggle() && msg.String() == " " {
```

- [ ] **Step 8: Add toggle rendering in Render()**

In the `Render()` method (line 351), add the toggle rendering at the very beginning of the content build, **before** the profile picker (line 368):

```go
	// Render in-place toggle
	toggleLabel := "[ ] In-place (no git isolation)"
	if t.inPlace {
		toggleLabel = "[x] In-place (no git isolation)"
	}
	if t.isInPlaceToggle() {
		content += lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true).Render(toggleLabel) + "\n\n"
	} else {
		content += lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(toggleLabel) + "\n\n"
	}
	content += divider + "\n\n"

	// Render profile picker if present, above the prompt
	if t.profilePicker != nil {
		// ... existing code
```

Also update the branch and submodule picker rendering to respect the in-place toggle:

```go
	// Render branch picker if present and not in-place
	if t.branchPicker != nil && !t.inPlace {
		content += divider + "\n\n"
		content += t.branchPicker.Render() + "\n\n"
	}

	// Render submodule picker if present and not in-place
	if t.submodulePicker != nil && !t.submodulePicker.IsEmpty() && !t.inPlace {
		content += divider + "\n\n"
		content += t.submodulePicker.Render() + "\n\n"
	}
```

- [ ] **Step 9: Run tests**

Run: `go test ./ui/overlay/ -run "TestInPlaceToggle" -v`
Expected: PASS

- [ ] **Step 10: Verify full build and tests**

Run: `go build ./... && go test ./... -timeout 120s`
Expected: All pass

- [ ] **Step 11: Commit**

```bash
git add ui/overlay/textInput.go ui/overlay/textInput_test.go
git commit -m "feat: add interactive in-place toggle to session creation overlay"
```

---

## Task 5: Keybindings and App Handlers

**Files:**
- Modify: `keys/keys.go`
- Modify: `app/app.go` — `handleKeyPress` switch (line 578), `KeyKill` case (line 646), `KeySubmit` case (line 685), `newPromptOverlay` (line 894), `statePrompt` submit (line 468)

- [ ] **Step 1: Add KeyInPlace to keys.go**

In `keys/keys.go`, add `KeyInPlace` to the iota block (after `KeyShiftDown`):

```go
	KeyInPlace
```

Add to `GlobalKeyStringsMap`:

```go
	"i":          KeyInPlace,
```

Add to `GlobalkeyBindings`:

```go
	KeyInPlace: key.NewBinding(
		key.WithKeys("i"),
		key.WithHelp("i", "in-place"),
	),
```

- [ ] **Step 2: Add KeyInPlace handler in app.go**

In `app/app.go`, add `inPlaceSession bool` field to the `home` struct (after `promptAfterName bool` at line 74):

```go
	// inPlaceSession is a transient flag set when 'i' is pressed, consumed when prompt submits
	inPlaceSession bool
```

In the `handleKeyPress` method's `switch name` block (line 578), add a new case after the `KeyPrompt` case (line 581-609). The `KeyInPlace` handler duplicates the `KeyPrompt` logic but sets the in-place flag:

```go
	case keys.KeyInPlace:
		if m.list.NumInstances() >= GlobalInstanceLimit {
			return m, m.handleError(
				fmt.Errorf("you can't create more than %d instances", GlobalInstanceLimit))
		}

		// No branch fetch needed for in-place sessions
		instance, err := session.NewInstance(session.InstanceOptions{
			Title:   "",
			Path:    ".",
			Program: m.program,
		})
		if err != nil {
			return m, m.handleError(err)
		}

		m.newInstanceFinalizer = m.list.AddInstance(instance)
		m.list.SetSelectedInstance(m.list.NumInstances() - 1)
		m.state = stateNew
		m.menu.SetState(ui.StateNewInstance)
		m.promptAfterName = true
		m.inPlaceSession = true

		return m, nil
```

Then in `newPromptOverlay()` (line 894), after creating the overlay, call `SetInPlace(true)` if `m.inPlaceSession` is set. This uses the overlay's interactive toggle rather than a separate constructor:

```go
func (m *home) newPromptOverlay() *overlay.TextInputOverlay {
	profiles := m.appConfig.GetProfiles()

	cwd, _ := os.Getwd()
	submodules, err := git.ListSubmodules(cwd)
	if err != nil {
		log.ErrorLog.Printf("failed to list submodules: %v", err)
	}

	var o *overlay.TextInputOverlay
	var subPaths []string
	for _, s := range submodules {
		subPaths = append(subPaths, s.Path)
	}
	if len(subPaths) > 0 {
		o = overlay.NewTextInputOverlayWithSubmodules("Enter prompt", "", profiles, subPaths)
	} else {
		o = overlay.NewTextInputOverlayWithBranchPicker("Enter prompt", "", profiles)
	}

	// Pre-select the in-place toggle if 'i' was pressed
	if m.inPlaceSession {
		o.SetInPlace(true)
	}

	return o
}
```

- [ ] **Step 3: Guard KeyKill handler**

In `app/app.go`, in the `KeyKill` case (line 646), modify the `killAction` closure. Replace the worktree/branch check at the top with:

```go
		killAction := func() tea.Msg {
			if !selected.IsInPlace() {
				// Only check branch checkout for normal sessions
				worktree, err := selected.GetGitWorktree()
				if err != nil {
					return err
				}
				checkedOut, err := worktree.IsBranchCheckedOut()
				if err != nil {
					return err
				}
				if checkedOut {
					return fmt.Errorf("instance %s is currently checked out", selected.Title)
				}
			}

			m.tabbedWindow.CleanupTerminalForInstance(selected.Title)
			if err := m.storage.DeleteInstance(selected.Title); err != nil {
				return err
			}
			m.list.Kill()
			return instanceChangedMsg{}
		}
```

- [ ] **Step 4: Guard KeySubmit (push) handler**

In `app/app.go`, in the `KeySubmit` case (line 685), add an early return at the top:

```go
	case keys.KeySubmit:
		selected := m.list.GetSelectedInstance()
		if selected == nil || selected.Status == session.Loading {
			return m, nil
		}
		if selected.IsInPlace() {
			return m, m.handleError(fmt.Errorf("push is not available for in-place sessions"))
		}
		// ... rest of existing push logic
```

- [ ] **Step 5: Wire in-place flag through statePrompt submit**

In the `statePrompt` submit handler (line 468-505), after `selected.SetSelectedSubmodules(...)` (line 483) and before `selected.Prompt = prompt` (line 485), read the toggle state from the overlay and reset the transient flag:

```go
					if m.textInputOverlay.IsInPlace() {
						selected.SetInPlace(true)
					}
					m.inPlaceSession = false // consume the transient flag
```

Add `SetInPlace` to `session/instance.go`:

```go
// SetInPlace marks this session as in-place (no git isolation).
func (i *Instance) SetInPlace(inPlace bool) {
	i.inPlace = inPlace
}
```

Also in `cancelPromptOverlay`, reset the transient flag:

```go
func (m *home) cancelPromptOverlay() tea.Cmd {
	m.inPlaceSession = false // reset transient flag
	// ... rest unchanged
```

- [ ] **Step 6: Verify build**

Run: `go build ./...`
Expected: Success (Task 4 overlay changes are already committed)

- [ ] **Step 7: Commit**

```bash
git add keys/keys.go app/app.go session/instance.go
git commit -m "feat: add KeyInPlace handler and guard Kill/Push for in-place sessions"
```

---

## Task 6: Session List Display

**Files:**
- Modify: `ui/list.go:117-226` (Render method)

- [ ] **Step 1: Show [in-place] for in-place sessions**

In `ui/list.go`, in the `Render()` method of `InstanceRenderer`, find where the `branch` variable is set (around line 187). Replace the branch assignment block:

```go
	branch := i.Branch
	if i.IsInPlace() {
		branch = "[in-place]"
	} else if i.Started() && hasMultipleRepos {
		repoName, err := i.RepoName()
		if err != nil {
			log.ErrorLog.Printf("could not get repo name in instance renderer: %v", err)
		} else {
			branch += fmt.Sprintf(" (%s)", repoName)
		}
	}
	// Show active submodules if any
	subPaths := i.GetActiveSubmodulePaths()
	if len(subPaths) > 0 {
		var names []string
		for _, path := range subPaths {
			parts := strings.Split(path, "/")
			names = append(names, parts[len(parts)-1])
		}
		branch += " [" + strings.Join(names, ",") + "]"
	}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add ui/list.go
git commit -m "feat: show [in-place] indicator in session list"
```

---

## Task 7: Integration Testing

**Files:**
- Modify: `session/storage_test.go`

The core lifecycle tests (Start, Kill, Pause, Resume) are difficult to unit test in isolation because they depend on tmux. However, we can test serialization round-trips and the guard logic.

- [ ] **Step 1: Add comprehensive serialization tests**

Add to `session/storage_test.go`:

```go
func TestInPlaceSessionSerialization_AllFields(t *testing.T) {
	data := InstanceData{
		Title:   "in-place-test",
		Path:    "/home/user/project",
		Branch:  "main",
		Status:  0, // Running
		InPlace: true,
		Program: "claude",
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Verify in_place is present in JSON
	jsonStr := string(jsonBytes)
	if !strings.Contains(jsonStr, `"in_place":true`) {
		t.Errorf("expected in_place:true in JSON, got: %s", jsonStr)
	}

	// Verify worktree is zero (empty fields not serialized with omitempty would still be present)
	if strings.Contains(jsonStr, `"repo_path":"/"`) {
		t.Error("in-place session should not have repo_path set")
	}

	var restored InstanceData
	if err := json.Unmarshal(jsonBytes, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !restored.InPlace {
		t.Error("expected InPlace to be true after round-trip")
	}
	if restored.Path != "/home/user/project" {
		t.Errorf("expected path preserved, got %q", restored.Path)
	}
	if restored.Branch != "main" {
		t.Errorf("expected branch preserved, got %q", restored.Branch)
	}
}

func TestInPlaceFromInstanceData_SkipsWorktreeConstruction(t *testing.T) {
	// Verify that FromInstanceData with InPlace=true does NOT construct a GitWorktree.
	// We can't call FromInstanceData directly (it starts tmux), but we can verify
	// the serialization round-trip preserves InPlace and has zero-value worktree.
	data := InstanceData{
		Title:   "in-place-from",
		Path:    "/tmp/test-project",
		Branch:  "feature",
		Status:  1, // Paused — avoids calling Start()
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
	// Worktree fields should be zero-value (not populated)
	if restored.Worktree.RepoPath != "" {
		t.Error("expected empty worktree RepoPath for in-place session")
	}
	if restored.Worktree.WorktreePath != "" {
		t.Error("expected empty worktree WorktreePath for in-place session")
	}
	if restored.Worktree.BranchName != "" {
		t.Error("expected empty worktree BranchName for in-place session")
	}
}
```

- [ ] **Step 2: Run all tests**

Run: `go test ./... -timeout 120s`
Expected: All pass

- [ ] **Step 3: Commit**

```bash
git add session/storage_test.go
git commit -m "test: add comprehensive in-place session tests"
```
