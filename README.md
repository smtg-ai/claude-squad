# Orzbob [![GitHub Release](https://img.shields.io/github/v/release/carnivoroustoad/orzbob)](https://github.com/carnivoroustoad/orzbob/releases/latest)

Orzbob is a terminal app that helps you become a 100x engineer by managing multiple [Claude Code](https://github.com/anthropics/claude-code), [Codex](https://github.com/openai/codex) (and other local agents including [Aider](https://github.com/Aider-AI/aider)) in separate workspaces, allowing you to work on multiple tasks simultaneously.

🚀 [Visit our website](https://carnivoroustoad.github.io/orzbob/) for more information.

![Orzbob Screenshot](assets/screenshot.png)

### Highlights
- Complete tasks in the background (including yolo / auto-accept mode!)
- Manage instances and tasks in one terminal window
- Review changes before applying them, checkout changes before pushing them
- Each task gets its own isolated git workspace, so no conflicts

<br />

https://github.com/user-attachments/assets/aef18253-e58f-4525-9032-f5a3d66c975a

<br />

### Installation

The easiest way to install `orzbob` is by running the following command:

```bash
curl -fsSL https://raw.githubusercontent.com/carnivoroustoad/orzbob/main/install.sh | bash
```

This will install the `orz` binary to `~/.local/bin` and add it to your PATH. To install with a different name, use the `--name` flag:

```bash
curl -fsSL https://raw.githubusercontent.com/carnivoroustoad/orzbob/main/install.sh | bash -s -- --name <name>
```

Alternatively, you can also install `orzbob` by building from source or installing a [pre-built binary](https://github.com/carnivoroustoad/orzbob/releases).

### Prerequisites

- [tmux](https://github.com/tmux/tmux/wiki/Installing)
- [gh](https://cli.github.com/)

### Usage

```
Usage:
  orz [flags]
  orz [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  debug       Print debug information like config paths
  help        Help about any command
  reset       Reset all stored instances
  update      Check for and apply updates
  version     Print the version number of orzbob

Flags:
  -y, --autoyes          [experimental] If enabled, all instances will automatically accept prompts for claude code & aider
  -h, --help             help for orzbob
  -p, --program string   Program to run in new instances (e.g. 'aider --model ollama_chat/gemma3:1b')
```

Run the application with:

```bash
orz
```

<br />

<b>Using Orzbob with other AI assistants:</b>
- For [Codex](https://github.com/openai/codex): Set your API key with `export OPENAI_API_KEY=<your_key>`
- Launch with specific assistants:
   - Codex: `orz -p "codex"`
   - Aider: `orz -p "aider ..."`
- Make this the default, by modifying the config file (locate with `orz debug`)

<br />

#### Menu
The menu at the bottom of the screen shows available commands: 

##### Instance/Session Management
- `n` - Create a new session
- `N` - Create a new session with a prompt
- `D` - Kill (delete) the selected session
- `↑/j`, `↓/k` - Navigate between sessions

##### Actions
- `↵/o` - Attach to the selected session to reprompt
- `ctrl-q` - Detach from session
- `s` - Commit and push branch to github
- `c` - Checkout. Commits changes and pauses the session
- `r` - Resume a paused session
- `?` - Show help menu

##### Navigation
- `tab` - Switch between preview tab and diff tab
- `q` - Quit the application
- `shift-↓/↑` - scroll in diff view

### How It Works

1. **tmux** to create isolated terminal sessions for each agent
2. **git worktrees** to isolate codebases so each session works on its own branch
3. A simple TUI interface for easy navigation and management
4. **Auto-updates** to keep your installation current with the latest features

### Configuration

You can customize Orzbob's behavior by editing the config file (find its location with `orz debug`). Some notable options:

- `default_program`: Set your preferred AI assistant as default
- `enable_auto_update`: Enable or disable checking for updates on startup
- `auto_install_updates`: Automatically install updates without prompting

### License

[AGPL-3.0](LICENSE.md)
