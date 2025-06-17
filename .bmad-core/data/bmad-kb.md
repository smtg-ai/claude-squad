# BMAD Knowledge Base

## Overview

BMAD-METHOD (Breakthrough Method of Agile AI-driven Development) is a framework that combines AI agents with Agile development methodologies. The v4 system introduces a modular architecture with improved dependency management, bundle optimization, and support for both web and IDE environments.

### Key Features

- **Modular Agent System**: Specialized AI agents for each Agile role
- **Build System**: Automated dependency resolution and optimization
- **Dual Environment Support**: Optimized for both web UIs and IDEs
- **Reusable Resources**: Portable templates, tasks, and checklists
- **Slash Command Integration**: Quick agent switching and control

## Core Philosophy

### Vibe CEO'ing

You are the "Vibe CEO" - thinking like a CEO with unlimited resources and a singular vision. Your AI agents are your high-powered team, and your role is to:

- **Direct**: Provide clear instructions and objectives
- **Refine**: Iterate on outputs to achieve quality
- **Oversee**: Maintain strategic alignment across all agents

### Core Principles

1. **MAXIMIZE_AI_LEVERAGE**: Push the AI to deliver more. Challenge outputs and iterate.
2. **QUALITY_CONTROL**: You are the ultimate arbiter of quality. Review all outputs.
3. **STRATEGIC_OVERSIGHT**: Maintain the high-level vision and ensure alignment.
4. **ITERATIVE_REFINEMENT**: Expect to revisit steps. This is not a linear process.
5. **CLEAR_INSTRUCTIONS**: Precise requests lead to better outputs.
6. **DOCUMENTATION_IS_KEY**: Good inputs (briefs, PRDs) lead to good outputs.
7. **START_SMALL_SCALE_FAST**: Test concepts, then expand.
8. **EMBRACE_THE_CHAOS**: Adapt and overcome challenges.

## IDE Development Workflow

1. Shard the PRD (And Architecture documents if they exist also based on workflow type) using the Doc Shard task. The BMad-Master agent can help you do this. You will select the task, provide the doc to shard and the output folder. for example: `BMad Master, please Shard the docs/prd.md to the doc/prd/ folder` - this should ask you to use the md-tree-parser which is recommended, but either way shoudl result in multiple documents being created in the folder docs/prd.
2. If you have fullstack, front end and or back end architecture documents you will want to follow the same thing, but shard all of these to an architecture folder instead of a prd folder.
3. Ensure that you have at least one epic-n.md file in your prd folder, with the stories in order to develop.
4. The docs or architecture folder or prd folder should have a source tree document and coding standards at a minimum. These are used by the dev agent, and the many other sharded docs are used by the SM agent.
5. Use a new chat window to allow the SM agent to `draft the next story`.
6. If you agree the story is correct, mark it as approved in the status field, and then start a new chat window with the dev agent.
7. Ask the dev agent to implement the next story. If you draft the story file into the chat it will save time for the dev to have to find what the next one is. The dev should follow the tasks and subtasks marking them off as they are completed. The dev agent will also leave notes potentially for the SM to know about any deviations that might have occured to help draft the next story.
8. Once complete and you have verified, mark it done, and start a new chat. Ask the SM to draft the next story - repeating the cycle.

With this work flow, there is only 1 story in progress at a time, worked sequentially.
