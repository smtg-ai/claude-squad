# Deep Agent Integration — Design Notes

## Context

We built the Brain Server IPC system (Tier 1-2): centralized state management via Unix socket, MCP tools for `list_instances`, `get_brain`, `update_status`, `send_message`, `get_my_session_summary`, `get_my_diff`. Agents can see each other, detect file conflicts, and exchange messages (polling-based).

This document captures the next evolution: deeper integration where agents become first-class participants in the Hivemind ecosystem.

## Bugs Fixed During Initial Implementation

1. **Stale shared worktree**: `StartInSharedWorktree` didn't recreate the worktree directory if it had been deleted. Fixed by checking `os.Stat` and calling `worktree.Setup()` if missing.
2. **Agent identity collision**: All agents in a shared worktree registered under the same MCP server name `"hivemind"`, causing the second agent to inherit the first agent's `HIVEMIND_INSTANCE_ID`. Fixed by using unique names: `hivemind-<instance-title>` (e.g., `hivemind-agent-1-mcp`, `hivemind-agent-2-mcp`).

## Verified Working (Tier 1-2)

- Both agents appear separately in `get_brain`
- `update_status` correctly detects file conflicts (e.g., both agents touching `main.go`)
- Broadcast and directed messages flow between agents
- `list_instances` and `get_brain` show accurate, distinct data per agent
- Brain server runs in TUI process, MCP servers connect via Unix socket with file-based fallback

---

## Proposed Features

### 1. Agents Spawning New Instances (Tier 3)

Add an MCP tool `create_instance` that sends a request through the brain server to the TUI. The TUI already has all the instance creation logic — it just needs an API entrypoint.

```
create_instance(title="code-reviewer", role="review", wait_for=["agent-1", "agent-2"])
```

The brain server relays this to the TUI process, which creates the instance in the same topic/worktree. The `wait_for` parameter enables dependency-based scheduling — the new instance only starts when the specified agents complete.

### 2. Direct Message Injection (Interrupts)

Instead of agents polling `get_brain` to discover messages, Hivemind can **inject text directly into an agent's terminal input**. Claude Code processes stdin, so we can use tmux `send-keys` to type a message into an agent's pane.

Example injection:
```
[HIVEMIND URGENT] Agent A says: "Stop working on auth.go — I'm refactoring the auth module"
```

This gives real-time coordination without polling delay. Two modes:
- **Urgent**: Injected immediately via `send-keys`, interrupting the agent's current work
- **Normal**: Stored in brain state, picked up on next `get_brain` call (current behavior)

The tmux `send-keys` infrastructure already exists in the codebase (`session/tmux/tmux_io.go`).

### 3. Role-Based Agents with Specialized Prompts

When creating an instance, pass a role that determines behavior. The role would be stored in brain state so other agents know each agent's purpose.

Possible roles:
- `coder` — writes implementation code
- `reviewer` — reviews PRs, runs after coders finish
- `architect` — plans and delegates, spawns other agents
- `tester` — writes and runs tests

The role could influence:
- The system prompt / MCP server instructions
- Which MCP tools are available (tier gating already exists)
- How the agent appears in `get_brain` / `list_instances`

### 4. Workflow Orchestration / DAG Execution

The brain server tracks a task DAG:

```
Agent A (implement feature) ──┐
                               ├──► Agent C (code review) ──► Agent D (merge)
Agent B (write tests) ────────┘
```

When A and B both call `update_status(status="done")`, the brain server automatically triggers C. This is a lightweight CI pipeline managed by the agents themselves.

Key concepts:
- Agents declare completion via `update_status` or a new `complete_task` tool
- Brain server evaluates the DAG and spawns waiting agents
- The architect agent can define the DAG via a `define_workflow` tool
- Failed agents can be retried or escalated

### 5. Agent Lifecycle Control

MCP tools for agents to manage each other:
- `create_instance` — spawn a new agent
- `pause_instance` — pause another agent
- `resume_instance` — resume a paused agent
- `kill_instance` — terminate an agent

These would be Tier 3 (write) tools, gated behind `HIVEMIND_TIER=3`.

---

## Architecture Evolution

```
Current (Tier 1-2):  Agent ──MCP──► Brain Server (read/write state)
                                         ↑ polling

Next (Tier 3):       Agent ──MCP──► Brain Server ──► TUI (create/kill instances)
                                         │
                                         ├──► tmux send-keys (direct message injection)
                                         │
                                         └──► Task DAG (workflow orchestration)
```

The brain server becomes the central nervous system — it already holds state in memory, has the Unix socket, and runs inside the TUI process with full access to instance management.

## Implementation Priority

1. **`create_instance` tool** — unlocks agent-driven spawning, the foundation for everything else
2. **Direct message injection** — real-time coordination without polling
3. **Role system** — specialized agents with different capabilities
4. **Workflow DAG** — automated orchestration of multi-step pipelines

## Key Files

| File | Role |
|------|------|
| `brain/protocol.go` | Shared IPC types and method constants |
| `brain/manager.go` | In-memory state manager (agents, messages, mutex) |
| `brain/server.go` | Unix socket listener, dispatches to Manager |
| `brain/client.go` | Socket client for MCP servers |
| `mcp/server.go` | MCP tool registration (tier-gated) |
| `mcp/tools.go` | MCP tool handlers |
| `mcp/brain_client.go` | BrainClient interface + file fallback |
| `session/mcp_config.go` | MCP registration with Claude Code (`claude mcp add`) |
| `session/instance_lifecycle.go` | Instance start/stop/resume lifecycle |
| `session/tmux/tmux_io.go` | Terminal I/O, send-keys infrastructure |
| `app/app.go` | TUI — starts brain server, manages instances |
| `app/app_actions.go` | Instance actions (kill, pause, agent cleanup) |
| `cmd/mcp-server/main.go` | MCP server entry point (socket vs file client) |
