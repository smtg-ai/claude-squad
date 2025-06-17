# Workflow Management

This utility enables the BMAD orchestrator to manage and execute team workflows.

## Important: Dynamic Workflow Loading

The BMAD orchestrator MUST read the available workflows from the current team configuration's `workflows` field. Do not use hardcoded workflow lists. Each team bundle defines its own set of supported workflows based on the agents it includes.

**Critical Distinction**:

- When asked "what workflows are available?", show ONLY the workflows defined in the current team bundle's configuration
- Use `/agent-list` to show agents in the current bundle
- Use `/workflows` to show workflows in the current bundle, NOT any creation tasks

### Workflow Descriptions

When displaying workflows, use these descriptions based on the workflow ID:

- **greenfield-fullstack**: Build a new full-stack application from concept to development
- **brownfield-fullstack**: Enhance an existing full-stack application with new features
- **greenfield-service**: Build a new backend service or API from concept to development
- **brownfield-service**: Enhance an existing backend service or API
- **greenfield-ui**: Build a new frontend/UI application from concept to development
- **brownfield-ui**: Enhance an existing frontend/UI application

## Workflow Commands

### /workflows

Lists all available workflows for the current team. The available workflows are determined by the team configuration and may include workflows such as:

- greenfield-fullstack
- brownfield-fullstack
- greenfield-service
- brownfield-service
- greenfield-ui
- brownfield-ui

The actual list depends on which team bundle is loaded. When responding to this command, display the workflows that are configured in the current team's `workflows` field.

Example response format:

````text
Available workflows for [Team Name]:
1. [workflow-id] - [Brief description based on workflow type]
2. [workflow-id] - [Brief description based on workflow type]
[... etc. ...]

Use /workflow-start {number or id} to begin a workflow.
```text

### /workflow-start {workflow-id}

Starts a specific workflow and transitions to the first agent.

Example: `/workflow-start greenfield-fullstack`

### /workflow-status

Shows current workflow progress, completed artifacts, and next steps.

Example response:

```text
Current Workflow: Greenfield Full-Stack Development
Stage: Product Planning (2 of 6)
Completed:
  ✓ Discovery & Requirements
    - project-brief (completed by Mary)

In Progress:
  ⚡ Product Planning
    - Create PRD (John) - awaiting input

Next: Technical Architecture
```text

### /workflow-resume

Resumes a workflow from where it left off, useful when starting a new chat.

User can provide completed artifacts:

```text
User: /workflow-resume greenfield-fullstack
      I have completed: project-brief, PRD
BMad: I see you've completed Discovery and part of Product Planning.
      Based on the greenfield-fullstack workflow, the next step is:
      - UX Strategy with Sally (ux-expert)

      Would you like me to load Sally to continue?
```text

### /workflow-next

Shows the next recommended agent and action in the current workflow.

## Workflow Execution Flow

### 1. Starting a Workflow

When a workflow is started:

1. Load the workflow definition
2. Identify the first stage and step
3. Transition to the required agent
4. Provide context about expected inputs/outputs
5. Guide artifact creation

### 2. Stage Transitions

After each artifact is completed:

1. Mark the step as complete
2. Check transition conditions
3. If stage is complete, move to next stage
4. Load the appropriate agent
5. Pass relevant artifacts as context

### 3. Artifact Tracking

Track all created artifacts:

```yaml
workflow_state:
  current_workflow: greenfield-fullstack
  current_stage: planning
  current_step: 2
  artifacts:
    project-brief:
      status: completed
      created_by: analyst
      timestamp: 2024-01-15T10:30:00.000Z
    prd:
      status: in-progress
      created_by: pm
      started: 2024-01-15T11:00:00.000Z
````

### 4. Workflow Interruption Handling

When user returns after interruption:

1. Ask if continuing previous workflow
2. Request any completed artifacts
3. Analyze provided artifacts
4. Determine workflow position
5. Suggest next appropriate step

Example:

````text
User: I'm working on a new app. Here's my PRD and architecture doc.
BMad: I see you have a PRD and architecture document. Based on these artifacts,
      it looks like you're following the greenfield-fullstack workflow and have completed
      stages 1-3. The next recommended step would be:

      Stage 4: Validation & Refinement
      - Load Sarah (Product Owner) to validate all artifacts

      Would you like to continue with this workflow?
```text

## Workflow Context Passing

When transitioning between agents, pass:

1. Previous artifacts created
2. Current workflow stage
3. Expected outputs
4. Any decisions or constraints identified

Example transition:

```text
BMad: Great! John has completed the PRD. According to the greenfield-fullstack workflow,
      the next step is UX Strategy with Sally.

      /ux-expert

Sally: I see we're in the Product Planning stage of the greenfield-fullstack workflow.
       I have access to:
       - Project Brief from Mary
       - PRD from John

       Let's create the UX strategy and UI specifications. First, let me review
       the PRD to understand the features we're designing for...
```text

## Multi-Path Workflows

Some workflows may have multiple paths:

```yaml
conditional_paths:
  - condition: project_type == 'mobile'
    next_stage: mobile-specific-design
  - condition: project_type == 'web'
    next_stage: web-architecture
  - default: fullstack-architecture
````

Handle these by asking clarifying questions when needed.

## Workflow Best Practices

1. **Always show progress** - Users should know where they are
2. **Explain transitions** - Why moving to next agent
3. **Preserve context** - Pass relevant information forward
4. **Allow flexibility** - Users can skip or modify steps
5. **Track everything** - Maintain complete workflow state

## Integration with Agents

Each agent should be workflow-aware:

- Know which workflow is active
- Understand their role in the workflow
- Access previous artifacts
- Know expected outputs
- Guide toward workflow goals

This creates a seamless experience where the entire team works together toward the workflow's objectives.
