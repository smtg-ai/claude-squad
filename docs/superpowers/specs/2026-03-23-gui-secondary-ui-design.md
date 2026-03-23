# Claude Squad GUI — Secondary UI Design Spec

## Overview

A native macOS GUI application for claude-squad, built with Fyne (pure Go GUI toolkit) and `fyne-io/terminal` for embedded terminal widgets. Replaces the current Bubble Tea TUI's limitations with a multi-pane, IDE-style interface that provides full terminal interactivity, mouse support, and a persistent session sidebar.

The GUI reuses the existing session management, git worktree, tmux, storage, and config layers. It is launched via `cs gui` as a separate subcommand, coexisting with the current TUI. The long-term goal is to replace the TUI once the GUI is mature.

## Motivation

The current Bubble Tea TUI has fundamental limitations:

- **Single-pane preview**: Only one session visible at a time; no side-by-side view.
- **Attach/detach ceremony**: Viewing a session requires attaching to tmux (full-screen takeover) and detaching with Ctrl+Q.
- **No native terminal feel**: The preview pane captures and renders tmux output — it's read-only, not interactive.
- **tmux/macOS friction**: tmux doesn't integrate well with native macOS terminal features (clipboard, mouse, scrollback).
- **Limited mouse support**: Keyboard-only navigation in most flows.

## Technology

- **Fyne v2** (`fyne.io/fyne/v2`): Pure Go cross-platform GUI toolkit. Custom-rendered via OpenGL. Dark theme built-in. Provides widgets for lists, split containers, buttons, dialogs, and layout management.
- **fyne-io/terminal** (`fyne.io/terminal`): Embedded terminal emulator widget for Fyne. VT100 emulation, mouse support, color rendering. Connects directly to a PTY.
- **Existing Go packages**: `session`, `session/git`, `session/tmux`, `config`, `session/storage` — all reused as-is with no modifications.

## Layout

### Sidebar (left, fixed width ~240px)

A persistent session list with two groups:

**Active group** (top):
- Contains sessions with status Running, Ready, or Loading.
- Sorted alphabetically by session name within the group.
- Each item shows: status icon, session name (bold), subtitle with status text and diff stats.
- Status icons: green circle (running), yellow triangle (needs input / alarm), spinner (loading).
- The "alarm" state is triggered when `TmuxSession.HasUpdated()` detects a prompt (session waiting for user input). Displayed as a yellow warning icon and yellow text.

**Paused group** (below Active, separated by a divider):
- Contains sessions with status Paused.
- Sorted alphabetically by session name.
- Dimmed text, gray pause icon.

**Sorting stability**: Items do not reorder within a group unless renamed. A session moves between groups only when its status changes (e.g., paused to active on resume). New sessions slot into alphabetical position in the Active group.

**Selection**: Clicking a session selects it (highlighted with accent border). The selected session's info is shown but it is not automatically opened in a pane.

**Bottom bar**: "+ New" button and "Settings" button.

**Toggling**: `Ctrl+Shift+B` hides/shows the sidebar.

### Main Pane Area (right, flexible)

One or more terminal panes arranged in a binary tree layout. Each leaf node is either:
- An **active terminal**: A `fyne-io/terminal` widget connected to a tmux session's PTY. Full keyboard, mouse, color, and scrollback support.
- An **empty pane**: Displays a prompt to select a session.

One pane is the **focused pane** at any time — it receives keyboard input and has a highlighted border (purple/accent color). Other panes continue rendering but don't receive input.

### Pane Header

Each pane has a thin header bar showing:
- Status icon
- Session name
- Branch name
- Hint text for split hotkeys (on the focused pane only)

## Pane Management

### Binary Tree Model

The pane layout is a binary tree. Each node is either:
- A **leaf**: A single terminal pane.
- A **split**: Horizontal or vertical, with two child nodes and a draggable divider.

Example:
```
Split(vertical)
+-- Leaf(auth-refactor)     <- focused
+-- Split(horizontal)
    +-- Leaf(api-tests)
    +-- Leaf(db-migration)
```

### Splitting

- `Ctrl+Shift+\` splits the focused pane vertically (side-by-side).
- `Ctrl+Shift+-` splits the focused pane horizontally (top/bottom).
- The split divides the focused pane 50/50. The new pane starts empty.
- Dividers are draggable to resize.

### Closing

- `Ctrl+Shift+W` closes the focused pane. The sibling expands to fill the space.
- Closing a pane does NOT kill the session — it keeps running in the background.
- Closing the last pane leaves a single empty pane (the app does not quit).

### Session-Pane Relationship

- A session can be open in multiple panes simultaneously (same PTY, same content).
- The sidebar indicates which sessions are currently visible in panes.

## Interaction Model

### Hotkeys

All UI hotkeys use `Ctrl+Shift+` prefix to avoid conflicts with terminal input.

| Action | Hotkey |
|--------|--------|
| New session | `Ctrl+Shift+N` |
| Split vertical | `Ctrl+Shift+\` |
| Split horizontal | `Ctrl+Shift+-` |
| Close pane | `Ctrl+Shift+W` |
| Navigate panes | `Ctrl+Shift+Arrow` |
| Navigate sessions (sidebar) | `Ctrl+Shift+J/K` |
| Open session in focused pane | `Ctrl+Shift+Enter` |
| Kill session | `Ctrl+Shift+D` |
| Push changes | `Ctrl+Shift+P` |
| Pause/Resume | `Ctrl+Shift+R` |
| Toggle sidebar | `Ctrl+Shift+B` |
| Quit | `Ctrl+Shift+Q` |

### Mouse

- Click session in sidebar: select it.
- Double-click session: open it in the focused pane.
- Click a pane: focus it.
- Drag split divider: resize panes.
- Right-click session: context menu (Kill, Push, Pause, Resume, Checkout).
- Mouse interactions within terminal panes (selection, scroll) pass through to `fyne-io/terminal`.

### Session Lifecycle

1. `Ctrl+Shift+N` or click "+ New" opens a dialog for session name, optional prompt, optional branch selection, and optional program/profile selection. Reuses existing `config.Config` profiles.
2. Session starts in the background. Appears in sidebar Active group with loading indicator.
3. Once running, open it in any pane via `Ctrl+Shift+Enter` or double-click.
4. A pane can be switched to a different session at any time.
5. Alarm state triggers when `HasUpdated()` detects a prompt — sidebar icon changes to yellow warning.

## Status Polling

Reuses the existing polling model:
- **Preview/terminal content**: Each `fyne-io/terminal` widget handles its own rendering via direct PTY connection. No polling needed for active terminals.
- **Session metadata** (status, diff stats, alarm): Polled every ~500ms using `TmuxSession.HasUpdated()` and `Instance.UpdateDiffStats()`, same as the current TUI.
- **Trust prompt handling**: `TmuxSession.CheckAndHandleTrustPrompt()` runs on the same polling interval.

## Theme

Dark IDE-style theme using Fyne's theming system:
- Background: dark charcoal (#1e1e2e)
- Sidebar: slightly darker (#181825)
- Borders/dividers: subtle gray (#313244)
- Accent: purple (#cba6f7)
- Text: light gray (#cdd6f4)
- Success/running: green (#a6e3a1)
- Warning/alarm: yellow (#f9e2af)
- Muted/paused: gray (#6c7086)

Inspired by the Catppuccin Mocha palette. Implemented as a custom Fyne `fyne.Theme`.

## Package Structure

```
claude-squad/
  gui/                        <- all new code
    app.go                    <- Fyne app setup, window creation, main loop
    sidebar/
      sidebar.go              <- sidebar container widget
      session_list.go         <- session list with grouping/sorting
    panes/
      manager.go              <- binary tree pane layout manager
      pane.go                 <- single pane (terminal widget + header)
      split.go                <- split container with draggable divider
    hotkeys.go                <- Ctrl+Shift hotkey handling
    theme.go                  <- dark IDE theme definition
    dialogs/
      new_session.go          <- new session dialog (name, prompt, branch, profile)
      confirm.go              <- confirmation dialog (kill, push)
  app/                        <- existing Bubble Tea UI (unchanged)
  session/                    <- existing (shared, unchanged)
  config/                     <- existing (shared, unchanged)
  cmd/
    cmd.go                    <- add "gui" subcommand
  main.go
```

## Launch & Coexistence

- `cs gui` starts the Fyne GUI application.
- `cs` (no args) continues to start the existing Bubble Tea TUI.
- Both share session storage, config, and tmux sessions. Switching between them is safe.
- Future: once the GUI is mature, `cs` defaults to GUI with `cs tui` for the legacy interface.

## Dependencies Added

- `fyne.io/fyne/v2` — GUI framework
- `fyne.io/terminal` — terminal emulator widget

## Build & Distribution

- `go build` produces a single binary (same as today).
- `fyne package` can optionally produce a macOS `.app` bundle.
- macOS only for v1. Cross-platform possible later since Fyne supports Linux and Windows.

## Out of Scope (v1)

- System notifications (macOS notification center) for alarm states
- Pane border color changes for alarm states
- Sidebar drag-to-resize
- Configurable hotkey bindings
- Diff/preview tabs (the terminal pane IS the session — diff can be viewed within Claude Code itself)
- Auto-layout (automatic pane arrangement)
