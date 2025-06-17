# {{Project Name}} Fullstack Architecture Document

[[LLM: If available, review any provided relevant documents to gather all relevant context before beginning. At minimum, you should have access to docs/prd.md and docs/front-end-spec.md. Ask the user for any documents you need but cannot locate. This template creates a unified architecture that covers both backend and frontend concerns to guide AI-driven fullstack development.]]

## Introduction

[[LLM: This section establishes the document's purpose and scope. Keep the content below but ensure project name is properly substituted.

After presenting this section, apply `tasks#advanced-elicitation` protocol]]

This document outlines the complete fullstack architecture for {{Project Name}}, including backend systems, frontend implementation, and their integration. It serves as the single source of truth for AI-driven development, ensuring consistency across the entire technology stack.

This unified approach combines what would traditionally be separate backend and frontend architecture documents, streamlining the development process for modern fullstack applications where these concerns are increasingly intertwined.

### Starter Template or Existing Project

[[LLM: Before proceeding with architecture design, check if the project is based on any starter templates or existing codebases:

1. Review the PRD and other documents for mentions of:

- Fullstack starter templates (e.g., T3 Stack, MEAN/MERN starters, Django + React templates)
- Monorepo templates (e.g., Nx, Turborepo starters)
- Platform-specific starters (e.g., Vercel templates, AWS Amplify starters)
- Existing projects being extended or cloned

2. If starter templates or existing projects are mentioned:

- Ask the user to provide access (links, repos, or files)
- Analyze to understand pre-configured choices and constraints
- Note any architectural decisions already made
- Identify what can be modified vs what must be retained

3. If no starter is mentioned but this is greenfield:

- Suggest appropriate fullstack starters based on tech preferences
- Consider platform-specific options (Vercel, AWS, etc.)
- Let user decide whether to use one

4. Document the decision and any constraints it imposes

If none, state "N/A - Greenfield project"

### Change Log

[[LLM: Track document versions and changes]]

| Date | Version | Description | Author |
| :--- | :------ | :---------- | :----- |

## High Level Architecture

[[LLM: This section contains multiple subsections that establish the foundation. Present all subsections together, then apply `tasks#advanced-elicitation` protocol to the complete section.]]

### Technical Summary

[[LLM: Provide a comprehensive overview (4-6 sentences) covering:

- Overall architectural style and deployment approach
- Frontend framework and backend technology choices
- Key integration points between frontend and backend
- Infrastructure platform and services
- How this architecture achieves PRD goals]]

### Platform and Infrastructure Choice

[[LLM: Based on PRD requirements and technical assumptions, make a platform recommendation:

1. Consider common patterns (not an exhaustive list, use your own best judgement and search the web as needed for emerging trends):

   - **Vercel + Supabase**: For rapid development with Next.js, built-in auth/storage
   - **AWS Full Stack**: For enterprise scale with Lambda, API Gateway, S3, Cognito
   - **Azure**: For .NET ecosystems or enterprise Microsoft environments
   - **Google Cloud**: For ML/AI heavy applications or Google ecosystem integration

2. Present 2-3 viable options with clear pros/cons
3. Make a recommendation with rationale
4. Get explicit user confirmation

Document the choice and key services that will be used.]]

**Platform:** {{selected_platform}}
**Key Services:** {{core_services_list}}
**Deployment Host and Regions:** {{regions}}

### Repository Structure

[[LLM: Define the repository approach based on PRD requirements and platform choice:

1. For modern fullstack apps, monorepo is often preferred
2. Consider tooling (Nx, Turborepo, Lerna, npm workspaces)
3. Define package/app boundaries
4. Plan for shared code between frontend and backend]]

**Structure:** {{repo_structure_choice}}
**Monorepo Tool:** {{monorepo_tool_if_applicable}}
**Package Organization:** {{package_strategy}}

### High Level Architecture Diagram

[[LLM: Create a Mermaid diagram showing the complete system architecture including:

- User entry points (web, mobile)
- Frontend application deployment
- API layer (REST/GraphQL)
- Backend services
- Databases and storage
- External integrations
- CDN and caching layers

Use appropriate diagram type for clarity.]]

````mermaid
{{architecture_diagram}}
```text

### Architectural Patterns

[[LLM: List patterns that will guide both frontend and backend development. Include patterns for:

- Overall architecture (e.g., Jamstack, Serverless, Microservices)
- Frontend patterns (e.g., Component-based, State management)
- Backend patterns (e.g., Repository, CQRS, Event-driven)
- Integration patterns (e.g., BFF, API Gateway)

For each pattern, provide recommendation and rationale.]]

<<REPEAT: pattern>>

- **{{pattern_name}}:** {{pattern_description}} - _Rationale:_ {{rationale}}
  <</REPEAT>>

@{example: patterns}

- **Jamstack Architecture:** Static site generation with serverless APIs - _Rationale:_ Optimal performance and scalability for content-heavy applications
- **Component-Based UI:** Reusable React components with TypeScript - _Rationale:_ Maintainability and type safety across large codebases
- **Repository Pattern:** Abstract data access logic - _Rationale:_ Enables testing and future database migration flexibility
- **API Gateway Pattern:** Single entry point for all API calls - _Rationale:_ Centralized auth, rate limiting, and monitoring
  @{/example}

## Tech Stack

[[LLM: This is the DEFINITIVE technology selection for the entire project. Work with user to finalize all choices. This table is the single source of truth - all development must use these exact versions.

Key areas to cover:

- Frontend and backend languages/frameworks
- Databases and caching
- Authentication and authorization
- API approach
- Testing tools for both frontend and backend
- Build and deployment tools
- Monitoring and logging

Upon render, apply `tasks#advanced-elicitation` display immediately.]]

### Technology Stack Table

| Category                 | Technology        | Version     | Purpose     | Rationale      |
| :----------------------- | :---------------- | :---------- | :---------- | :------------- |
| **Frontend Language**    | {{fe_language}}   | {{version}} | {{purpose}} | {{why_chosen}} |
| **Frontend Framework**   | {{fe_framework}}  | {{version}} | {{purpose}} | {{why_chosen}} |
| **UI Component Library** | {{ui_library}}    | {{version}} | {{purpose}} | {{why_chosen}} |
| **State Management**     | {{state_mgmt}}    | {{version}} | {{purpose}} | {{why_chosen}} |
| **Backend Language**     | {{be_language}}   | {{version}} | {{purpose}} | {{why_chosen}} |
| **Backend Framework**    | {{be_framework}}  | {{version}} | {{purpose}} | {{why_chosen}} |
| **API Style**            | {{api_style}}     | {{version}} | {{purpose}} | {{why_chosen}} |
| **Database**             | {{database}}      | {{version}} | {{purpose}} | {{why_chosen}} |
| **Cache**                | {{cache}}         | {{version}} | {{purpose}} | {{why_chosen}} |
| **File Storage**         | {{storage}}       | {{version}} | {{purpose}} | {{why_chosen}} |
| **Authentication**       | {{auth}}          | {{version}} | {{purpose}} | {{why_chosen}} |
| **Frontend Testing**     | {{fe_test}}       | {{version}} | {{purpose}} | {{why_chosen}} |
| **Backend Testing**      | {{be_test}}       | {{version}} | {{purpose}} | {{why_chosen}} |
| **E2E Testing**          | {{e2e_test}}      | {{version}} | {{purpose}} | {{why_chosen}} |
| **Build Tool**           | {{build_tool}}    | {{version}} | {{purpose}} | {{why_chosen}} |
| **Bundler**              | {{bundler}}       | {{version}} | {{purpose}} | {{why_chosen}} |
| **IaC Tool**             | {{iac_tool}}      | {{version}} | {{purpose}} | {{why_chosen}} |
| **CI/CD**                | {{cicd}}          | {{version}} | {{purpose}} | {{why_chosen}} |
| **Monitoring**           | {{monitoring}}    | {{version}} | {{purpose}} | {{why_chosen}} |
| **Logging**              | {{logging}}       | {{version}} | {{purpose}} | {{why_chosen}} |
| **CSS Framework**        | {{css_framework}} | {{version}} | {{purpose}} | {{why_chosen}} |

@{example: tech_stack_rows}
| **Frontend Language** | TypeScript | 5.3.3 | Type-safe frontend development | Strong typing, excellent tooling |
| **Frontend Framework** | Next.js | 14.1.0 | React framework with SSR/SSG | SEO, performance, Vercel integration |
| **Backend Language** | TypeScript | 5.3.3 | Type-safe backend development | Code sharing with frontend |
| **API Style** | REST + tRPC | - | Type-safe API communication | End-to-end type safety |
| **Database** | PostgreSQL | 16.1 | Primary data store | ACID compliance, JSON support |
| **Authentication** | Supabase Auth | 2.39.0 | User authentication | Built-in auth flows, social providers |
@{/example}

## Data Models

[[LLM: Define the core data models/entities that will be shared between frontend and backend:

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

```typescript
{
  {
    model_interface;
  }
}
````

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

````typescript
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
```text

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

```yml
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
````

^^/CONDITION: has_rest_api^^

^^CONDITION: has_graphql_api^^

````graphql
# GraphQL Schema
{{graphql_schema}}
```text

^^/CONDITION: has_graphql_api^^

^^CONDITION: has_trpc_api^^

```typescript
// tRPC Router Definitions
{
  {
    trpc_routers;
  }
}
````

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

## Core Workflows

[[LLM: Illustrate key system workflows using sequence diagrams:

1. Identify critical user journeys from PRD
2. Show component interactions including external APIs
3. Include both frontend and backend flows
4. Include error handling paths
5. Document async operations
6. Create both high-level and detailed diagrams as needed

Focus on workflows that clarify architecture decisions or complex interactions.

After presenting the workflow diagrams, apply `tasks#advanced-elicitation` protocol]]

## Database Schema

[[LLM: Transform the conceptual data models into concrete database schemas:

1. Use the database type(s) selected in Tech Stack
2. Create schema definitions using appropriate notation
3. Include indexes, constraints, and relationships
4. Consider performance and scalability
5. For NoSQL, show document structures

Present schema in format appropriate to database type (SQL DDL, JSON schema, etc.)

After presenting the database schema, apply `tasks#advanced-elicitation` protocol]]

## Frontend Architecture

[[LLM: Define frontend-specific architecture details. After each subsection, note if user wants to refine before continuing.

After presenting this section, apply `tasks#advanced-elicitation` protocol]]

### Component Architecture

[[LLM: Define component organization and patterns based on chosen framework.]]

**Component Organization:**

`````text
{{component_structure}}
```text

**Component Template:**

````typescript
{
  {
    component_template;
  }
}
```text

### State Management Architecture

[[LLM: Detail state management approach based on chosen solution.]]

**State Structure:**

```typescript
{
  {
    state_structure;
  }
}
`````

**State Management Patterns:**

- {{pattern_1}}
- {{pattern_2}}

### Routing Architecture

[[LLM: Define routing structure based on framework choice.]]

**Route Organization:**

`````text
{{route_structure}}
```text

**Protected Route Pattern:**

````typescript
{
  {
    protected_route_example;
  }
}
```text

### Frontend Services Layer

[[LLM: Define how frontend communicates with backend.]]

**API Client Setup:**

```typescript
{
  {
    api_client_setup;
  }
}
`````

**Service Example:**

````typescript
{
  {
    service_example;
  }
}
```text

## Backend Architecture

[[LLM: Define backend-specific architecture details. Consider serverless vs traditional server approaches.

After presenting this section, apply `tasks#advanced-elicitation` protocol]]

### Service Architecture

[[LLM: Based on platform choice, define service organization.]]

^^CONDITION: serverless^^
**Function Organization:**

````

{{function_structure}}

````text

**Function Template:**

```typescript
{
  {
    function_template;
  }
}
````

^^/CONDITION: serverless^^

^^CONDITION: traditional_server^^
**Controller/Route Organization:**

`````text
{{controller_structure}}
```text

**Controller Template:**

````typescript
{
  {
    controller_template;
  }
}
```text

^^/CONDITION: traditional_server^^

### Database Architecture

[[LLM: Define database schema and access patterns.]]

**Schema Design:**

```sql
{{database_schema}}
`````

**Data Access Layer:**

````typescript
{
  {
    repository_pattern;
  }
}
```text

### Authentication and Authorization

[[LLM: Define auth implementation details.]]

**Auth Flow:**

```mermaid
{{auth_flow_diagram}}
````

**Middleware/Guards:**

````typescript
{
  {
    auth_middleware;
  }
}
```text

## Unified Project Structure

[[LLM: Create a monorepo structure that accommodates both frontend and backend. Adapt based on chosen tools and frameworks. After presenting, apply `tasks#advanced-elicitation` protocol.]]

```plaintext
{{project-name}}/
├── .github/                    # CI/CD workflows
│   └── workflows/
│       ├── ci.yml
│       └── deploy.yml
├── apps/                       # Application packages
│   ├── web/                    # Frontend application
│   │   ├── src/
│   │   │   ├── components/     # UI components
│   │   │   ├── pages/          # Page components/routes
│   │   │   ├── hooks/          # Custom React hooks
│   │   │   ├── services/       # API client services
│   │   │   ├── stores/         # State management
│   │   │   ├── styles/         # Global styles/themes
│   │   │   └── utils/          # Frontend utilities
│   │   ├── public/             # Static assets
│   │   ├── tests/              # Frontend tests
│   │   └── package.json
│   └── api/                    # Backend application
│       ├── src/
│       │   ├── routes/         # API routes/controllers
│       │   ├── services/       # Business logic
│       │   ├── models/         # Data models
│       │   ├── middleware/     # Express/API middleware
│       │   ├── utils/          # Backend utilities
│       │   └── {{serverless_or_server_entry}}
│       ├── tests/              # Backend tests
│       └── package.json
├── packages/                   # Shared packages
│   ├── shared/                 # Shared types/utilities
│   │   ├── src/
│   │   │   ├── types/          # TypeScript interfaces
│   │   │   ├── constants/      # Shared constants
│   │   │   └── utils/          # Shared utilities
│   │   └── package.json
│   ├── ui/                     # Shared UI components
│   │   ├── src/
│   │   └── package.json
│   └── config/                 # Shared configuration
│       ├── eslint/
│       ├── typescript/
│       └── jest/
├── infrastructure/             # IaC definitions
│   └── {{iac_structure}}
├── scripts/                    # Build/deploy scripts
├── docs/                       # Documentation
│   ├── prd.md
│   ├── front-end-spec.md
│   └── fullstack-architecture.md
├── .env.example                # Environment template
├── package.json                # Root package.json
├── {{monorepo_config}}         # Monorepo configuration
└── README.md
````

@{example: vercel_structure}
apps/
├── web/ # Next.js app
│ ├── app/ # App directory (Next.js 14+)
│ ├── components/
│ └── lib/
└── api/ # API routes in Next.js or separate
└── pages/api/ # API routes
@{/example}

## Development Workflow

[[LLM: Define the development setup and workflow for the fullstack application.

After presenting this section, apply `tasks#advanced-elicitation` protocol]]

### Local Development Setup

**Prerequisites:**

````bash
{{prerequisites_commands}}
```text

**Initial Setup:**

```bash
{{setup_commands}}
````

**Development Commands:**

````bash
# Start all services
{{start_all_command}}

# Start frontend only
{{start_frontend_command}}

# Start backend only
{{start_backend_command}}

# Run tests
{{test_commands}}
```text

### Environment Configuration

**Required Environment Variables:**

```bash
# Frontend (.env.local)
{{frontend_env_vars}}

# Backend (.env)
{{backend_env_vars}}

# Shared
{{shared_env_vars}}
````

## Deployment Architecture

[[LLM: Define deployment strategy based on platform choice. After presenting, apply `tasks#advanced-elicitation` protocol.]]

### Deployment Strategy

**Frontend Deployment:**

- **Platform:** {{frontend_deploy_platform}}
- **Build Command:** {{frontend_build_command}}
- **Output Directory:** {{frontend_output_dir}}
- **CDN/Edge:** {{cdn_strategy}}

**Backend Deployment:**

- **Platform:** {{backend_deploy_platform}}
- **Build Command:** {{backend_build_command}}
- **Deployment Method:** {{deployment_method}}

### CI/CD Pipeline

````yaml
'[object Object]': null
```text

### Environments

| Environment | Frontend URL       | Backend URL        | Purpose                |
| :---------- | :----------------- | :----------------- | :--------------------- |
| Development | {{dev_fe_url}}     | {{dev_be_url}}     | Local development      |
| Staging     | {{staging_fe_url}} | {{staging_be_url}} | Pre-production testing |
| Production  | {{prod_fe_url}}    | {{prod_be_url}}    | Live environment       |

## Security and Performance

[[LLM: Define security and performance considerations for the fullstack application.

After presenting this section, apply `tasks#advanced-elicitation` protocol]]

### Security Requirements

**Frontend Security:**

- CSP Headers: {{csp_policy}}
- XSS Prevention: {{xss_strategy}}
- Secure Storage: {{storage_strategy}}

**Backend Security:**

- Input Validation: {{validation_approach}}
- Rate Limiting: {{rate_limit_config}}
- CORS Policy: {{cors_config}}

**Authentication Security:**

- Token Storage: {{token_strategy}}
- Session Management: {{session_approach}}
- Password Policy: {{password_requirements}}

### Performance Optimization

**Frontend Performance:**

- Bundle Size Target: {{bundle_size}}
- Loading Strategy: {{loading_approach}}
- Caching Strategy: {{fe_cache_strategy}}

**Backend Performance:**

- Response Time Target: {{response_target}}
- Database Optimization: {{db_optimization}}
- Caching Strategy: {{be_cache_strategy}}

## Testing Strategy

[[LLM: Define comprehensive testing approach for fullstack application.

After presenting this section, apply `tasks#advanced-elicitation` protocol]]

### Testing Pyramid

````

        E2E Tests
       /          \
    Integration Tests

/ \
 Frontend Unit Backend Unit

```text

### Test Organization

**Frontend Tests:**

```

{{frontend_test_structure}}

````text

**Backend Tests:**

```text

{{backend_test_structure}}

```text

**E2E Tests:**

````

{{e2e_test_structure}}

````text

### Test Examples

**Frontend Component Test:**

```typescript
{
  {
    frontend_test_example;
  }
}
````

**Backend API Test:**

````typescript
{
  {
    backend_test_example;
  }
}
```text

**E2E Test:**

```typescript
{
  {
    e2e_test_example;
  }
}
````

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

## Error Handling Strategy

[[LLM: Define unified error handling across frontend and backend.

After presenting this section, apply `tasks#advanced-elicitation` protocol]]

### Error Flow

````mermaid
{{error_flow_diagram}}
```text

### Error Response Format

```typescript
interface ApiError {
  error: {
    code: string;
    message: string;
    details?: Record<string, any>;
    timestamp: string;
    requestId: string;
  };
}
````

### Frontend Error Handling

````typescript
{
  {
    frontend_error_handler;
  }
}
```text

### Backend Error Handling

```typescript
{
  {
    backend_error_handler;
  }
}
````

## Monitoring and Observability

[[LLM: Define monitoring strategy for fullstack application.

After presenting this section, apply `tasks#advanced-elicitation` protocol]]

### Monitoring Stack

- **Frontend Monitoring:** {{frontend_monitoring}}
- **Backend Monitoring:** {{backend_monitoring}}
- **Error Tracking:** {{error_tracking}}
- **Performance Monitoring:** {{perf_monitoring}}

### Key Metrics

**Frontend Metrics:**

- Core Web Vitals
- JavaScript errors
- API response times
- User interactions

**Backend Metrics:**

- Request rate
- Error rate
- Response time
- Database query performance

## Checklist Results Report

[[LLM: Before running the checklist, offer to output the full architecture document. Once user confirms, execute the `architect-checklist` and populate results here.]]
