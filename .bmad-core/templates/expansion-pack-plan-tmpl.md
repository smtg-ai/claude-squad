# {Pack Name} Expansion Pack Plan

## Overview

- **Pack Name**: {pack-identifier}
- **Display Name**: {Full Expansion Pack Name}
- **Description**: {Brief description of what this pack does}
- **Target Domain**: {Industry/domain this serves}
- **Author**: {Your name/organization}

## Problem Statement

{What specific challenges does this expansion pack solve?}

## Target Users

{Who will benefit from this expansion pack?}

## Components to Create

### Agents

- [ ] `{pack-name}-orchestrator` - **REQUIRED**: Master orchestrator for {domain} workflows
  - Key commands: {list main commands}
  - Manages: {what it orchestrates}
- [ ] `{agent-1-name}` - {Role description}
  - Tasks used: {task-1}, {task-2}
  - Templates used: {template-1}
  - Data required: {data-file-1}
- [ ] `{agent-2-name}` - {Role description}
  - Tasks used: {task-3}
  - Templates used: {template-2}
  - Data required: {data-file-2}

### Tasks

- [ ] `{task-1}.md` - {Purpose} (used by: {agent})
- [ ] `{task-2}.md` - {Purpose} (used by: {agent})
- [ ] `{task-3}.md` - {Purpose} (used by: {agent})

### Templates

- [ ] `{template-1}-tmpl.md` - {Document type} (used by: {agent/task})
- [ ] `{template-2}-tmpl.md` - {Document type} (used by: {agent/task})

### Checklists

- [ ] `{checklist-1}-checklist.md` - {What it validates}
- [ ] `{checklist-2}-checklist.md` - {What it validates}

### Data Files Required from User

Users must add these files to `bmad-core/data/`:

- [ ] `{data-file-1}.{ext}` - {Description of required content}
  - Format: {file format}
  - Purpose: {why needed}
  - Example: {brief example}
- [ ] `{data-file-2}.{ext}` - {Description of required content}
  - Format: {file format}
  - Purpose: {why needed}
  - Example: {brief example}

## Workflow Overview

1. {Step 1 - typically starts with orchestrator}
2. {Step 2}
3. {Step 3}
4. {Final output/deliverable}

## Integration Points

- Depends on core agents: {list any core BMAD agents used}
- Extends teams: {which teams to update}

## Success Criteria

- [ ] All components created and cross-referenced
- [ ] No orphaned task/template references
- [ ] Data requirements clearly documented
- [ ] Orchestrator provides clear workflow
- [ ] README includes setup instructions

## User Approval

- [ ] Plan reviewed by user
- [ ] Approval to proceed with implementation

---

**Next Steps**: Once approved, proceed with Phase 3 implementation starting with the orchestrator agent.
