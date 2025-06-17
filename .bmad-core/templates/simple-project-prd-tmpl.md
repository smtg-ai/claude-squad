# {{Project Name}} Product Requirements Document (PRD)

[[LLM: If available, review any provided document or ask if any are optionally available: Project Brief]]

## Goals and Background Context

[[LLM: Populate the 2 child sections based on what we have received from user description or the provided brief. Allow user to review the 2 sections and offer changes before proceeding]]

### Goals

[[LLM: Bullet list of 1 line desired outcomes the PRD will deliver if successful - user and project desires]]

### Background Context

[[LLM: 1-2 short paragraphs summarizing the background context, such as what we learned in the brief without being redundant with the goals, what and why this solves a problem, what the current landscape or need is etc...]]

### Change Log

[[LLM: Track document versions and changes]]

| Date | Version | Description | Author |
| :--- | :------ | :---------- | :----- |

## Requirements

[[LLM: Draft the list of functional and non functional requirements under the two child sections, and immediately execute tasks#advanced-elicitation display]]

### Functional

[[LLM: Each Requirement will be a bullet markdown and an identifier sequence starting with FR`.]]
@{example: - FR6: The Todo List uses AI to detect and warn against adding potentially duplicate todo items that are worded differently.}

### Non Functional

[[LLM: Each Requirement will be a bullet markdown and an identifier sequence starting with NFR`.]]
@{example: - NFR1: AWS service usage **must** aim to stay within free-tier limits where feasible.}

^^CONDITION: has_ui^^

## User Interface Design Goals

[[LLM: Capture high-level UI/UX vision to inform story creation and also generate a prompt for Lovable or V0 if the user would like either. Steps:

1. Pre-fill all subsections with educated guesses based on project context
2. Present the complete rendered section to user
3. Clearly let the user know where assumptions were made
4. Ask targeted questions for unclear/missing elements or areas needing more specification
5. This is NOT detailed UI spec - focus on product vision and user goals
6. After section completion, immediately apply `tasks#advanced-elicitation` protocol]]

### Overall UX Vision

### Key Interaction Paradigms

### Core Screens and Views

[[LLM: From a product perspective, what are the most critical screens or views necessary to deliver the the PRD values and goals? This is meant to be Conceptual High Level to Drive Rough Epic or User Stories]]

@{example}

- Login Screen
- Main Dashboard
- Item Detail Page
- Settings Page
  @{/example}

### Accessibility: { None, WCAG, etc }

### Branding

[[LLM: Any known branding elements or style guides that must be incorporated?]]

@{example}

- Replicate the look and feel of early 1900s black and white cinema, including animated effects replicating film damage or projector glitches during page or state transitions.
- Attached is the full color pallet and tokens for our corporate branding.
  @{/example}

### Target Device and Platforms

@{example}
"Web Responsive, and all mobile platforms", "IPhone Only", "ASCII Windows Desktop"
@{/example}

^^/CONDITION: has_ui^^

## Technical Assumptions

[[LLM: Gather technical decisions that will be used for this simple technical PRD that includes architecture decisions. Steps:

1. Check if `data#technical-preferences` or an attached `technical-preferences` file exists - use it to pre-populate choices
2. Ask user about: languages, frameworks, starter templates, libraries, APIs, deployment targets
3. For unknowns, offer guidance based on project goals and MVP scope
4. Document ALL technical choices with rationale (why this choice fits the project)
5. These become constraints for the Architect - be specific and complete
6. After section completion, apply `tasks#advanced-elicitation` protocol.]]

### Repository Structure: { Monorepo, Polyrepo, etc...}

### Service Architecture

[[LLM: CRITICAL DECISION - Document the high-level service architecture (e.g., Monolith, Microservices, Serverless functions within a Monorepo).]]

## Testing requirements

[[LLM: CRITICAL DECISION - Document the testing requirements, unit only, integration, e2e, manual, need for manual testing convenience methods).]]

### Additional Technical Assumptions and Requests

[[LLM: Throughout the entire process of drafting this document, if any other technical assumptions are raised or discovered appropriate for the architect, add them here as additional bulleted items]]

## Data Models

[[LLM: Define the core data models/entities that will be used in the front end (if there is one), core application or back end, and if both, shared between frontend and backend:

1. Review PRD requirements and identify key business entities
2. For each model, explain its purpose and relationships
3. Include key attributes and data types
4. Show relationships between models
5. Create TypeScript interfaces that can be shared
6. Discuss design decisions with user

Create a clear conceptual model before moving to database schema.

After presenting all data models, apply `tasks#advanced-elicitation` protocol]]

<<REPEAT: data_model>>

### {{model_name}}

**Purpose:** {{model_purpose}}

**Key Attributes:**

- {{attribute_1}}: {{type_1}} - {{description_1}}
- {{attribute_2}}: {{type_2}} - {{description_2}}

**TypeScript Interface:**

````typescript
{
  {
    model_interface;
  }
}
```text

**Relationships:**

- {{relationship_1}}
- {{relationship_2}}
  <</REPEAT>>

@{example: data_model}

### User

**Purpose:** Represents authenticated users in the system

**Key Attributes:**

- id: string - Unique identifier
- email: string - User's email address
- name: string - Display name
- role: enum - User permission level
- timestamps: Date - Created and updated times

**TypeScript Interface:**

```typescript
interface User {
  id: string;
  email: string;
  name: string;
  role: "admin" | "user" | "guest";
  createdAt: Date;
  updatedAt: Date;
  profile?: UserProfile;
}

interface UserProfile {
  avatarUrl?: string;
  bio?: string;
  preferences: Record<string, any>;
}
````

**Relationships:**

- Has many Posts (1:n)
- Has one Profile (1:1)
  @{/example}

## REST API Spec

[[LLM: Based on the chosen API style from Tech Stack:

1. If REST API, create an OpenAPI 3.0 specification
2. If GraphQL, provide the GraphQL schema
3. If tRPC, show router definitions
4. Include all endpoints from epics/stories
5. Define request/response schemas based on data models
6. Document authentication requirements
7. Include example requests/responses

Use appropriate format for the chosen API style. If no API (e.g., static site), skip this section.]]

^^CONDITION: has_rest_api^^

````yml
openapi: 3.0.0
info:
  title:
    '[object Object]': null
  version:
    '[object Object]': null
  description:
    '[object Object]': null
servers:
  - url:
      '[object Object]': null
    description:
      '[object Object]': null
```text

^^/CONDITION: has_rest_api^^

^^CONDITION: has_graphql_api^^

```graphql
# GraphQL Schema
{{graphql_schema}}
````

^^/CONDITION: has_graphql_api^^

^^CONDITION: has_trpc_api^^

```typescript
// tRPC Router Definitions
{
  {
    trpc_routers;
  }
}
```

^^/CONDITION: has_trpc_api^^

[[LLM: After presenting the API spec (or noting its absence if not applicable), apply `tasks#advanced-elicitation` protocol]]

## Components

[[LLM: Based on the architectural patterns, tech stack, and data models from above:

1. Identify major logical components/services across the fullstack
2. Consider both frontend and backend components
3. Define clear boundaries and interfaces between components
4. For each component, specify:

- Primary responsibility
- Key interfaces/APIs exposed
- Dependencies on other components
- Technology specifics based on tech stack choices

5. Create component diagrams where helpful
6. After presenting all components, apply `tasks#advanced-elicitation` protocol]]

<<REPEAT: component>>

### {{component_name}}

**Responsibility:** {{component_description}}

**Key Interfaces:**

- {{interface_1}}
- {{interface_2}}

**Dependencies:** {{dependencies}}

**Technology Stack:** {{component_tech_details}}
<</REPEAT>>

### Component Diagrams

[[LLM: Create Mermaid diagrams to visualize component relationships. Options:

- C4 Container diagram for high-level view
- Component diagram for detailed internal structure
- Sequence diagrams for complex interactions
  Choose the most appropriate for clarity

After presenting the diagrams, apply `tasks#advanced-elicitation` protocol]]

## External APIs

[[LLM: For each external service integration:

1. Identify APIs needed based on PRD requirements and component design
2. If documentation URLs are unknown, ask user for specifics
3. Document authentication methods and security considerations
4. List specific endpoints that will be used
5. Note any rate limits or usage constraints

If no external APIs are needed, state this explicitly and skip to next section.]]

^^CONDITION: has_external_apis^^

<<REPEAT: external_api>>

### {{api_name}} API

- **Purpose:** {{api_purpose}}
- **Documentation:** {{api_docs_url}}
- **Base URL(s):** {{api_base_url}}
- **Authentication:** {{auth_method}}
- **Rate Limits:** {{rate_limits}}

**Key Endpoints Used:**
<<REPEAT: endpoint>>

- `{{method}} {{endpoint_path}}` - {{endpoint_purpose}}
  <</REPEAT>>

**Integration Notes:** {{integration_considerations}}
<</REPEAT>>

@{example: external_api}

### Stripe API

- **Purpose:** Payment processing and subscription management
- **Documentation:** https://stripe.com/docs/api
- **Base URL(s):** `https://api.stripe.com/v1`
- **Authentication:** Bearer token with secret key
- **Rate Limits:** 100 requests per second

**Key Endpoints Used:**

- `POST /customers` - Create customer profiles
- `POST /payment_intents` - Process payments
- `POST /subscriptions` - Manage subscriptions
  @{/example}

^^/CONDITION: has_external_apis^^

[[LLM: After presenting external APIs (or noting their absence), apply `tasks#advanced-elicitation` protocol]]

## Coding Standards

[[LLM: Define MINIMAL but CRITICAL standards for AI agents. Focus only on project-specific rules that prevent common mistakes. These will be used by dev agents.

After presenting this section, apply `tasks#advanced-elicitation` protocol]]

### Critical Fullstack Rules

<<REPEAT: critical_rule>>

- **{{rule_name}}:** {{rule_description}}
  <</REPEAT>>

@{example: critical_rules}

- **Type Sharing:** Always define types in packages/shared and import from there
- **API Calls:** Never make direct HTTP calls - use the service layer
- **Environment Variables:** Access only through config objects, never process.env directly
- **Error Handling:** All API routes must use the standard error handler
- **State Updates:** Never mutate state directly - use proper state management patterns
  @{/example}

### Naming Conventions

| Element         | Frontend             | Backend    | Example             |
| :-------------- | :------------------- | :--------- | :------------------ |
| Components      | PascalCase           | -          | `UserProfile.tsx`   |
| Hooks           | camelCase with 'use' | -          | `useAuth.ts`        |
| API Routes      | -                    | kebab-case | `/api/user-profile` |
| Database Tables | -                    | snake_case | `user_profiles`     |

## Epics

[[LLM: First, present a high-level list of all epics for user approval, the epic_list and immediately execute tasks#advanced-elicitation display. Each epic should have a title and a short (1 sentence) goal statement. This allows the user to review the overall structure before diving into details.

CRITICAL: Epics MUST be logically sequential following agile best practices:

- Each epic should deliver a significant, end-to-end, fully deployable increment of testable functionality
- Epic 1 must establish foundational project infrastructure (app setup, Git, CI/CD, core services) unless we are adding new functionality to an existing app, while also delivering an initial piece of functionality, even as simple as a health-check route or display of a simple canary page
- Each subsequent epic builds upon previous epics' functionality delivering major blocks of functionality that provide tangible value to users or business when deployed
- Not every project needs multiple epics, an epic needs to deliver value. For example, an API completed can deliver value even if a UI is not complete and planned for a separate epic.
- Err on the side of less epics, but let the user know your rationale and offer options for splitting them if it seems some are too large or focused on disparate things.
- Cross Cutting Concerns should flow through epics and stories and not be final stories. For example, adding a logging framework as a last story of an epic, or at the end of a project as a final epic or story would be terrible as we would not have logging from the beginning.]]

<<REPEAT: epic_list>>

- Epic{{epic_number}} {{epic_title}}: {{short_goal}}

<</REPEAT>>

@{example: epic_list}

1. Foundation & Core Infrastructure: Establish project setup, authentication, and basic user management
2. Core Business Entities: Create and manage primary domain objects with CRUD operations
3. User Workflows & Interactions: Enable key user journeys and business processes
4. Reporting & Analytics: Provide insights and data visualization for users

@{/example}

[[LLM: After the epic list is approved, present each `epic_details` with all its stories and acceptance criteria as a complete review unit and immediately execute tasks#advanced-elicitation display, before moving on to the next epic.]]

<<REPEAT: epic_details>>

## Epic {{epic_number}} {{epic_title}}

{{epic_goal}} [[LLM: Expanded goal - 2-3 sentences describing the objective and value all the stories will achieve]]

[[LLM: CRITICAL STORY SEQUENCING REQUIREMENTS:

- Stories within each epic MUST be logically sequential
- Each story should be a "vertical slice" delivering complete functionality
- No story should depend on work from a later story or epic
- Identify and note any direct prerequisite stories
- Focus on "what" and "why" not "how" (leave technical implementation to Architect) yet be precise enough to support a logical sequential order of operations from story to story.
- Ensure each story delivers clear user or business value, try to avoid enablers and build them into stories that deliver value.
- Size stories for AI agent execution: Each story must be completable by a single AI agent in one focused session without context overflow
- Think "junior developer working for 2-4 hours" - stories must be small, focused, and self-contained
- If a story seems complex, break it down further as long as it can deliver a vertical slice
- Each story should result in working, testable code before the agent's context window fills]]

<<REPEAT: story>>

### Story {{epic_number}}.{{story_number}} {{story_title}}

As a {{user_type}},
I want {{action}},
so that {{benefit}}.

#### Acceptance Criteria

[[LLM: Define clear, comprehensive, and testable acceptance criteria that:

- Precisely define what "done" means from a functional perspective
- Are unambiguous and serve as basis for verification
- Include any critical non-functional requirements from the PRD
- Consider local testability for backend/data components
- Specify UI/UX requirements and framework adherence where applicable
- Avoid cross-cutting concerns that should be in other stories or PRD sections]]

<<REPEAT: criteria>>

- {{criterion number}}: {{criteria}}

<</REPEAT>>
<</REPEAT>>
<</REPEAT>>

## Next Steps

### Design Architect Prompt

[[LLM: This section will contain the prompt for the Design Architect, keep it short and to the point to initiate create architecture mode using this document as input.]]
