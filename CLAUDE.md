# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Claude Squad is a terminal application that manages multiple AI coding assistants (Claude Code, Aider, Codex) in isolated workspaces using git worktrees and tmux sessions. Each assistant runs in complete isolation with its own branch and terminal session.

## Common Development Commands

```bash
# Build and run the application
go build -o cs
./cs

# Clean development environment (removes all worktrees, tmux sessions, and config)
./clean.sh

# Web development (Next.js frontend)
cd web/
npm run dev

# Testing
go test ./...
go test ./app -v
go test ./session/git -v
```

## Architecture

### Core Components

- **App Layer** (`app/`): Bubble Tea TUI with state management, supports max 10 concurrent instances
- **Session Management** (`session/`): Handles AI assistant instances with persistent storage and status tracking
- **Git Integration** (`session/git/`): Creates isolated worktrees per session with automatic branch management
- **Tmux Management** (`session/tmux/`): Provides terminal isolation with cross-platform support
- **UI Components** (`ui/`): List views, tabbed windows, overlays, and error handling
- **Configuration** (`config/`): Persistent user settings stored in `~/.claude-squad/`
- **Daemon** (`daemon/`): Background processing for AutoYes mode

### Key Architectural Patterns

- **Session Isolation**: Each AI assistant gets its own git worktree and tmux session
- **State Persistence**: Instances survive application restarts via JSON serialization  
- **Real-time Updates**: 500ms polling for live diff tracking and UI updates
- **Event-driven UI**: Bubble Tea framework for responsive terminal interface

### Important Files

- `main.go`: Entry point with Cobra CLI framework
- `app/app.go`: Main TUI application logic and state management
- `session/instance.go`: Core session abstraction
- `session/git/worktree.go`: Git worktree operations and branch management
- `session/tmux/tmux.go`: Terminal session management
- `config/config.go`: Configuration and state persistence

## Development Notes

### Dependencies

- Go 1.23.0+ required
- Uses Charm's Bubble Tea ecosystem for TUI
- Requires `tmux` and `gh` CLI tools for full functionality
- Web component uses Next.js with TypeScript

### Session Management

Instances have states: Running, Ready, Loading, Paused. Each session automatically:
- Creates a git worktree with user-prefixed branch name
- Spawns isolated tmux session
- Tracks diff statistics in real-time
- Persists state for resume/checkout functionality

### Configuration Location

All configuration and state stored in `~/.claude-squad/`:
- `config.json`: User preferences
- Instance state files for session persistence
- Git worktree storage