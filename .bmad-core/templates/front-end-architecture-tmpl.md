# {{Project Name}} Frontend Architecture Document

[[LLM: Review provided documents including PRD, UX-UI Specification, and main Architecture Document. Focus on extracting technical implementation details needed for AI frontend tools and developer agents. Ask the user for any of these documents if you are unable to locate and were not provided.]]

## Template and Framework Selection

[[LLM: Before proceeding with frontend architecture design, check if the project is using a frontend starter template or existing codebase:

1. Review the PRD, main architecture document, and brainstorming brief for mentions of:

   - Frontend starter templates (e.g., Create React App, Next.js, Vite, Vue CLI, Angular CLI, etc.)
   - UI kit or component library starters
   - Existing frontend projects being used as a foundation
   - Admin dashboard templates or other specialized starters
   - Design system implementations

2. If a frontend starter template or existing project is mentioned:

   - Ask the user to provide access via one of these methods:
     - Link to the starter template documentation
     - Upload/attach the project files (for small projects)
     - Share a link to the project repository
   - Analyze the starter/existing project to understand:
     - Pre-installed dependencies and versions
     - Folder structure and file organization
     - Built-in components and utilities
     - Styling approach (CSS modules, styled-components, Tailwind, etc.)
     - State management setup (if any)
     - Routing configuration
     - Testing setup and patterns
     - Build and development scripts

- Use this analysis to ensure your frontend architecture aligns with the starter's patterns

3. If no frontend starter is mentioned but this is a new UI, ensure we know what the ui language and framework is:

   - Based on the framework choice, suggest appropriate starters:
     - React: Create React App, Next.js, Vite + React
     - Vue: Vue CLI, Nuxt.js, Vite + Vue
     - Angular: Angular CLI
     - Or suggest popular UI templates if applicable
   - Explain benefits specific to frontend development

4. If the user confirms no starter template will be used:
   - Note that all tooling, bundling, and configuration will need manual setup
   - Proceed with frontend architecture from scratch

Document the starter template decision and any constraints it imposes before proceeding.]]

### Change Log

[[LLM: Track document versions and changes]]

| Date | Version | Description | Author |
| :--- | :------ | :---------- | :----- |

## Frontend Tech Stack

[[LLM: Extract from main architecture's Technology Stack Table. This section MUST remain synchronized with the main architecture document. After presenting this section, apply `tasks#advanced-elicitation` protocol]]

### Technology Stack Table

| Category              | Technology           | Version     | Purpose     | Rationale      |
| :-------------------- | :------------------- | :---------- | :---------- | :------------- |
| **Framework**         | {{framework}}        | {{version}} | {{purpose}} | {{why_chosen}} |
| **UI Library**        | {{ui_library}}       | {{version}} | {{purpose}} | {{why_chosen}} |
| **State Management**  | {{state_management}} | {{version}} | {{purpose}} | {{why_chosen}} |
| **Routing**           | {{routing_library}}  | {{version}} | {{purpose}} | {{why_chosen}} |
| **Build Tool**        | {{build_tool}}       | {{version}} | {{purpose}} | {{why_chosen}} |
| **Styling**           | {{styling_solution}} | {{version}} | {{purpose}} | {{why_chosen}} |
| **Testing**           | {{test_framework}}   | {{version}} | {{purpose}} | {{why_chosen}} |
| **Component Library** | {{component_lib}}    | {{version}} | {{purpose}} | {{why_chosen}} |
| **Form Handling**     | {{form_library}}     | {{version}} | {{purpose}} | {{why_chosen}} |
| **Animation**         | {{animation_lib}}    | {{version}} | {{purpose}} | {{why_chosen}} |
| **Dev Tools**         | {{dev_tools}}        | {{version}} | {{purpose}} | {{why_chosen}} |

[[LLM: Fill in appropriate technology choices based on the selected framework and project requirements.]]

## Project Structure

[[LLM: Define exact directory structure for AI tools based on the chosen framework. Be specific about where each type of file goes. Generate a structure that follows the framework's best practices and conventions. After presenting this section, apply `tasks#advanced-elicitation` protocol]]

## Component Standards

[[LLM: Define exact patterns for component creation based on the chosen framework. After presenting this section, apply `tasks#advanced-elicitation` protocol]]

### Component Template

[[LLM: Generate a minimal but complete component template following the framework's best practices. Include TypeScript types, proper imports, and basic structure.]]

### Naming Conventions

[[LLM: Provide naming conventions specific to the chosen framework for components, files, services, state management, and other architectural elements.]]

## State Management

[[LLM: Define state management patterns based on the chosen framework. After presenting this section, apply `tasks#advanced-elicitation` protocol]]

### Store Structure

[[LLM: Generate the state management directory structure appropriate for the chosen framework and selected state management solution.]]

### State Management Template

[[LLM: Provide a basic state management template/example following the framework's recommended patterns. Include TypeScript types and common operations like setting, updating, and clearing state.]]

## API Integration

[[LLM: Define API service patterns based on the chosen framework. After presenting this section, apply `tasks#advanced-elicitation` protocol]]

### Service Template

[[LLM: Provide an API service template that follows the framework's conventions. Include proper TypeScript types, error handling, and async patterns.]]

### API Client Configuration

[[LLM: Show how to configure the HTTP client for the chosen framework, including authentication interceptors/middleware and error handling.]]

## Routing

[[LLM: Define routing structure and patterns based on the chosen framework. After presenting this section, apply `tasks#advanced-elicitation` protocol]]

### Route Configuration

[[LLM: Provide routing configuration appropriate for the chosen framework. Include protected route patterns, lazy loading where applicable, and authentication guards/middleware.]]

## Styling Guidelines

[[LLM: Define styling approach based on the chosen framework. After presenting this section, apply `tasks#advanced-elicitation` protocol]]

### Styling Approach

[[LLM: Describe the styling methodology appropriate for the chosen framework (CSS Modules, Styled Components, Tailwind, etc.) and provide basic patterns.]]

### Global Theme Variables

[[LLM: Provide a CSS custom properties (CSS variables) theme system that works across all frameworks. Include colors, spacing, typography, shadows, and dark mode support.]]

## Testing Requirements

[[LLM: Define minimal testing requirements based on the chosen framework. After presenting this section, apply `tasks#advanced-elicitation` protocol]]

### Component Test Template

[[LLM: Provide a basic component test template using the framework's recommended testing library. Include examples of rendering tests, user interaction tests, and mocking.]]

### Testing Best Practices

1. **Unit Tests**: Test individual components in isolation
2. **Integration Tests**: Test component interactions
3. **E2E Tests**: Test critical user flows (using Cypress/Playwright)
4. **Coverage Goals**: Aim for 80% code coverage
5. **Test Structure**: Arrange-Act-Assert pattern
6. **Mock External Dependencies**: API calls, routing, state management

## Environment Configuration

[[LLM: List required environment variables based on the chosen framework. Show the appropriate format and naming conventions for the framework. After presenting this section, apply `tasks#advanced-elicitation` protocol]]

## Frontend Developer Standards

### Critical Coding Rules

[[LLM: List essential rules that prevent common AI mistakes, including both universal rules and framework-specific ones. After presenting this section, apply `tasks#advanced-elicitation` protocol]]

### Quick Reference

[[LLM: Create a framework-specific cheat sheet with:

- Common commands (dev server, build, test)
- Key import patterns
- File naming conventions
- Project-specific patterns and utilities]]
