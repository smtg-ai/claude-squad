# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Claude Squad is a terminal multiplexer for AI coding assistants written in Go. It manages multiple instances of Claude Code, Aider, Codex, and Gemini in isolated git worktrees with separate tmux sessions, allowing parallel task execution.

## Development Commands

### Building and Testing

```bash
# Build the binary
go build -v -o build/claude-squad

# Run tests
go test -v ./...

# Run tests for a specific package
go test -v ./session/...
go test -v ./session/git/...
go test -v ./ui/...

# Build for specific platforms (as per CI)
GOOS=linux GOARCH=amd64 go build -v -o build/linux_amd64/claude-squad
GOOS=darwin GOARCH=arm64 go build -v -o build/darwin_arm64/claude-squad
```

### Running the Application

```bash
# Run directly
go run main.go

# Run with specific program
go run main.go -p "aider --model ollama_chat/gemma3:1b"

# Run with auto-yes mode (experimental)
go run main.go -y

# Show debug information
go run main.go debug

# Reset all stored instances
go run main.go reset
```

## Architecture

### Core Components

**Instance Management** (`session/instance.go`, `session/storage.go`):
- `Instance` is the central entity representing a running AI assistant session
- Each instance has: title, git worktree, tmux session, branch, status (Running/Ready/Loading/Paused)
- Instances can be paused (commits changes, removes worktree, keeps branch) and resumed
- Storage handles serialization/deserialization of instances between runs

**Git Worktree Integration** (`session/git/`):
- Each instance gets an isolated git worktree in `~/.config/claude-squad/worktrees/`
- Worktrees are created from the current repo with unique branches (prefix + sanitized session name)
- Operations: Setup, Cleanup, Remove, Prune, IsDirty, CommitChanges, PushChanges
- Diff tracking compares current state against base commit SHA

**Tmux Session Management** (`session/tmux/tmux.go`):
- Each instance runs in a dedicated tmux session prefixed with `claudesquad_`
- PTY-based attachment enables resizing and input/output streaming
- StatusMonitor tracks content changes using SHA256 hashing to detect when AI is working vs. waiting
- Supports Claude, Aider, and Gemini with auto-detection of trust prompts
- Mouse scrolling and history are enabled (10000 line limit)

**UI Layer** (`app/app.go`, `ui/`):
- Built with Bubble Tea TUI framework
- Three-pane layout: List (30%) | Preview/Diff tabs (70%)
- States: stateDefault, stateNew, statePrompt, stateHelp, stateConfirm
- Key components: List, Menu, TabbedWindow (Preview + Diff), ErrBox, Overlays
- Preview pane shows live tmux output; Diff pane shows git changes

**Configuration** (`config/`):
- Config stored in `~/.config/claude-squad/config.json`
- State (instances) stored in `~/.config/claude-squad/state.json`
- Configurable: DefaultProgram, BranchPrefix, AutoYes

### Key Workflows

**Creating a New Instance**:
1. User presses `n` or `N` (with prompt)
2. Instance created in memory (not started)
3. User enters title
4. Start() creates git worktree and tmux session
5. Instance saved to storage
6. UI switches to default state, shows help screen

**Pausing/Resuming**:
- Pause (`c`): Commits changes, removes worktree, kills tmux, sets status to Paused
- Resume (`r`): Recreates worktree, restarts tmux session (or restores if still exists)
- Branch and commit history are preserved

**Attaching to Instance**:
- User presses Enter on selected instance
- Switches to raw terminal mode, streams tmux I/O
- Ctrl-Q to detach (not Ctrl-D, which kills the session)

**AutoYes Mode**:
- Daemon process monitors instances and auto-presses Enter on prompts
- Identified by detecting prompt strings (e.g., "No, and tell Claude what to do differently")

## Important Implementation Details

### Instance Lifecycle
- `Instance.Start(firstTimeSetup bool)` handles both new instances and loading from storage
- Always cleanup resources (worktree, tmux) in defer blocks with error accumulation
- `started` flag prevents operations on uninitialized instances

### Tmux PTY Management
- Each tmux session requires a PTY (`ptmx`) for sizing control
- On Attach: creates goroutines for I/O streaming and window size monitoring
- On Detach: must close PTY, restore new one, cancel goroutines, wait for cleanup
- DetachSafely vs. Detach: Safely version doesn't panic, used in Pause operation

### Git Worktree Naming
- Worktrees stored in config dir, not in repo, to avoid cluttering user's workspace
- Names: `<sanitized_title>_<hex_timestamp>` for uniqueness
- Branch names: `<configurable_prefix><sanitized_title>`
- Always use absolute paths for reliability

### Testing Patterns
- Dependency injection for testability: `PtyFactory`, `cmd.Executor`
- Test constructors: `NewTmuxSessionWithDeps`, `Instance.SetTmuxSession`
- Mock git operations using test repos in temp directories

### Concurrency
- Preview updates every 100ms (previewTickMsg)
- Metadata updates every 500ms (tickUpdateMetadataMessage) for status/diff
- All UI updates go through Bubble Tea's message loop

## Common Gotchas

1. **Sanitization**: Session names are sanitized (spaces removed, dots replaced with underscores) before use in tmux
2. **Exact Match**: Use `tmux has-session -t=name` (with `=`) for exact matching, not prefix matching
3. **PTY Cleanup**: Always close and restore PTY after operations; never leave `t.ptmx` as nil after Start/Restore
4. **Context Cancellation**: Attach goroutines must respect context for clean shutdown
5. **Storage Sync**: Call `storage.SaveInstances()` after state changes (new instance, delete, pause)
6. **Branch Checkout**: Cannot resume if branch is checked out elsewhere
7. **History Capture**: Use `-S - -E -` for full scrollback history in tmux

## Prerequisites

- tmux
- gh (GitHub CLI)
- git (with worktree support)
- Go 1.23+

## Configuration Location

Use `cs debug` (or `go run main.go debug`) to find config paths.
