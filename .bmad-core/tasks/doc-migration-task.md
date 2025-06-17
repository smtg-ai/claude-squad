# Document Migration Task

## Purpose

Simple document migration that cleans up heading formats and adds epic structure for PRDs.

## Task Requirements

1. **Input**: User specifies the document to migrate (e.g., `docs/prd.md`)
2. **Detection**: Automatically determine if it's a PRD or other document type
3. **Migration**: Apply appropriate transformations
4. **Backup**: Create backup with `.bak` extension

## Migration Rules

### For PRDs

- Find all level 3 headings that appear to be epics
- Add a level 2 heading "## Epic #" (incrementing number) before each epic
- Also apply the heading cleanup rules below

### For All Documents

- Find all level 2 headings (`## ...`)
- Remove leading numbers and symbols
- Keep only alphabetic characters and spaces
- **CRITICAL**: Do not lose any information - preserve all content under appropriate headings
- Examples:
  - `## 1. Foo & Bar` → `## Foo Bar`
  - `## 2.1 Technical Overview` → `## Technical Overview`
  - `## 3) User Experience` → `## User Experience`

### For Architecture Documents

- **PRIMARY GOAL**: Align level 2 headings to match template level 2 titles exactly
- **PRESERVE EVERYTHING**: Do not lose any information during migration
- Map existing content to the closest matching template section
- If content doesn't fit template sections, create appropriate level 3 subsections

## Detection Logic

A document is considered a PRD if:

- Filename contains "prd" (case insensitive)
- OR main title contains "Product Requirements" or "PRD"
- OR contains sections like "User Stories", "Functional Requirements", "Acceptance Criteria"

## Implementation Steps

1. **Backup Original**: Copy `filename.md` to `filename.md.bak`
2. **Detect Type**: Check if document is a PRD
3. **Process Headings**:
   - Clean all level 2 headings
   - If PRD: Add epic structure before level 3 headings that look like epics
4. **Write Result**: Overwrite original file with migrated content

## Epic Detection for PRDs

Level 3 headings are treated as epics if they:

- Describe features or functionality
- Are substantial sections (not just "Overview" or "Notes")
- Common epic patterns: "User Management", "Payment Processing", "Reporting Dashboard"

The epic numbering starts at 1 and increments for each epic found.

## Example

### Before (PRD):

`````markdown
# Product Requirements Document

## 1. Executive Summary

Content here...

## 2.1 Functional Requirements & Specs

Content here...

### User Management System

Epic content...

### Payment Processing

Epic content...

## 3) Success Metrics

Content here...

````text

### After (PRD):
```markdown
# Product Requirements Document

## Executive Summary
Content here...

## Functional Requirements Specs
Content here...

## Epic 1
### User Management System
Epic content...

## Epic 2
### Payment Processing
Epic content...

## Success Metrics
Content here...
```text

### Before (Non-PRD):
```markdown
# Architecture Document

## 1. System Overview
Content...

## 2.1 Technical Stack & Tools
Content...
```text

### After (Non-PRD):
```markdown
# Architecture Document

## System Overview
Content...

## Technical Stack Tools
Content...
````
`````

```text

```
