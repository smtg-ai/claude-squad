# Setup Development Environment for Story

## Purpose
Ensure development environment is ready and validated for story implementation. Focused on story-specific setup and validation.

## Inputs
- `story_file`: Path to the approved story file

## Task Execution

### 1. Environment Health Check
- Verify development services are running (database, redis, backend, frontend)
- Check service connectivity and responsiveness
- Validate port availability and configuration
- Ensure no service conflicts or failures

### 2. Development Dependencies
- Verify all required dependencies are installed
- Check package versions match project requirements  
- Validate development tools are available
- Ensure environment variables are properly configured

### 3. Build and Quality Validation
- Execute complete build process to ensure success
- Run linting and type checking to establish baseline
- Verify all existing tests pass before new development
- Check that development server starts successfully

### 4. Authentication and Security
- Test authentication flow with development credentials
- Verify authorization rules are working
- Check security configurations are properly set
- Validate API access and permissions

### 5. Story-Specific Validation
- Review story requirements for any special environment needs
- Check if story requires specific tools or configurations
- Validate access to necessary external services (if applicable)
- Ensure development environment supports story implementation

## Success Criteria
- All services responding correctly
- Build process completes without errors
- Baseline quality checks pass (lint, typecheck, tests)
- Authentication working with test credentials
- Development environment ready for story work

## Outputs
- `environment_status`: "READY" or "ISSUES_FOUND"
- `issues_found`: List of any problems requiring resolution
- `setup_notes`: Any special configurations or notes for development

## Failure Actions
- Document specific environment issues
- Attempt automatic resolution of common problems
- Provide clear remediation steps
- Halt development until environment is stable

## Notes
- Lightweight validation focused on story development readiness
- Not comprehensive infrastructure validation (use validate-infrastructure for that)
- Designed to quickly verify environment is ready for immediate story work