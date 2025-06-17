# {{Project Name}} Brownfield Enhancement PRD

[[LLM: IMPORTANT - SCOPE ASSESSMENT REQUIRED:

This PRD is for SIGNIFICANT enhancements to existing projects that require comprehensive planning and multiple stories. Before proceeding:

1. **Assess Enhancement Complexity**: If this is a simple feature addition or bug fix that could be completed in 1-2 focused development sessions, STOP and recommend: "For simpler changes, consider using the brownfield-create-epic or brownfield-create-story task with the Product Owner instead. This full PRD process is designed for substantial enhancements that require architectural planning and multiple coordinated stories."

2. **Project Context**: Determine if we're working in an IDE with the project already loaded or if the user needs to provide project information. If project files are available, analyze existing documentation in the docs folder. If insufficient documentation exists, recommend running the document-project task first.

3. **Deep Assessment Requirement**: You MUST thoroughly analyze the existing project structure, patterns, and constraints before making ANY suggestions. Every recommendation must be grounded in actual project analysis, not assumptions.]]

## Intro Project Analysis and Context

[[LLM: Gather comprehensive information about the existing project. This section must be completed before proceeding with requirements.

CRITICAL: Throughout this analysis, explicitly confirm your understanding with the user. For every assumption you make about the existing project, ask: "Based on my analysis, I understand that [assumption]. Is this correct?"

Do not proceed with any recommendations until the user has validated your understanding of the existing system.]]

### Existing Project Overview

[[LLM: If working in IDE with project loaded, analyze the project structure and existing documentation. If working in web interface, request project upload or detailed project information from user.]]

**Project Location**: [[LLM: Note if this is IDE-based analysis or user-provided information]]

**Current Project State**: [[LLM: Brief description of what the project currently does and its primary purpose]]

### Available Documentation Analysis

[[LLM: Check for existing documentation in docs folder or provided by user. List what documentation is available and assess its completeness. Required documents include:

- Tech stack documentation
- Source tree/architecture overview
- Coding standards
- API documentation or OpenAPI specs
- External API integrations
- UX/UI guidelines or existing patterns]]

**Available Documentation**:

- [ ] Tech Stack Documentation
- [ ] Source Tree/Architecture
- [ ] Coding Standards
- [ ] API Documentation
- [ ] External API Documentation
- [ ] UX/UI Guidelines
- [ ] Other: \***\*\_\_\_\*\***

[[LLM: If critical documentation is missing, STOP and recommend: "I recommend running the document-project task first to generate baseline documentation including tech-stack, source-tree, coding-standards, APIs, external-APIs, and UX/UI information. This will provide the foundation needed for a comprehensive brownfield PRD."]]

### Enhancement Scope Definition

[[LLM: Work with user to clearly define what type of enhancement this is. This is critical for scoping and approach.]]

**Enhancement Type**: [[LLM: Determine with user which applies]]

- [ ] New Feature Addition
- [ ] Major Feature Modification
- [ ] Integration with New Systems
- [ ] Performance/Scalability Improvements
- [ ] UI/UX Overhaul
- [ ] Technology Stack Upgrade
- [ ] Bug Fix and Stability Improvements
- [ ] Other: \***\*\_\_\_\*\***

**Enhancement Description**: [[LLM: 2-3 sentences describing what the user wants to add or change]]

**Impact Assessment**: [[LLM: Assess the scope of impact on existing codebase]]

- [ ] Minimal Impact (isolated additions)
- [ ] Moderate Impact (some existing code changes)
- [ ] Significant Impact (substantial existing code changes)
- [ ] Major Impact (architectural changes required)

### Goals and Background Context

#### Goals

[[LLM: Bullet list of 1-line desired outcomes this enhancement will deliver if successful]]

#### Background Context

[[LLM: 1-2 short paragraphs explaining why this enhancement is needed, what problem it solves, and how it fits with the existing project]]

### Change Log

| Change | Date | Version | Description | Author |
| ------ | ---- | ------- | ----------- | ------ |

## Requirements

[[LLM: Draft functional and non-functional requirements based on your validated understanding of the existing project. Before presenting requirements, confirm: "These requirements are based on my understanding of your existing system. Please review carefully and confirm they align with your project's reality." Then immediately execute tasks#advanced-elicitation display]]

### Functional

[[LLM: Each Requirement will be a bullet markdown with identifier starting with FR]]
@{example: - FR1: The existing Todo List will integrate with the new AI duplicate detection service without breaking current functionality.}

### Non Functional

[[LLM: Each Requirement will be a bullet markdown with identifier starting with NFR. Include constraints from existing system]]
@{example: - NFR1: Enhancement must maintain existing performance characteristics and not exceed current memory usage by more than 20%.}

### Compatibility Requirements

[[LLM: Critical for brownfield - what must remain compatible]]

- CR1: [[LLM: Existing API compatibility requirements]]
- CR2: [[LLM: Database schema compatibility requirements]]
- CR3: [[LLM: UI/UX consistency requirements]]
- CR4: [[LLM: Integration compatibility requirements]]

^^CONDITION: has_ui^^

## User Interface Enhancement Goals

[[LLM: For UI changes, capture how they will integrate with existing UI patterns and design systems]]

### Integration with Existing UI

[[LLM: Describe how new UI elements will fit with existing design patterns, style guides, and component libraries]]

### Modified/New Screens and Views

[[LLM: List only the screens/views that will be modified or added]]

### UI Consistency Requirements

[[LLM: Specific requirements for maintaining visual and interaction consistency with existing application]]

^^/CONDITION: has_ui^^

## Technical Constraints and Integration Requirements

[[LLM: This section replaces separate architecture documentation. Gather detailed technical constraints from existing project analysis.]]

### Existing Technology Stack

[[LLM: Document the current technology stack that must be maintained or integrated with]]

**Languages**: [[LLM: Current programming languages in use]]
**Frameworks**: [[LLM: Current frameworks and their versions]]
**Database**: [[LLM: Current database technology and schema considerations]]
**Infrastructure**: [[LLM: Current deployment and hosting infrastructure]]
**External Dependencies**: [[LLM: Current third-party services and APIs]]

### Integration Approach

[[LLM: Define how the enhancement will integrate with existing architecture]]

**Database Integration Strategy**: [[LLM: How new features will interact with existing database]]
**API Integration Strategy**: [[LLM: How new APIs will integrate with existing API structure]]
**Frontend Integration Strategy**: [[LLM: How new UI components will integrate with existing frontend]]
**Testing Integration Strategy**: [[LLM: How new tests will integrate with existing test suite]]

### Code Organization and Standards

[[LLM: Based on existing project analysis, define how new code will fit existing patterns]]

**File Structure Approach**: [[LLM: How new files will fit existing project structure]]
**Naming Conventions**: [[LLM: Existing naming conventions that must be followed]]
**Coding Standards**: [[LLM: Existing coding standards and linting rules]]
**Documentation Standards**: [[LLM: How new code documentation will match existing patterns]]

### Deployment and Operations

[[LLM: How the enhancement fits existing deployment pipeline]]

**Build Process Integration**: [[LLM: How enhancement builds with existing process]]
**Deployment Strategy**: [[LLM: How enhancement will be deployed alongside existing features]]
**Monitoring and Logging**: [[LLM: How enhancement will integrate with existing monitoring]]
**Configuration Management**: [[LLM: How new configuration will integrate with existing config]]

### Risk Assessment and Mitigation

[[LLM: Identify risks specific to working with existing codebase]]

**Technical Risks**: [[LLM: Risks related to modifying existing code]]
**Integration Risks**: [[LLM: Risks in integrating with existing systems]]
**Deployment Risks**: [[LLM: Risks in deploying alongside existing features]]
**Mitigation Strategies**: [[LLM: Specific strategies to address identified risks]]

## Epic and Story Structure

[[LLM: For brownfield projects, favor a single comprehensive epic unless the user is clearly requesting multiple unrelated enhancements. Before presenting the epic structure, confirm: "Based on my analysis of your existing project, I believe this enhancement should be structured as [single epic/multiple epics] because [rationale based on actual project analysis]. Does this align with your understanding of the work required?" Then present the epic structure and immediately execute tasks#advanced-elicitation display.]]

### Epic Approach

[[LLM: Explain the rationale for epic structure - typically single epic for brownfield unless multiple unrelated features]]

**Epic Structure Decision**: [[LLM: Single Epic or Multiple Epics with rationale]]

## Epic 1: {{enhancement_title}}

[[LLM: Comprehensive epic that delivers the brownfield enhancement while maintaining existing functionality]]

**Epic Goal**: [[LLM: 2-3 sentences describing the complete enhancement objective and value]]

**Integration Requirements**: [[LLM: Key integration points with existing system]]

[[LLM: CRITICAL STORY SEQUENCING FOR BROWNFIELD:

- Stories must ensure existing functionality remains intact
- Each story should include verification that existing features still work
- Stories should be sequenced to minimize risk to existing system
- Include rollback considerations for each story
- Focus on incremental integration rather than big-bang changes
- Size stories for AI agent execution in existing codebase context
- MANDATORY: Present the complete story sequence and ask: "This story sequence is designed to minimize risk to your existing system. Does this order make sense given your project's architecture and constraints?"
- Stories must be logically sequential with clear dependencies identified
- Each story must deliver value while maintaining system integrity]]

<<REPEAT: story>>

### Story 1.{{story_number}} {{story_title}}

As a {{user_type}},
I want {{action}},
so that {{benefit}}.

#### Acceptance Criteria

[[LLM: Define criteria that include both new functionality and existing system integrity]]

<<REPEAT: criteria>>

- {{criterion number}}: {{criteria}}

<</REPEAT>>

#### Integration Verification

[[LLM: Specific verification steps to ensure existing functionality remains intact]]

- IV1: [[LLM: Existing functionality verification requirement]]
- IV2: [[LLM: Integration point verification requirement]]
- IV3: [[LLM: Performance impact verification requirement]]

<</REPEAT>>
