# {{Project Name}} Brownfield Enhancement Architecture

[[LLM: IMPORTANT - SCOPE AND ASSESSMENT REQUIRED:

This architecture document is for SIGNIFICANT enhancements to existing projects that require comprehensive architectural planning. Before proceeding:

1. **Verify Complexity**: Confirm this enhancement requires architectural planning. For simple additions, recommend: "For simpler changes that don't require architectural planning, consider using the brownfield-create-epic or brownfield-create-story task with the Product Owner instead."

2. **REQUIRED INPUTS**:

   - Completed brownfield-prd.md
   - Existing project technical documentation (from docs folder or user-provided)
   - Access to existing project structure (IDE or uploaded files)

3. **DEEP ANALYSIS MANDATE**: You MUST conduct thorough analysis of the existing codebase, architecture patterns, and technical constraints before making ANY architectural recommendations. Every suggestion must be based on actual project analysis, not assumptions.

4. **CONTINUOUS VALIDATION**: Throughout this process, explicitly validate your understanding with the user. For every architectural decision, confirm: "Based on my analysis of your existing system, I recommend [decision] because [evidence from actual project]. Does this align with your system's reality?"

If any required inputs are missing, request them before proceeding.]]

## Introduction

[[LLM: This section establishes the document's purpose and scope for brownfield enhancements. Keep the content below but ensure project name and enhancement details are properly substituted.

After presenting this section, apply `tasks#advanced-elicitation` protocol]]

This document outlines the architectural approach for enhancing {{Project Name}} with {{Enhancement Description}}. Its primary goal is to serve as the guiding architectural blueprint for AI-driven development of new features while ensuring seamless integration with the existing system.

**Relationship to Existing Architecture:**
This document supplements existing project architecture by defining how new components will integrate with current systems. Where conflicts arise between new and existing patterns, this document provides guidance on maintaining consistency while implementing enhancements.

### Existing Project Analysis

[[LLM: Analyze the existing project structure and architecture:

1. Review existing documentation in docs folder
2. Examine current technology stack and versions
3. Identify existing architectural patterns and conventions
4. Note current deployment and infrastructure setup
5. Document any constraints or limitations

CRITICAL: After your analysis, explicitly validate your findings: "Based on my analysis of your project, I've identified the following about your existing system: [key findings]. Please confirm these observations are accurate before I proceed with architectural recommendations."

Present findings and apply `tasks#advanced-elicitation` protocol]]

**Current Project State:**

- **Primary Purpose:** {{existing_project_purpose}}
- **Current Tech Stack:** {{existing_tech_summary}}
- **Architecture Style:** {{existing_architecture_style}}
- **Deployment Method:** {{existing_deployment_approach}}

**Available Documentation:**

- {{existing_docs_summary}}

**Identified Constraints:**

- {{constraint_1}}
- {{constraint_2}}
- {{constraint_3}}

### Change Log

| Change | Date | Version | Description | Author |
| ------ | ---- | ------- | ----------- | ------ |

## Enhancement Scope and Integration Strategy

[[LLM: Define how the enhancement will integrate with the existing system:

1. Review the brownfield PRD enhancement scope
2. Identify integration points with existing code
3. Define boundaries between new and existing functionality
4. Establish compatibility requirements

VALIDATION CHECKPOINT: Before presenting the integration strategy, confirm: "Based on my analysis, the integration approach I'm proposing takes into account [specific existing system characteristics]. These integration points and boundaries respect your current architecture patterns. Is this assessment accurate?"

Present complete integration strategy and apply `tasks#advanced-elicitation` protocol]]

### Enhancement Overview

**Enhancement Type:** {{enhancement_type}}
**Scope:** {{enhancement_scope}}
**Integration Impact:** {{integration_impact_level}}

### Integration Approach

**Code Integration Strategy:** {{code_integration_approach}}
**Database Integration:** {{database_integration_approach}}
**API Integration:** {{api_integration_approach}}
**UI Integration:** {{ui_integration_approach}}

### Compatibility Requirements

- **Existing API Compatibility:** {{api_compatibility}}
- **Database Schema Compatibility:** {{db_compatibility}}
- **UI/UX Consistency:** {{ui_compatibility}}
- **Performance Impact:** {{performance_constraints}}

## Tech Stack Alignment

[[LLM: Ensure new components align with existing technology choices:

1. Use existing technology stack as the foundation
2. Only introduce new technologies if absolutely necessary
3. Justify any new additions with clear rationale
4. Ensure version compatibility with existing dependencies

Present complete tech stack alignment and apply `tasks#advanced-elicitation` protocol]]

### Existing Technology Stack

[[LLM: Document the current stack that must be maintained or integrated with]]

| Category           | Current Technology | Version     | Usage in Enhancement | Notes     |
| :----------------- | :----------------- | :---------- | :------------------- | :-------- |
| **Language**       | {{language}}       | {{version}} | {{usage}}            | {{notes}} |
| **Runtime**        | {{runtime}}        | {{version}} | {{usage}}            | {{notes}} |
| **Framework**      | {{framework}}      | {{version}} | {{usage}}            | {{notes}} |
| **Database**       | {{database}}       | {{version}} | {{usage}}            | {{notes}} |
| **API Style**      | {{api_style}}      | {{version}} | {{usage}}            | {{notes}} |
| **Authentication** | {{auth}}           | {{version}} | {{usage}}            | {{notes}} |
| **Testing**        | {{test_framework}} | {{version}} | {{usage}}            | {{notes}} |
| **Build Tool**     | {{build_tool}}     | {{version}} | {{usage}}            | {{notes}} |

### New Technology Additions

[[LLM: Only include if new technologies are required for the enhancement]]

^^CONDITION: has_new_tech^^

| Technology   | Version     | Purpose     | Rationale     | Integration Method |
| :----------- | :---------- | :---------- | :------------ | :----------------- |
| {{new_tech}} | {{version}} | {{purpose}} | {{rationale}} | {{integration}}    |

^^/CONDITION: has_new_tech^^

## Data Models and Schema Changes

[[LLM: Define new data models and how they integrate with existing schema:

1. Identify new entities required for the enhancement
2. Define relationships with existing data models
3. Plan database schema changes (additions, modifications)
4. Ensure backward compatibility

Present data model changes and apply `tasks#advanced-elicitation` protocol]]

### New Data Models

<<REPEAT: new_data_model>>

### {{model_name}}

**Purpose:** {{model_purpose}}
**Integration:** {{integration_with_existing}}

**Key Attributes:**

- {{attribute_1}}: {{type_1}} - {{description_1}}
- {{attribute_2}}: {{type_2}} - {{description_2}}

**Relationships:**

- **With Existing:** {{existing_relationships}}
- **With New:** {{new_relationships}}

<</REPEAT>>

### Schema Integration Strategy

**Database Changes Required:**

- **New Tables:** {{new_tables_list}}
- **Modified Tables:** {{modified_tables_list}}
- **New Indexes:** {{new_indexes_list}}
- **Migration Strategy:** {{migration_approach}}

**Backward Compatibility:**

- {{compatibility_measure_1}}
- {{compatibility_measure_2}}

## Component Architecture

[[LLM: Define new components and their integration with existing architecture:

1. Identify new components required for the enhancement
2. Define interfaces with existing components
3. Establish clear boundaries and responsibilities
4. Plan integration points and data flow

MANDATORY VALIDATION: Before presenting component architecture, confirm: "The new components I'm proposing follow the existing architectural patterns I identified in your codebase: [specific patterns]. The integration interfaces respect your current component structure and communication patterns. Does this match your project's reality?"

Present component architecture and apply `tasks#advanced-elicitation` protocol]]

### New Components

<<REPEAT: new_component>>

### {{component_name}}

**Responsibility:** {{component_description}}
**Integration Points:** {{integration_points}}

**Key Interfaces:**

- {{interface_1}}
- {{interface_2}}

**Dependencies:**

- **Existing Components:** {{existing_dependencies}}
- **New Components:** {{new_dependencies}}

**Technology Stack:** {{component_tech_details}}

<</REPEAT>>

### Component Interaction Diagram

[[LLM: Create Mermaid diagram showing how new components interact with existing ones]]

````mermaid
{{component_interaction_diagram}}
```text

## API Design and Integration

[[LLM: Define new API endpoints and integration with existing APIs:

1. Plan new API endpoints required for the enhancement
2. Ensure consistency with existing API patterns
3. Define authentication and authorization integration
4. Plan versioning strategy if needed

Present API design and apply `tasks#advanced-elicitation` protocol]]

### New API Endpoints

^^CONDITION: has_new_api^^

**API Integration Strategy:** {{api_integration_strategy}}
**Authentication:** {{auth_integration}}
**Versioning:** {{versioning_approach}}

<<REPEAT: new_endpoint>>

#### {{endpoint_name}}

- **Method:** {{http_method}}
- **Endpoint:** {{endpoint_path}}
- **Purpose:** {{endpoint_purpose}}
- **Integration:** {{integration_with_existing}}

**Request:**

```json
{{request_schema}}
````

**Response:**

````json
{{response_schema}}
```text

<</REPEAT>>

^^/CONDITION: has_new_api^^

## External API Integration

[[LLM: Document new external API integrations required for the enhancement]]

^^CONDITION: has_new_external_apis^^

<<REPEAT: external_api>>

### {{api_name}} API

- **Purpose:** {{api_purpose}}
- **Documentation:** {{api_docs_url}}
- **Base URL:** {{api_base_url}}
- **Authentication:** {{auth_method}}
- **Integration Method:** {{integration_approach}}

**Key Endpoints Used:**

- `{{method}} {{endpoint_path}}` - {{endpoint_purpose}}

**Error Handling:** {{error_handling_strategy}}

<</REPEAT>>

^^/CONDITION: has_new_external_apis^^

## Source Tree Integration

[[LLM: Define how new code will integrate with existing project structure:

1. Follow existing project organization patterns
2. Identify where new files/folders will be placed
3. Ensure consistency with existing naming conventions
4. Plan for minimal disruption to existing structure

Present integration plan and apply `tasks#advanced-elicitation` protocol]]

### Existing Project Structure

[[LLM: Document relevant parts of current structure]]

```plaintext
{{existing_structure_relevant_parts}}
````

### New File Organization

[[LLM: Show only new additions to existing structure]]

```plaintext
{{project-root}}/
├── {{existing_structure_context}}
│   ├── {{new_folder_1}}/           # {{purpose_1}}
│   │   ├── {{new_file_1}}
│   │   └── {{new_file_2}}
│   ├── {{existing_folder}}/        # Existing folder with additions
│   │   ├── {{existing_file}}       # Existing file
│   │   └── {{new_file_3}}          # New addition
│   └── {{new_folder_2}}/           # {{purpose_2}}
```

### Integration Guidelines

- **File Naming:** {{file_naming_consistency}}
- **Folder Organization:** {{folder_organization_approach}}
- **Import/Export Patterns:** {{import_export_consistency}}

## Infrastructure and Deployment Integration

[[LLM: Define how the enhancement will be deployed alongside existing infrastructure:

1. Use existing deployment pipeline and infrastructure
2. Identify any infrastructure changes needed
3. Plan deployment strategy to minimize risk
4. Define rollback procedures

Present deployment integration and apply `tasks#advanced-elicitation` protocol]]

### Existing Infrastructure

**Current Deployment:** {{existing_deployment_summary}}
**Infrastructure Tools:** {{existing_infrastructure_tools}}
**Environments:** {{existing_environments}}

### Enhancement Deployment Strategy

**Deployment Approach:** {{deployment_approach}}
**Infrastructure Changes:** {{infrastructure_changes}}
**Pipeline Integration:** {{pipeline_integration}}

### Rollback Strategy

**Rollback Method:** {{rollback_method}}
**Risk Mitigation:** {{risk_mitigation}}
**Monitoring:** {{monitoring_approach}}

## Coding Standards and Conventions

[[LLM: Ensure new code follows existing project conventions:

1. Document existing coding standards from project analysis
2. Identify any enhancement-specific requirements
3. Ensure consistency with existing codebase patterns
4. Define standards for new code organization

Present coding standards and apply `tasks#advanced-elicitation` protocol]]

### Existing Standards Compliance

**Code Style:** {{existing_code_style}}
**Linting Rules:** {{existing_linting}}
**Testing Patterns:** {{existing_test_patterns}}
**Documentation Style:** {{existing_doc_style}}

### Enhancement-Specific Standards

[[LLM: Only include if new patterns are needed for the enhancement]]

<<REPEAT: enhancement_standard>>

- **{{standard_name}}:** {{standard_description}}

<</REPEAT>>

### Critical Integration Rules

- **Existing API Compatibility:** {{api_compatibility_rule}}
- **Database Integration:** {{db_integration_rule}}
- **Error Handling:** {{error_handling_integration}}
- **Logging Consistency:** {{logging_consistency}}

## Testing Strategy

[[LLM: Define testing approach for the enhancement:

1. Integrate with existing test suite
2. Ensure existing functionality remains intact
3. Plan for testing new features
4. Define integration testing approach

Present testing strategy and apply `tasks#advanced-elicitation` protocol]]

### Integration with Existing Tests

**Existing Test Framework:** {{existing_test_framework}}
**Test Organization:** {{existing_test_organization}}
**Coverage Requirements:** {{existing_coverage_requirements}}

### New Testing Requirements

#### Unit Tests for New Components

- **Framework:** {{test_framework}}
- **Location:** {{test_location}}
- **Coverage Target:** {{coverage_target}}
- **Integration with Existing:** {{test_integration}}

#### Integration Tests

- **Scope:** {{integration_test_scope}}
- **Existing System Verification:** {{existing_system_verification}}
- **New Feature Testing:** {{new_feature_testing}}

#### Regression Testing

- **Existing Feature Verification:** {{regression_test_approach}}
- **Automated Regression Suite:** {{automated_regression}}
- **Manual Testing Requirements:** {{manual_testing_requirements}}

## Security Integration

[[LLM: Ensure security consistency with existing system:

1. Follow existing security patterns and tools
2. Ensure new features don't introduce vulnerabilities
3. Maintain existing security posture
4. Define security testing for new components

Present security integration and apply `tasks#advanced-elicitation` protocol]]

### Existing Security Measures

**Authentication:** {{existing_auth}}
**Authorization:** {{existing_authz}}
**Data Protection:** {{existing_data_protection}}
**Security Tools:** {{existing_security_tools}}

### Enhancement Security Requirements

**New Security Measures:** {{new_security_measures}}
**Integration Points:** {{security_integration_points}}
**Compliance Requirements:** {{compliance_requirements}}

### Security Testing

**Existing Security Tests:** {{existing_security_tests}}
**New Security Test Requirements:** {{new_security_tests}}
**Penetration Testing:** {{pentest_requirements}}

## Risk Assessment and Mitigation

[[LLM: Identify and plan for risks specific to brownfield development:

1. Technical integration risks
2. Deployment and operational risks
3. User impact and compatibility risks
4. Mitigation strategies for each risk

Present risk assessment and apply `tasks#advanced-elicitation` protocol]]

### Technical Risks

<<REPEAT: technical_risk>>

**Risk:** {{risk_description}}
**Impact:** {{impact_level}}
**Likelihood:** {{likelihood}}
**Mitigation:** {{mitigation_strategy}}

<</REPEAT>>

### Operational Risks

<<REPEAT: operational_risk>>

**Risk:** {{risk_description}}
**Impact:** {{impact_level}}
**Likelihood:** {{likelihood}}
**Mitigation:** {{mitigation_strategy}}

<</REPEAT>>

### Monitoring and Alerting

**Enhanced Monitoring:** {{monitoring_additions}}
**New Alerts:** {{new_alerts}}
**Performance Monitoring:** {{performance_monitoring}}

## Checklist Results Report

[[LLM: Execute the architect-checklist and populate results here, focusing on brownfield-specific validation]]

## Next Steps

[[LLM: After completing the brownfield architecture:

1. Review integration points with existing system
2. Begin story implementation with Dev agent
3. Set up deployment pipeline integration
4. Plan rollback and monitoring procedures]]

### Story Manager Handoff

[[LLM: Create a brief prompt for Story Manager to work with this brownfield enhancement. Include:

- Reference to this architecture document
- Key integration requirements validated with user
- Existing system constraints based on actual project analysis
- First story to implement with clear integration checkpoints
- Emphasis on maintaining existing system integrity throughout implementation]]

### Developer Handoff

[[LLM: Create a brief prompt for developers starting implementation. Include:

- Reference to this architecture and existing coding standards analyzed from actual project
- Integration requirements with existing codebase validated with user
- Key technical decisions based on real project constraints
- Existing system compatibility requirements with specific verification steps
- Clear sequencing of implementation to minimize risk to existing functionality]]
