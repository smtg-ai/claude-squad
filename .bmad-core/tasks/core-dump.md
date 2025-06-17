# Core Dump Task

## Purpose

To create a concise memory recording file (`.ai/core-dump-n.md`) that captures the essential context of the current agent session, enabling seamless continuation of work in future agent sessions. This task ensures persistent context across agent conversations while maintaining minimal token usage for efficient context loading.

## Inputs for this Task

- Current session conversation history and accomplishments
- Files created, modified, or deleted during the session
- Key decisions made and procedures followed
- Current project state and next logical steps
- User requests and agent responses that shaped the session

## Task Execution Instructions

### 0. Check Existing Core Dump

Before proceeding, check if `.ai/core-dump.md` already exists:

- If file exists, ask user: "Core dump file exists. Should I: 1. Overwrite, 2. Update, 3. Append or 4. Create new?"
- **Overwrite**: Replace entire file with new content
- **Update**: Merge new session info with existing content, updating relevant sections
- **Append**: Add new session as a separate entry while preserving existing content
- **Create New**: Create a new file, appending the next possible -# to the file, such as core-dump-3.md if 1 and 2 already exist.
- If file doesn't exist, proceed with creation of `core-dump-1.md`

### 1. Analyze Session Context

- Review the entire conversation to identify key accomplishments
- Note any specific tasks, procedures, or workflows that were executed
- Identify important decisions made or problems solved
- Capture the user's working style and preferences observed during the session

### 2. Document What Was Accomplished

- **Primary Actions**: List the main tasks completed concisely
- **Story Progress**: For story work, use format "Tasks Complete: 1-6, 8. Next Task Pending: 7, 9"
- **Problem Solving**: Document any challenges encountered and how they were resolved
- **User Communications**: Summarize key user requests, preferences, and discussion points

### 3. Record File System Changes (Concise Format)

- **Files Created**: `filename.ext` (brief purpose/size)
- **Files Modified**: `filename.ext` (what changed)
- **Files Deleted**: `filename.ext` (why removed)
- Focus on essential details, avoid verbose descriptions

### 4. Capture Current Project State

- **Project Progress**: Where the project stands after this session
- **Current Issues**: Any blockers or problems that need resolution
- **Next Logical Steps**: What would be the natural next actions to take

### 5. Create/Update Core Dump File

Based on user's choice from step 0, handle the file accordingly:

### 6. Optimize for Minimal Context

- Keep descriptions concise but informative
- Use abbreviated formats where possible (file sizes, task numbers)
- Focus on actionable information rather than detailed explanations
- Avoid redundant information that can be found in project documentation
- Prioritize information that would be lost without this recording
- Ensure the file can be quickly scanned and understood

### 7. Validate Completeness

- Verify all significant session activities are captured
- Ensure a future agent could understand the current state
- Check that file changes are accurately recorded
- Confirm next steps are clear and actionable
- Verify user communication style and preferences are noted
