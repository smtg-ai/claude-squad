# Hivemind Integration Test Prompt

Paste everything below the line into an agent running inside Hivemind.

---

You are the **architect agent** in a Hivemind swarm. Your job is to test every Hivemind coordination feature by orchestrating a small team that makes real changes to this repository.

## Your mission

Create a tiny documentation site: a `docs/hivemind-guide/` directory with 3 markdown files, built by 3 different agents working in parallel, coordinated by you.

## Step-by-step instructions

Follow these steps **in order**. After each step, briefly report what happened.

### Phase 1: Register yourself

1. Call `update_status` with feature: "orchestrating hivemind test", files: "docs/hivemind-guide/", role: "architect".
2. Call `get_brain` to confirm you appear in the agent list.
3. Call `list_instances` to see all running instances.

### Phase 2: Define the workflow

4. Call `define_workflow` with this task DAG (as JSON):
```json
[
  {
    "id": "write-overview",
    "title": "Write overview guide",
    "prompt": "Create the file docs/hivemind-guide/01-overview.md with a brief (10-15 lines) overview of what Hivemind is: a TUI for managing multiple AI coding agents in parallel. Mention it uses Go, Bubble Tea, and tmux. Mention it supports Claude Code, Aider, Codex, and Amp as agent programs. When done, call complete_task with task_id 'write-overview' and status 'done'.",
    "role": "writer"
  },
  {
    "id": "write-workflow",
    "title": "Write workflow guide",
    "prompt": "Create the file docs/hivemind-guide/02-workflow.md explaining the Hivemind agent workflow in 10-15 lines: agents register with update_status, check get_brain for coordination, send messages to each other, can define workflow DAGs, and can subscribe to real-time events with wait_for_events. When done, call complete_task with task_id 'write-workflow' and status 'done'.",
    "role": "writer"
  },
  {
    "id": "write-index",
    "title": "Write index page",
    "depends_on": ["write-overview", "write-workflow"],
    "prompt": "Read docs/hivemind-guide/01-overview.md and docs/hivemind-guide/02-workflow.md, then create docs/hivemind-guide/README.md that serves as an index linking to both files with a one-line summary of each. When done, call complete_task with task_id 'write-index' and status 'done'.",
    "role": "writer"
  }
]
```

Report which tasks were triggered immediately (should be write-overview and write-workflow — they have no dependencies).

### Phase 3: Spawn worker agents

5. Call `create_instance` to spawn an agent named "writer-overview" with the prompt from the "write-overview" task and role "writer".
6. Call `create_instance` to spawn an agent named "writer-workflow" with the prompt from the "write-workflow" task and role "writer".

### Phase 4: Subscribe to events

7. Call `wait_for_events` with `types` set to "instance_status_changed,instance_created,task_completed" and `timeout` set to 5. This creates your event subscription and returns any events that have already happened (like the instance_created events from step 5-6). Note the `subscriber_id` in the response — you will reuse it for subsequent polls.

Report what events you received. You should see at least `instance_created` events for the two writers.

### Phase 5: Coordinate via messages

8. Call `send_message` to broadcast (leave "to" empty) with message: "Architect here. Two writers are starting on the overview and workflow guides. Please stay out of docs/hivemind-guide/ to avoid conflicts."
9. Call `send_message` to "writer-overview" with message: "Keep it concise — max 15 lines. Focus on what Hivemind is, not how to install it."
10. Call `get_brain` and confirm you can see the messages and any new agents.

### Phase 6: Test direct injection

11. Call `inject_message` targeting "writer-overview" with message: "Reminder from architect: make sure to mention that Hivemind supports Claude Code, Aider, Codex, and Amp as agent programs."

### Phase 7: Wait for writers to finish (event-driven)

Instead of polling in a loop, use the event subscription to wait for your sub-agents to complete.

12. Call `wait_for_events` with the `subscriber_id` from Phase 4, and `timeout` set to 25. This will block until events arrive or 25 seconds pass.
13. Check the returned events for `instance_status_changed` events with `"status": "ready"` — this means a writer agent has finished its work. Also look for `task_completed` events.
14. If both writers haven't finished yet, call `wait_for_events` again with the same `subscriber_id` and `timeout` 25. Repeat until you see both "writer-overview" and "writer-workflow" have reached "ready" status or their tasks show as completed.

Report each event as you receive it: which agent changed status, what the new status is.

### Phase 8: Dependent task triggers

Once both writer tasks are done:

15. Call `get_workflow` — the "write-index" task should now be triggered (status: running).
16. If it wasn't auto-triggered, call `create_instance` to spawn "writer-index" with the write-index prompt and role "writer".

### Phase 9: Wait for index writer (event-driven)

17. Call `wait_for_events` with the same `subscriber_id` and `timeout` 25. Wait for the "writer-index" agent to finish (look for `instance_status_changed` with status "ready" from the index writer, or `task_completed` for "write-index").

### Phase 10: Test pause/resume

18. If the index writer is still running, call `pause_instance` targeting "writer-index". If it already finished, spawn a temporary agent with `create_instance` (name: "pause-test", prompt: "Wait for instructions.") and pause that instead.
19. Call `list_instances` to confirm it shows as paused.
20. Wait 5 seconds, then call `resume_instance` on the same agent.
21. Call `list_instances` to confirm it's running again.

### Phase 11: Verify completion

22. Call `get_workflow` and confirm all 3 tasks show status "done".
23. Read the 3 files to verify they exist and have reasonable content:
    - `docs/hivemind-guide/01-overview.md`
    - `docs/hivemind-guide/02-workflow.md`
    - `docs/hivemind-guide/README.md`

### Phase 12: Clean up

24. Call `unsubscribe_events` with the `subscriber_id` from Phase 4 to clean up the event subscription.
25. Call `kill_instance` targeting "writer-overview".
26. Call `kill_instance` targeting "writer-workflow".
27. Call `kill_instance` targeting "writer-index" (if it was spawned).
28. Call `kill_instance` targeting "pause-test" (if it was spawned).
29. Call `list_instances` to confirm only you remain.

### Final report

Summarize what happened at each phase:
- Which Hivemind tools did you call and what were the results?
- Did the workflow DAG trigger the index task automatically when its dependencies completed?
- Did message sending and injection work?
- Did `wait_for_events` deliver real-time notifications? How many events did you receive and of what types?
- Did pause/resume work?
- Were there any errors or unexpected behaviors?
- How did the event-driven approach compare to polling? Did you receive events promptly when agents changed status?

This tests: `update_status`, `get_brain`, `list_instances`, `define_workflow`, `get_workflow`, `complete_task`, `create_instance`, `send_message`, `inject_message`, `wait_for_events`, `unsubscribe_events`, `pause_instance`, `resume_instance`, `kill_instance`.
