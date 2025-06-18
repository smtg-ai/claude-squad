# Claude Squad MCP Server

A Model Context Protocol (MCP) server that enables LLMs to interact directly with Claude Squad's Terminal User Interface (TUI). This allows AI agents to manage multiple Claude Code instances, monitor their progress, and provide real-time feedback.

## 🎯 Overview

This MCP server bridges the gap between LLMs and the Claude Squad TUI, similar to how Playwright MCP enables web automation. It provides:

- **Real TUI Interaction**: Direct keyboard simulation and screen parsing
- **Instance Management**: Create, monitor, and control AI agent instances  
- **Git Integration**: Monitor changes, diffs, and repository state
- **Multi-tab Support**: Access Preview, Diff, and Console tabs
- **Automated Workflows**: Pre-built patterns for common tasks

## 🚀 Quick Start

### Installation

```bash
cd mcp-server
pip install -r requirements.txt
```

### Running the MCP Server

```bash
python main.py
```

### Connect to Claude Desktop

Add to your Claude Desktop MCP configuration:

```json
{
  "mcpServers": {
    "claude-squad-tui": {
      "command": "python",
      "args": ["/path/to/claude-squad/mcp-server/main.py"]
    }
  }
}
```

## 🛠 Available Tools

### Instance Management
- `start_claude_squad` - Start the Claude Squad application
- `create_instance` - Create new AI agent instance
- `navigate_to_instance` - Select specific instance by index
- `kill_instance` - Delete an instance
- `checkout_instance` - Pause an instance
- `resume_instance` - Resume a paused instance

### Navigation & Interaction  
- `switch_tab` - Switch between Preview/Diff/Console tabs
- `scroll_content` - Scroll content up/down
- `navigate` - Send directional keys
- `send_key` - Send any key to the application
- `send_prompt` - Send prompt to current instance
- `push_changes` - Push git changes

## 📊 Available Resources

### Screen State
- `claude-squad://screen/current` - Complete UI state (JSON)
- `claude-squad://instances/list` - List of instances with status

### Tab Content
- `claude-squad://content/preview` - Preview tab content
- `claude-squad://content/diff` - Git diff content  
- `claude-squad://content/console` - Console output

## 💡 Example LLM Interactions

### Creating and Monitoring an Instance

```
LLM: I need to create a new instance to fix a bug in the authentication system.

Tool Call: create_instance
{
  "name": "fix-auth-bug",
  "with_prompt": true,
  "prompt": "Fix the authentication middleware bug that's causing login failures. Add proper error handling and unit tests."
}

Tool Call: switch_tab
{
  "tab": "diff"
}

Resource Read: claude-squad://content/diff
```

### Automated Code Review

```
LLM: Let me review the changes in the "fix-auth-bug" instance.

Tool Call: navigate_to_instance
{
  "index": 0
}

Tool Call: switch_tab  
{
  "tab": "diff"
}

Resource Read: claude-squad://content/diff

Tool Call: send_prompt
{
  "prompt": "The changes look good! The error handling is much improved. Please add a test case for the edge case where the token expires during the request."
}
```

### Batch Operations

```
LLM: I need to review all active instances and check their status.

Resource Read: claude-squad://instances/list

For each instance:
  Tool Call: navigate_to_instance
  Tool Call: switch_tab {"tab": "diff"}
  Resource Read: claude-squad://content/diff
  
Generate summary report...
```

## 🏗 Architecture

### Terminal Automation Core (`terminal_automator.py`)
- **pexpect**: Low-level terminal interaction
- **pyte**: Screen buffer parsing and ANSI escape code handling
- **Screen Parsing**: Intelligent extraction of UI elements
- **State Management**: Tracking UI changes and synchronization

### MCP Interface (`mcp_server.py`)
- **Tools**: Actions the LLM can perform
- **Resources**: Data the LLM can read
- **Error Handling**: Robust error recovery and reporting
- **Async Operations**: Non-blocking UI interactions

### Workflow Examples (`workflows.py`)
- **Instance Monitoring**: Track progress and status changes
- **Code Review**: Automated diff analysis and feedback
- **Batch Operations**: Multi-instance management
- **Error Recovery**: Handle common failure scenarios

## 🔧 Technical Details

### Screen Parsing Intelligence

The server parses Claude Squad's TUI layout:

```
┌─ Instance List (30%) ─┬─ Tabbed Content (70%) ─┐
│ ● Running instance-1  │ █ Preview │ Diff │ Console │
│ ⏸ Paused instance-2   │                          │
│ ● Ready instance-3    │ [Tab Content Area]       │
│   +5 -2 (main)        │                          │
└───────────────────────┴──────────────────────────┘
│ n:new │ D:kill │ tab:switch │ ?:help │
```

### Key Mapping

```python
key_mappings = {
    'up': '\x1b[A',           # Navigate up
    'down': '\x1b[B',         # Navigate down  
    'tab': '\t',              # Switch tabs
    'n': 'n',                 # New instance
    'D': 'D',                 # Delete instance
    'c': 'c',                 # Checkout
    'r': 'r',                 # Resume
    'p': 'p',                 # Push changes
    # ... and more
}
```

### State Synchronization

```python
async def _wait_for_ui_update(self, timeout: float = 2.0) -> bool:
    """Wait for UI to update after an action"""
    start_hash = self._get_screen_hash()
    
    while time.time() - start_time < timeout:
        await asyncio.sleep(0.1)
        if self._get_screen_hash() != start_hash:
            return True  # UI changed
    
    return False  # Timeout
```

## 🧪 Testing & Development

### Manual Testing

```bash
# Test terminal automation
python -c "
from claude_squad_mcp.terminal_automator import ClaudeSquadAutomator
import asyncio

async def test():
    automator = ClaudeSquadAutomator()
    await automator.start_claude_squad()
    state = automator.get_screen_content()
    print(f'Found {len(state.instances)} instances')
    await automator.stop()

asyncio.run(test())
"
```

### Workflow Testing

```bash
# Test example workflows
python claude_squad_mcp/workflows.py
```

## 📋 Requirements

- Python 3.8+
- Claude Squad installed and accessible as `cs` command
- tmux (required by Claude Squad)
- pexpect, pyte, mcp libraries

## 🔄 Workflow Patterns

### 1. Task Creation & Monitoring
```python
create_instance(name, prompt) 
→ monitor_progress() 
→ review_changes() 
→ provide_feedback()
```

### 2. Code Review Automation
```python
navigate_to_instance(target)
→ switch_to_diff_tab()
→ analyze_changes() 
→ send_review_comments()
```

### 3. Batch Management
```python
get_all_instances()
→ for_each_instance(operation)
→ generate_summary_report()
```

## 🚧 Limitations & Future Enhancements

### Current Limitations
- Terminal size dependency (120x30 default)
- Basic error recovery
- Limited tmux session interaction
- No visual element detection

### Planned Enhancements
- Dynamic terminal sizing
- Advanced error recovery
- Visual diff highlighting  
- Session recording/playback
- Performance metrics
- Multi-project support

## 📄 License

Same as Claude Squad project (AGPL-3.0)

---

This MCP server enables a new paradigm of AI-assisted development workflow management, bringing the power of LLM automation to terminal-based development tools.