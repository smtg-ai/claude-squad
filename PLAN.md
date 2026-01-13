# Claude Squad - Feature Plan

This document outlines planned features for Claude Squad.

---

## Feature 1: Multi AI Support

### Overview
Replace the current `n` key behavior with a dialog that allows users to select which AI agent to spawn.

### Current Behavior
- `n` creates a new Claude session directly
- `N` creates a new Claude session with a prompt

### Proposed Behavior
- `n` opens an AI selection dialog
- User selects from available AI options:
  - Claude (default)
  - Gemini
  - Codex (OpenAI)
  - Copilot
  - Custom (allows entering a custom command)

### Implementation Notes
- Reuse existing overlay/dialog patterns from `ui/overlay/`
- The selection list should show only configured/available AIs
- After AI selection, proceed to the existing title input flow
- `N` (with prompt) should also show the AI selection dialog first

### Keys
- `n` - Open AI selection dialog → title input → create session
- `N` - Open AI selection dialog → title input → prompt input → create session

---

## Feature 2: Instance Status Indicator

### Overview
Visual indicator showing the git state of each instance's worktree, plus audio notification when AI completes work.

### Visual States
| State | Color | Description |
|-------|-------|-------------|
| No changes | Blue | Worktree is clean, matches the branch |
| Uncommitted changes | Red | There are uncommitted changes in the worktree |
| Pushed to origin | Green | Changes have been committed and pushed to remote |

### Proposed Implementation
**Option A: Border color on Instances panel**
- Change the border color of the Instances list panel based on the currently selected instance's state
- Updates as user navigates up/down through instances

**Option B: Per-instance indicator**
- Small colored dot/icon next to each instance name in the list
- All states visible at a glance without navigating

### Audio Notification (formerly Feature 5)
- Play a sound when an AI instance transitions from "Running" to "Ready" state
- Must work over SSH (use terminal bell `\a` or similar)
- Config option to enable/disable: `notification_sound: true/false`

### Implementation Notes
- Poll git status for each instance (already have diff stats polling)
- Check `git status --porcelain` for uncommitted changes
- Check if local branch is ahead/behind remote
- For sound: terminal bell character `\a` works over SSH

---

## Feature 3: Yolo Mode Selection

### Overview
Easier method to enable auto-yes/yolo mode when creating new instances.

### Current Behavior
- Yolo mode set via `-y` / `--autoyes` CLI flag (applies globally)
- Or configured in `~/.claude-squad/config.json`

### Proposed Behavior
- When selecting an AI from the dialog (Feature 1), use a modifier key to enable yolo mode:
  - `Enter` - Create instance in standard mode
  - `Shift+Enter` - Create instance in yolo mode

### UI Indication
- Show hint at bottom of AI selection dialog: `[Enter] Standard  [Shift+Enter] Yolo Mode`
- When yolo mode is selected, show indicator in title input (e.g., "[YOLO]" prefix or colored border)

### Implementation Notes
- Modify the AI selection overlay to capture `Shift+Enter`
- Pass `AutoYes: true` to instance creation when shift modifier detected
- Bubble Tea key handling: check `key.Shift` modifier on Enter key

---

## Feature 4: Git Commands Menu

### Overview
Quick access to common git operations for the selected instance's worktree.

### Proposed Key
- `g` - Open git commands menu (available, not currently in use)

### Git Operations Menu
| Option | Description |
|--------|-------------|
| Pull latest from main | Fetch and merge latest changes from main/master branch |
| Fetch all | Fetch all remotes |
| Checkout branch | Switch to a different existing branch |
| View branches | List all local and remote branches |
| Reset to HEAD | Discard uncommitted changes |

### Implementation Notes
- Create new overlay similar to help menu
- Execute git commands in the instance's worktree directory
- Show command output/result in a temporary overlay or the preview pane
- Handle errors gracefully (e.g., merge conflicts)

### Workflow Example
1. User selects an instance
2. Presses `g`
3. Git menu appears with options
4. User selects "Pull latest from main"
5. Command executes: `git fetch origin && git merge origin/main`
6. Result shown to user

---

## Feature 5: Submit PR

### Overview
Enhanced PR creation workflow with more control over PR details before submission.

### Current Behavior
- `p` commits changes, pushes to remote, and opens branch URL in browser
- No PR creation exists - user must manually create PR on GitHub

### Proposed Behavior
- `P` (Shift+P) opens a PR submission dialog with options:
  - **Title**: Editable, defaults to branch name or last commit message
  - **Description**: Multi-line text input for PR body
  - **Target branch**: Select base branch (defaults to main/master)
  - **Draft**: Toggle to create as draft PR
  - **Reviewers**: Optional, select from recent collaborators

### Workflow
1. User selects an instance with committed changes
2. Presses `P`
3. PR dialog appears with pre-filled defaults
4. User edits title/description as needed
5. Selects target branch (if not main)
6. Toggles draft mode if desired
7. Submits with `Enter`
8. PR URL displayed/copied to clipboard

### Implementation Notes
- Use `gh pr create` with flags: `--title`, `--body`, `--base`, `--draft`, `--reviewer`
- Create multi-step overlay or single form with tabs
- Pre-populate title from branch name: `feature/my-feature` → "My feature"
- Show diff stats in dialog for context
- Keep `p` for quick push (current behavior) vs `P` for detailed PR

### Keys
- `p` - Quick commit + push + auto PR (unchanged)
- `P` - Open PR submission dialog with options

---

## Summary of Key Bindings

### New/Modified Keys
| Key | Current | Proposed |
|-----|---------|----------|
| `n` | New session | AI selection dialog → new session |
| `N` | New session with prompt | AI selection dialog → new session with prompt |
| `g` | (unused) | Git commands menu |
| `P` | (unused) | PR submission dialog |

### Modifier Keys
| Key Combo | Action |
|-----------|--------|
| `Enter` (in AI dialog) | Create standard mode instance |
| `Shift+Enter` (in AI dialog) | Create yolo mode instance |

---

## Configuration Additions

```json
{
  "notification_sound": true,
  "available_ais": ["claude", "gemini", "codex", "copilot"]
}
```

---

## Implementation Priority

1. **Feature 1: Multi AI Support** - Foundation for other features
2. **Feature 3: Yolo Mode Selection** - Builds on Feature 1's dialog
3. **Feature 2: Instance Status Indicator** - Independent, can be parallel
4. **Feature 4: Git Commands Menu** - Independent, can be parallel
5. **Feature 5: Submit PR** - Independent, enhances existing `p` workflow

---

## Open Questions

- [ ] Should the AI selection remember the last used AI as default?
- [ ] For git operations, should we show a confirmation before destructive operations (reset)?
- [ ] Should there be keyboard shortcuts within the git menu (e.g., `p` for pull)?
- [ ] For PR submission, should we fetch and show a list of reviewers, or use free-text input?
- [ ] Should PR description support markdown preview?
