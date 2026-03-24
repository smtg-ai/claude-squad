# GUI Session Creation — Branch Picker & In-Place Mode

**Date:** 2026-03-24
**Goal:** Add branch selection and in-place session toggle to the GUI's new session dialog, achieving parity with (and extending) the TUI's session creation flow.
**Related:** `docs/superpowers/plans/2026-03-23-in-place-sessions.md` — broader in-place sessions plan. This spec implements the core in-place data model and adds GUI-specific branch picker + in-place toggle.

---

## 1. Data Model

### Instance struct (`session/instance.go`)

Add `inPlace bool` field after `selectedBranch`. Add `IsInPlace() bool` accessor.

### InstanceOptions (`session/instance.go`)

Add `InPlace bool` field. Wire it through in `NewInstance()` to set `i.inPlace`.

### InstanceData (`session/storage.go`)

Add `InPlace bool` with tag `json:"in_place,omitempty"` for backward-compatible serialization. Old sessions without this field default to `false`.

### ToInstanceData

Set `data.InPlace = i.inPlace`. Existing nil guard on `gitWorktree` already handles in-place sessions (worktree data will be zero-valued).

### FromInstanceData

When `data.InPlace` is true, skip `NewGitWorktreeFromStorage` — leave `gitWorktree` as nil. This prevents constructing a `GitWorktree` with empty/invalid paths from zero-valued worktree data.

---

## 2. Git — Default Branch Detection & Fetch

### New function: `GetDefaultBranch(repoPath string) string`

Location: `session/git/worktree_git.go`

1. Try `git symbolic-ref refs/remotes/origin/HEAD` — parse out the branch name (strip `origin/` prefix)
2. If that fails, fall back to `git branch --show-current`
3. If both fail (e.g., not a git repo), return `"main"` as a sensible default
4. Return the branch name (e.g., `"main"`)

### Modified `Start()` behavior (`session/instance.go`)

When `firstTimeSetup` is true:

**In-place path (`inPlace == true`):**
- Skip all worktree creation — `gitWorktree` stays nil
- Skip `gitWorktree.Setup()` call
- Call `tmuxSession.Start(i.Path)` directly (use the working directory, not a worktree path)
- Set `i.Branch` to the current branch of the working directory (via `git branch --show-current`), or leave empty if not a git repo. This gives the sidebar something to display alongside the `[in-place]` label.

**Normal path (`inPlace == false`):**
- **No selected branch (default path):** Run `git fetch origin` (best-effort, ignore errors like `FetchBranches` does) first, then create the worktree based on `origin/<defaultBranch>` instead of local HEAD. This ensures the new session branch starts from the latest remote state. Add a new function `NewGitWorktreeFromRef(repoPath, ref, sessionTitle string)` that creates a worktree with a new branch based on the given ref (e.g., `git worktree add <path> -b <branch> <ref>`). This keeps `NewGitWorktree` (from HEAD) and `NewGitWorktreeFromBranch` (existing branch) unchanged.
- **Existing branch selected:** Current behavior unchanged — check out that branch into a worktree, no fetch.

**`!firstTimeSetup` path (loading from storage):**
- In-place sessions: `tmuxSession.Restore()` works as-is (no worktree dependency on that path). The `FromInstanceData` guard (Section 1) ensures `gitWorktree` is nil, so no worktree-related code runs.

---

## 3. GUI Dialog

### SessionOptions (`gui/dialogs/new_session.go`)

Add two fields:

```go
type SessionOptions struct {
    Name    string
    Prompt  string
    Program string
    Branch  string // empty = new branch from default
    InPlace bool
}
```

### Dialog layout (top to bottom)

1. **Name** — text entry (existing)
2. **In-place** — checkbox, unchecked by default
3. **Branch** — searchable select, hidden when in-place is checked
4. **Prompt** — multi-line entry (existing)
5. **Program** — dropdown, shown only when multiple profiles (existing)

Resize the dialog to accommodate the new fields (current size is 500x350; increase height as needed).

### Branch picker behavior

- On dialog open, call `git.GetDefaultBranch(".")` to get the default branch name and `git.SearchBranches(".", "")` to get the initial branch list.
- If both calls fail (not a git repo), hide the branch picker and default to in-place mode.
- First option: `"New branch (from <default>)"` — this is the default selection.
- Remaining options: existing branches from `SearchBranches`.
- As user types in the search/filter field, re-call `SearchBranches` with the filter text and update the list.
- Selecting an existing branch sets `SessionOptions.Branch` to that branch name.
- Selecting the "New branch" option leaves `Branch` empty.

### In-place toggle behavior

- When checked: hide the branch picker, set `SessionOptions.InPlace = true`.
- When unchecked: show the branch picker, `InPlace = false`.

### showNewSessionDialog (`gui/app.go`)

Wire new options to the Instance:

- If `opts.InPlace` is true, pass `InPlace: true` in `InstanceOptions`.
- If `opts.Branch` is non-empty, call `inst.SetSelectedBranch(opts.Branch)`.

---

## 4. Lifecycle Guards for In-Place Sessions

### Start (`session/instance.go`)

See Section 2 for the full in-place `Start()` flow. Key point: when `inPlace && firstTimeSetup`, skip worktree creation/setup and pass `i.Path` to `tmuxSession.Start()`.

### Kill (`session/instance.go`)

Already handles nil `gitWorktree` — no changes needed.

### Pause (`session/instance.go`)

Add guard: if `inPlace`, skip the entire dirty-check/commit/worktree-removal block (lines 426-465) AND the clipboard write of the branch name (line 473, would nil-deref). Go straight to tmux detach, then set status to Paused.

### Resume (`session/instance.go`)

Add guard: if `inPlace`, skip branch-checked-out check and worktree setup (lines 487-498). Go straight to tmux restore, passing `i.Path` as the working directory.

### UpdateDiffStats

If `inPlace` (or `gitWorktree == nil`), skip — leave `diffStats` nil.

### RepoName

Replace the current logic: if `gitWorktree` is nil, return `filepath.Base(i.Path)` instead of calling `gitWorktree.GetRepoName()`. The nil check must come before the worktree method call.

### GetGitWorktree accessor

Returns `i.gitWorktree` which may be nil for in-place sessions. All callers must nil-check the result. Specific call sites that need guards:

- **TUI kill handler** (`app/app.go:659`) — calls `GetGitWorktree()` then `worktree.IsBranchCheckedOut()`. Add nil check; skip the branch-checked-out warning for in-place sessions.
- **TUI push handler** (`app/app.go:699`) — calls `GetGitWorktree()` then `worktree.PushChanges()`. Add `IsInPlace()` guard to block push.
- **GUI `pushSession`** (`gui/app.go:376`) — calls `GetGitWorktree()` then `worktree.PushChanges()`. Add nil check as defense-in-depth even if the UI hides the button.
- **Daemon `UpdateDiffStats`** (`daemon/daemon.go:53`) — calls `instance.UpdateDiffStats()`. The nil guard inside `UpdateDiffStats` itself (see above) protects all callers including the daemon.

### Push/Commit operations

Block for in-place sessions — there's no isolated branch to push. The GUI should hide/disable the "Push" context action for in-place sessions. The TUI should check `IsInPlace()` before attempting push. Both should also have runtime nil guards as defense-in-depth.

### GUI indicators

The sidebar should show an `[in-place]` label or similar visual indicator for in-place sessions.

---

## 5. Testing

- **Serialization:** Round-trip `InPlace` through JSON marshal/unmarshal. Verify backward compatibility (old JSON without `in_place` defaults to false).
- **FromInstanceData:** Verify that `data.InPlace == true` results in nil `gitWorktree` (not a GitWorktree with empty paths).
- **GetDefaultBranch:** Test the fallback chain (symbolic-ref succeeds, symbolic-ref fails and falls back to current branch, both fail and return "main").
- **Lifecycle guards:** Test that Start (firstTimeSetup + inPlace), Pause, Resume, and Kill don't panic when `gitWorktree` is nil.
- **Start with fetch:** Test that `git fetch origin` is called when using the default branch path.
- **Start in-place:** Test that `tmuxSession.Start(i.Path)` is called (not `gitWorktree.GetWorktreePath()`) when `inPlace` is true.
