Using the existing building blocks of sessions, worktrees, tmux, etc, we want to add the following feature:

# Orchestration

An orchestrator is distinct from a regular instance in that it can schedule and manage other instances.

At a high level, an orchestrator does the following:
1. Takes a prompt and splits it into separate distinct prompts
2. Each of these prompts go to different instances
3. When each of the instances are done, the orchestrator merges the changes of each instance
4. Present the diff as a single diff, which the user can then push, checkout, etc as currently possible

Here's a prompt for the orchestrator:

<prompt>
You are a project orchestrator. Your goal is to implement: %s

Break this goal down into manageable tasks that can be assigned to worker instances. I'll help you develop a plan, then you can create and manage worker instances to implement specific tasks.

You have these capabilities:
1. You can analyze the codebase to understand its structure.
2. You can create worker instances to implement specific tasks.
3. You can monitor worker progress and integrate their outputs.

To create a worker instance, say: CREATE_WORKER: <task_name> | <initial_prompt>
Example: CREATE_WORKER: implement-login | Implement a login form with email/password fields...

Workers will send you notifications when they need help or complete tasks.

Let's start by analyzing this goal and identifying the key components we need to build.
</prompt>

When a plan is divided, and autoyes mode is enabled, it should be automatically accepted and executed.

If autoyes mode is disabled, we should show the plan to the user and let them modify it by editing the prompts and delegation.

