# Claude Squad User Guide

This guide provides detailed instructions and examples for using Claude Squad effectively.

## Table of Contents
- [Getting Started](#getting-started)
- [Basic Commands](#basic-commands)
- [Working with Sessions](#working-with-sessions)
- [Managing Projects](#managing-projects)
- [Advanced Features](#advanced-features)
- [Tips and Best Practices](#tips-and-best-practices)

## Getting Started

After [installing Claude Squad](README.md#installation), launch it by running:

```bash
cs
```

This opens the main interface where you can create and manage multiple Claude Code sessions.

## Basic Commands

### Creating a New Session

1. Press `n` to create a new session with the default assistant.
2. Press `N` to create a new session with an initial prompt.

Example:
```
# Create a new session and immediately prompt the assistant to:
N
"Please create a new function to validate email addresses"
```

### Navigating Between Sessions

- Use `↑/j` and `↓/k` keys to move between different sessions.
- Press `↵/o` to attach to the selected session.
- Press `ctrl-q` to detach from the current session and return to the menu.

### Viewing Changes

- Press `tab` to toggle between preview and diff views.
- Use `shift-↑` and `shift-↓` to scroll in the diff view.

## Working with Sessions

### Attaching to Sessions

When you attach to a session, you can interact with Claude Code directly. After providing instructions:

1. Claude will analyze your codebase
2. Make changes as requested
3. You can review these changes when you detach

### Auto-Accept Mode

For hands-free operation, use the auto-accept mode:

```bash
cs -y
```

This automatically accepts prompts for Claude Code and other assistants, allowing them to work without requiring manual confirmation for each action.

### Using Different AI Assistants

Claude Squad supports various AI assistants:

```bash
# Use Aider with a specific model
cs -p "aider --model ollama_chat/gemma3:1b"

# Use a different Claude model
cs -p "claude --model claude-3-sonnet-20240229"
```

## Managing Projects

### Committing and Pushing Changes

1. Make changes in a session
2. Press `s` to commit and push the branch to GitHub
3. Claude Squad will handle the commit message and push automatically

### Checking Out Changes

To commit changes and pause the session:

1. Press `c` to checkout
2. This will commit all changes and pause the session
3. You can resume it later by pressing `r`

### Reviewing Before Applying

One of Claude Squad's key features is the ability to review changes before applying them:

1. After the assistant makes changes, detach from the session
2. Use the diff view (`tab` key) to review all changes
3. If satisfied, you can commit with `s` or checkout with `c`
4. If not, reattach and provide further instructions

## Advanced Features

### Project Isolation with Git Worktrees

Claude Squad isolates each session using git worktrees, ensuring:

- Each task gets its own dedicated branch
- No conflicts between different tasks
- Changes can be reviewed independently

### Resuming Paused Sessions

To resume a previously paused session:

1. Navigate to the paused session using `↑/j` and `↓/k`
2. Press `r` to resume it
3. Continue your work where you left off

## Tips and Best Practices

1. **Descriptive Task Names**: When creating a new session, use descriptive names for better organization.
2. **Regular Checkouts**: Use the checkout feature (`c`) to save progress on long-running tasks.
3. **Review Changes**: Always review the diff before committing to ensure the changes match your expectations.
4. **Multiple Parallel Tasks**: Take advantage of multiple sessions to work on different aspects of your project simultaneously.
5. **Background Processing**: Let Claude work on complex tasks in the background while you focus on other aspects of your project.

## Troubleshooting

If you encounter issues:

- Run `cs debug` to check configuration paths and settings
- Use `cs reset` to reset all stored instances if you encounter persistent problems
- Check the Claude Squad repository for updates and known issues

For more information, see the [README](README.md) or visit the [GitHub repository](https://github.com/stmg-ai/claude-squad).