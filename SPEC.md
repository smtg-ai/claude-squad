Using the existing building blocks of sessions, worktrees, tmux, etc, we want to add the following feature:

# Orchestration

An orchestrator is distinct from a regular instance in that it can schedule and manage other instances.

At a high level, an orchestrator does the following:
1. Receives a prompt from the user as input
2. Devises a plan to implement the prompt and surfaces it to the user for approval/revision
3. Breaks down the plan into distinct tasks
4. Creates worker instances to implement the tasks
5. Gets notifications from the worker instances when they need help or complete tasks
6. When each of the instances are done, the orchestrator merges the changes of each instance
7. Present the diff as a single diff, which the user can then push, checkout, etc as currently possible

### Implementation Details

We should be able to capture the output of the orchestrator's plan and surface it in the text input overlay.

Once the user accepts the plan, the orchestrator should create and manage worker instances to implement the plan. We need to be able to retrieve the commands of the orchestrator and translate them into discrete commands to create/manage worker instances.

Here's a prompt for the orchestrator once it's plan has been accepted:

<prompt>
You are a project orchestrator. Your goal is to implement: %s

Break this goal down into manageable tasks that can be assigned to worker instances. I'll help you develop a plan, then you can create and manage worker instances to implement specific tasks.

You have these additional capabilities:
1. You can create worker instances to implement specific tasks.
2. You will be notified when a worker instance needs help or completes a task.

To create a worker instance, respond with:

<cs command>

</cs command>

</prompt>

### UI

The UI should be similar to the current UI, the user should be able to use 'o' to create an orchestrator instance, this should open the prompt overlay to send to the orchestrator.

For any worker instance (spawned by an orchestrator), it should be shown in the list of instances but indented to show that it is a child of the orchestrator.
