# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Building and Testing
- `go build` - Build the main binary
- `go test -v ./...` - Run all tests with verbose output
- `go test -v ./cmd/cmd_test` - Run specific test package
- `gofmt -w .` - Format Go code (linting)

### Development
- `go run main.go` - Run the application directly
- `go run main.go -p "aider --model ollama_chat/gemma3:1b"` - Run with custom program
- `go run main.go -y` - Run with auto-yes mode
- `go run main.go debug` - Print debug information and config paths
- `go run main.go reset` - Reset all stored instances and cleanup

### Web Development (in web/ directory)
- `npm run dev` - Start Next.js development server with turbopack
- `npm run build` - Build the Next.js application
- `npm run lint` - Run ESLint

## Architecture

Claude Squad is a terminal-based session manager for AI coding assistants. It uses a combination of tmux for terminal session management and git worktrees for isolated workspaces.

### Core Components

1. **Session Management** (`session/`)
   - `Instance` struct represents a running AI assistant session
   - Sessions have states: Running, Ready, Loading, Paused
   - Each session gets its own git worktree and tmux session

2. **Git Worktree Integration** (`session/git/`)
   - `GitWorktree` manages isolated git branches per session
   - Creates branches with configurable prefixes (default: agent-farmer-)
   - Supports pausing/resuming by removing/restoring worktrees

3. **Tmux Integration** (`session/tmux/`)
   - `TmuxSession` wraps tmux sessions for AI assistants
   - Supports multiple programs: Claude Code, Aider, Codex
   - Monitors session status and handles auto-yes mode

4. **Terminal UI** (`ui/`)
   - Built with Bubble Tea framework
   - Tabbed interface showing session list, preview, and diffs
   - Keyboard shortcuts for session management

5. **Application State** (`app/`)
   - Main application loop and state management
   - Handles user interactions and navigation
   - Coordinates between UI components and session management

### Key Features

- **Isolated Workspaces**: Each session runs in its own git worktree, preventing conflicts
- **Background Execution**: Sessions can run tasks in background with auto-accept mode
- **Session Persistence**: Sessions can be paused (preserving branch) and resumed later
- **Multi-Assistant Support**: Works with Claude Code, Aider, Codex and other local agents

### Keyboard Shortcuts

#### Session Management
- `n` - Create new session with AI-generated name (prompts for task description)
- `N` - Create new session with prompt (same as `n` - will be unified)
- `enter` / `o` - Open/enter selected session
- `D` - Delete selected session (with confirmation)
- `q` - Quit application

#### Prompt Input
When entering prompts for name generation or sending to AI assistants:
- `Ctrl+Enter` - Submit the prompt (recommended)
- `Tab` - Switch focus to "Enter" button, then `Enter` to submit
- `Esc` - Cancel and close the prompt dialog

#### Navigation
- `↑` / `k` - Navigate up in session list
- `↓` / `j` - Navigate down in session list
- `tab` - Switch between tabs (list view, preview, diffs)
- `?` - Show help screen

### Configuration

- Config stored in `~/.agent-farmer/` directory
- State persisted in JSON format
- Branch naming configurable via `BranchPrefix` setting
- Default program and auto-yes mode configurable

### AI Name Generation

The application can automatically generate meaningful session names based on your task description:

- **API Support**: Works with Anthropic (Claude) and OpenAI APIs via environment variables:
  - `ANTHROPIC_API_KEY` - For Claude models
  - `OPENAI_API_KEY` - For GPT models
- **Fallback Mode**: Works without API keys using rule-based name generation
- **Smart Features**: 
  - Detects ticket numbers (ABC-123, PROJ-456) and incorporates them
  - Identifies coding keywords and actions
  - Ensures names are under 32 characters and git-branch friendly

### Prerequisites

The application requires:
- tmux (for session management)
- gh (GitHub CLI, for git operations)
- Must be run from within a git repository