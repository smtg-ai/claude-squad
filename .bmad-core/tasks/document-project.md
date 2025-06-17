# Document an Existing Project

## Purpose

Generate comprehensive documentation for existing projects optimized for AI development agents. This task creates structured reference materials that enable AI agents to understand project context, conventions, and patterns for effective contribution to any codebase.

## Task Instructions

### 1. Initial Project Analysis

[[LLM: Begin by conducting a comprehensive analysis of the existing project. Use available tools to:

1. **Project Structure Discovery**: Examine the root directory structure, identify main folders, and understand the overall organization
2. **Technology Stack Identification**: Look for package.json, requirements.txt, Cargo.toml, pom.xml, etc. to identify languages, frameworks, and dependencies
3. **Build System Analysis**: Find build scripts, CI/CD configurations, and development commands
4. **Existing Documentation Review**: Check for README files, docs folders, and any existing documentation
5. **Code Pattern Analysis**: Sample key files to understand coding patterns, naming conventions, and architectural approaches

Ask the user these elicitation questions to better understand their needs:

- What is the primary purpose of this project?
- Are there any specific areas of the codebase that are particularly complex or important for agents to understand?
- What types of tasks do you expect AI agents to perform on this project? (e.g., bug fixes, feature additions, refactoring, testing)
- Are there any existing documentation standards or formats you prefer?
- What level of technical detail should the documentation target? (junior developers, senior developers, mixed team)
  ]]

### 2. Core Documentation Generation

[[LLM: Based on your analysis, generate the following core documentation files. Adapt the content and structure to match the specific project type and context you discovered:

**Core Documents (always generate):**

1. **docs/index.md** - Master documentation index
2. **docs/architecture/index.md** - Architecture documentation index
3. **docs/architecture/coding-standards.md** - Coding conventions and style guidelines
4. **docs/architecture/tech-stack.md** - Technology stack and version constraints
5. **docs/architecture/unified-project-structure.md** - Project structure and organization
6. **docs/architecture/testing-strategy.md** - Testing approaches and requirements

**Backend Documents (generate for backend/full-stack projects):**

7. **docs/architecture/backend-architecture.md** - Backend service patterns and structure
8. **docs/architecture/rest-api-spec.md** - API endpoint specifications
9. **docs/architecture/data-models.md** - Data structures and validation rules
10. **docs/architecture/database-schema.md** - Database design and relationships
11. **docs/architecture/external-apis.md** - Third-party integrations

**Frontend Documents (generate for frontend/full-stack projects):**

12. **docs/architecture/frontend-architecture.md** - Frontend patterns and structure
13. **docs/architecture/components.md** - UI component specifications
14. **docs/architecture/core-workflows.md** - User interaction flows
15. **docs/architecture/ui-ux-spec.md** - UI/UX specifications and guidelines

**Additional Documents (generate if applicable):**

16. **docs/prd.md** - Product requirements document (if not exists)
17. **docs/architecture/deployment-guide.md** - Deployment and operations info
18. **docs/architecture/security-considerations.md** - Security patterns and requirements
19. **docs/architecture/performance-guidelines.md** - Performance optimization patterns

**Optional Enhancement Documents:**

20. **docs/architecture/troubleshooting-guide.md** - Common issues and solutions
21. **docs/architecture/changelog-conventions.md** - Change management practices
22. **docs/architecture/code-review-checklist.md** - Review standards and practices

Present each document section by section, using the advanced elicitation task after each major section.]]

### 3. Document Structure Template

[[LLM: Use this standardized structure for each documentation file, adapting content as needed:

```markdown
# {{Document Title}}

## Overview

{{Brief description of what this document covers and why it's important for AI agents}}

## Quick Reference

{{Key points, commands, or patterns that agents need most frequently}}

## Detailed Information

{{Comprehensive information organized into logical sections}}

## Examples

{{Concrete examples showing proper usage or implementation}}

## Common Patterns

{{Recurring patterns agents should recognize and follow}}

## Things to Avoid

{{Anti-patterns, deprecated approaches, or common mistakes}}

## Related Resources

{{Links to other relevant documentation or external resources}}
```

Each document should be:

- **Concrete and actionable** - Focus on what agents need to do, not just concepts
- **Pattern-focused** - Highlight recurring patterns agents can recognize and replicate
- **Example-rich** - Include specific code examples and real file references
- **Context-aware** - Reference actual project files, folders, and conventions
- **Assumption-free** - Don't assume agents know project history or implicit knowledge
  ]]

### 4. Content Guidelines for Each Document Type

#### Core Architecture Documents

##### docs/architecture/index.md

[[LLM: Create a comprehensive index of all architecture documentation:

- List all architecture documents with brief descriptions
- Group documents by category (backend, frontend, shared)
- Include quick links to key sections
- Provide reading order recommendations for different use cases]]

##### docs/architecture/unified-project-structure.md

[[LLM: Document the complete project structure:

- Root-level directory structure with explanations
- Where each type of code belongs (backend, frontend, tests, etc.)
- File naming conventions and patterns
- Module/package organization
- Generated vs. source file locations
- Build output locations]]

##### docs/architecture/coding-standards.md

[[LLM: Capture project-wide coding conventions:

- Language-specific style guidelines
- Naming conventions (variables, functions, classes, files)
- Code organization within files
- Import/export patterns
- Comment and documentation standards
- Linting and formatting tool configurations
- Git commit message conventions]]

##### docs/architecture/tech-stack.md

[[LLM: Document all technologies and versions:

- Primary languages and versions
- Frameworks and major libraries with versions
- Development tools and their versions
- Database systems and versions
- External services and APIs used
- Browser/runtime requirements]]

##### docs/architecture/testing-strategy.md

[[LLM: Define testing approaches and requirements:

- Test file locations and naming conventions
- Unit testing patterns and frameworks
- Integration testing approaches
- E2E testing setup (if applicable)
- Test coverage requirements
- Mocking strategies
- Test data management]]

#### Backend Architecture Documents

##### docs/architecture/backend-architecture.md

[[LLM: Document backend service structure:

- Service layer organization
- Controller/route patterns
- Middleware architecture
- Authentication/authorization patterns
- Request/response flow
- Background job processing
- Service communication patterns]]

##### docs/architecture/rest-api-spec.md

[[LLM: Specify all API endpoints:

- Base URL and versioning strategy
- Authentication methods
- Common headers and parameters
- Each endpoint with:
  - HTTP method and path
  - Request parameters/body
  - Response format and status codes
  - Error responses
- Rate limiting and quotas]]

##### docs/architecture/data-models.md

[[LLM: Define data structures and validation:

- Core business entities
- Data validation rules
- Relationships between entities
- Computed fields and derivations
- Data transformation patterns
- Serialization formats]]

##### docs/architecture/database-schema.md

[[LLM: Document database design:

- Database type and version
- Table/collection structures
- Indexes and constraints
- Relationships and foreign keys
- Migration patterns
- Seed data requirements
- Backup and recovery procedures]]

##### docs/architecture/external-apis.md

[[LLM: Document third-party integrations:

- List of external services used
- Authentication methods for each
- API endpoints and usage patterns
- Rate limits and quotas
- Error handling strategies
- Webhook configurations
- Data synchronization patterns]]

#### Frontend Architecture Documents

##### docs/architecture/frontend-architecture.md

[[LLM: Document frontend application structure:

- Component hierarchy and organization
- State management patterns
- Routing architecture
- Data fetching patterns
- Authentication flow
- Error boundary strategies
- Performance optimization patterns]]

##### docs/architecture/components.md

[[LLM: Specify UI components:

- Component library/design system used
- Custom component specifications
- Props and state for each component
- Component composition patterns
- Styling approaches
- Accessibility requirements
- Component testing patterns]]

##### docs/architecture/core-workflows.md

[[LLM: Document user interaction flows:

- Major user journeys
- Screen flow diagrams
- Form handling patterns
- Navigation patterns
- Data flow through workflows
- Error states and recovery
- Loading and transition states]]

##### docs/architecture/ui-ux-spec.md

[[LLM: Define UI/UX guidelines:

- Design system specifications
- Color palette and typography
- Spacing and layout grids
- Responsive breakpoints
- Animation and transition guidelines
- Accessibility standards
- Browser compatibility requirements]]

### 5. Adaptive Content Strategy

[[LLM: Adapt your documentation approach based on project characteristics:

**For Web Applications:**

- Focus on component patterns, routing, state management
- Include build processes, asset handling, and deployment
- Cover API integration patterns and data fetching

**For Backend Services:**

- Emphasize service architecture, data models, and API design
- Include database interaction patterns and migration strategies
- Cover authentication, authorization, and security patterns

**For CLI Tools:**

- Focus on command structure, argument parsing, and output formatting
- Include plugin/extension patterns if applicable
- Cover configuration file handling and user interaction patterns

**For Libraries/Frameworks:**

- Emphasize public API design and usage patterns
- Include extension points and customization approaches
- Cover versioning, compatibility, and migration strategies

**For Mobile Applications:**

- Focus on platform-specific patterns and navigation
- Include state management and data persistence approaches
- Cover platform integration and native feature usage

**For Data Science/ML Projects:**

- Emphasize data pipeline patterns and model organization
- Include experiment tracking and reproducibility approaches
- Cover data validation and model deployment patterns
  ]]

### 6. Quality Assurance

[[LLM: Before completing each document:

1. **Accuracy Check**: Verify all file paths, commands, and code examples work
2. **Completeness Review**: Ensure the document covers the most important patterns an agent would encounter
3. **Clarity Assessment**: Check that explanations are clear and actionable
4. **Consistency Verification**: Ensure terminology and patterns align across all documents
5. **Agent Perspective**: Review from the viewpoint of an AI agent that needs to contribute to this project

Ask the user to review each completed document and use the advanced elicitation task to refine based on their feedback.]]

### 7. Final Integration

[[LLM: After all documents are completed:

1. Ensure all documents are created in the proper BMAD-expected locations:

   - Core docs in `docs/` (index.md, prd.md)
   - Architecture shards in `docs/architecture/` subdirectory
   - Create the `docs/architecture/` directory if it doesn't exist

2. Create/update the master index documents:

   - Update `docs/index.md` to reference all documentation
   - Create `docs/architecture/index.md` listing all architecture shards

3. Verify document cross-references:

   - Ensure all documents link to related documentation
   - Check that file paths match the actual project structure
   - Validate that examples reference real files in the project

4. Provide maintenance guidance:

   - Document update triggers (when to update each doc)
   - Create a simple checklist for keeping docs current
   - Suggest automated validation approaches

5. Summary report including:
   - List of all documents created with their paths
   - Any gaps or areas needing human review
   - Recommendations for project-specific additions
   - Next steps for maintaining documentation accuracy

Present a summary of what was created and ask if any additional documentation would be helpful for AI agents working on this specific project.]]

## Success Criteria

- Documentation enables AI agents to understand project context without additional explanation
- All major architectural patterns and coding conventions are captured
- Examples reference actual project files and demonstrate real usage
- Documentation is structured consistently and easy to navigate
- Content is actionable and focuses on what agents need to do, not just understand

## Notes

- This task is designed to work with any project type, language, or framework
- The documentation should reflect the project as it actually is, not as it should be
- Focus on patterns that agents can recognize and replicate consistently
- Include both positive examples (what to do) and negative examples (what to avoid)
