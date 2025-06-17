# Reduced Product Requirements Document
## Epic 1 Multi-Project: Focused UX Improvements

### Executive Summary

Based on real user testing feedback, this PRD defines a **minimal scope** to address specific UX pain points identified during Epic 1.1 validation. Focus is on **fixing critical issues** and **improving core usability** rather than adding new features.

**Guiding Principle**: Fix what's broken, polish what's working, defer what's not essential.

---

## Problem Statement

Epic 1.1 delivered functional multi-project support but revealed **critical UX issues** and opportunities for enhancement:

**Phase 1 - Critical UX Fixes:**
1. **🔥 CRITICAL**: Tmux session name conflicts causing creation failures
2. **🎯 UX**: Add Project dialog is not visually centered  
3. **🎯 UX**: No visual indication of current active project

**Phase 2 - UX Enhancements:**
4. **🌿 UX**: Branch names truncated at wrong end (prefix vs suffix)
5. **📊 UX**: Redundant project name display in session lists
6. **🔌 FEATURE**: MCP (Model Context Protocol) integration for enhanced capabilities

**Success Criteria**: Address critical issues first, then add valuable enhancements with controlled scope.

---

## Scope Definition

### ✅ **IN SCOPE**
**Phase 1 - Critical Fixes:**
- Fix tmux session name collision errors
- Center Add Project dialog properly
- Display current active project clearly

**Phase 2 - UX Enhancements:**
- Fix branch name truncation (show suffix, not prefix)
- Remove redundant project name display
- MCP integration for enhanced development capabilities
- Error handling improvements for edge cases

### ❌ **OUT OF SCOPE**
- Advanced project operations (rename, delete)
- Project switching mechanisms
- Performance optimizations
- Help system updates
- Complex visual hierarchy (deferred)
- App-level MCP management (session-level only)

---

## User Stories

### **Story 1.2: Fix Session Creation Errors**
**Priority**: P0 (Critical Bug)
**User Story**: As a user, when I create a new instance, it should work reliably without tmux session conflicts.

**Current Problem**: `failed to start new session: tmux session already exists: claudesquad_piola`

**Acceptance Criteria**:
- AC1: Instance creation never fails due to tmux session name conflicts
- AC2: Clear error messages when tmux operations fail
- AC3: Automatic session name resolution for conflicts
- AC4: Graceful fallback when session names are unavailable

**Technical Implementation**:
- Add unique suffix to session names (timestamp/UUID)
- Implement session name availability checking
- Add proper error handling with user-friendly messages
- Add retry logic for session creation

**Definition of Done**:
- [ ] Instance creation succeeds 100% of the time when valid
- [ ] Error messages are clear and actionable
- [ ] No tmux session name collisions occur
- [ ] Regression tests cover session name scenarios

---

### **Story 1.3: Center Add Project Dialog**
**Priority**: P1 (UX Issue)
**User Story**: As a user, the Add Project dialog should be visually centered and properly positioned.

**Current Problem**: Dialog appears off-center and positioning is inconsistent.

**Acceptance Criteria**:
- AC1: Add Project dialog is perfectly centered on screen
- AC2: Dialog positioning is consistent across window sizes
- AC3: Dialog remains visible and accessible on all screen resolutions
- AC4: Visual alignment follows design standards

**Technical Implementation**:
- Fix dialog centering logic in project input overlay
- Ensure responsive positioning for different screen sizes
- Add proper padding and margins
- Validate positioning across resolutions

**Definition of Done**:
- [ ] Dialog is perfectly centered on all screen sizes
- [ ] Visual appearance is professional and consistent
- [ ] Positioning works correctly on resize
- [ ] No visual glitches or misalignment

---

### **Story 1.4: Display Current Active Project**
**Priority**: P1 (UX Issue)  
**User Story**: As a user, I want to clearly see which project is currently active at all times.

**Current Problem**: No visual indication of active project context.

**UX Considerations** (from UX Expert review):
- **Problem**: Ubicación puede crear visual clutter
- Poner "Active: ProjectName" cerca de "Instances" puede saturar el header
- **Recomendación**: Considerar integrar en el título mismo: "Instances (ProjectName)"

**Acceptance Criteria** (Updated):
- AC1: Current active project is integrated into "Instances" header to avoid clutter
- AC2: Active project display updates immediately when project changes
- AC3: Display shows meaningful project name, not technical ID
- AC4: Visual design is clean and doesn't clutter the interface
- AC5: Display handles "no active project" state gracefully (shows just "Instances")

**Technical Implementation** (Simplified):
- Integrate active project name into existing "Instances" header
- Format: "Instances (ProjectName)" or just "Instances" when no active project
- Use project name instead of ID for readability
- Add proper state management for active project changes
- Avoid separate UI component to prevent visual clutter

**Definition of Done**:
- [ ] Active project is always visible and clear
- [ ] Display updates immediately on project changes
- [ ] Visual design is clean and professional
- [ ] Handles all edge cases (no project, invalid project)
- [ ] Doesn't interfere with existing UI elements

---

### **Story 1.5: Fix Branch Name Truncation**
**Priority**: P1 (UX Fix)
**User Story**: As a developer, I want to see the meaningful part of branch names when they are truncated, so I can quickly identify which branch I'm working on.

**Current Problem**: Branch names are truncated from the suffix (end), hiding the most specific and useful information (e.g., "feature/epic1/story5/enhanced-vis..." shows "feature/epic1/story5/enhanced-vis..." but should show "...story5/enhanced-visualization").

**Acceptance Criteria**:
- AC1: Branch names truncated from prefix (beginning) instead of suffix (end)
- AC2: Most specific part of branch name (suffix) remains visible
- AC3: Truncation logic works consistently across all display contexts
- AC4: Real-time preview during branch creation shows truncated result
- AC5: Truncation respects terminal width constraints

**Technical Implementation**:
- Modify branch truncation logic in `ui/list.go` (lines ~212-219)
- Change from `branch[:remainingWidth-3] + "..."` to `"..." + branch[len(branch)-(remainingWidth-3):]`
- Add real-time preview in project input during branch creation
- Handle edge cases (very short names, exact width matches)

**Definition of Done**:
- [ ] Branch names show suffix when truncated across all UI contexts
- [ ] Real-time preview works during branch name input
- [ ] No regressions in branch display functionality
- [ ] Truncation logic handles all edge cases properly

---

### **Story 1.6: Remove Redundant Project Name Display**
**Priority**: P1 (UX Polish)
**User Story**: As a user, I want clean session displays without redundant information, so I can focus on relevant details.

**Current Problem**: Project names appear twice in session lists - once in the title `[PROJECT] name` and again in the branch line `(PROJECT)`, creating visual redundancy.

**Acceptance Criteria**:
- AC1: Project name appears only once per session (in title line)
- AC2: Branch line shows only branch and repository information
- AC3: Visual hierarchy remains clear and readable
- AC4: No information loss despite redundancy removal

**Technical Implementation**:
- Modify instance renderer in `ui/list.go` (lines ~143-156)
- Remove project name duplication from branch line display
- Maintain clear visual separation between different projects
- Preserve all other branch line information (repo name, etc.)

**Definition of Done**:
- [ ] Project names appear only once per session in lists
- [ ] Branch lines are cleaner without redundant project information
- [ ] Visual hierarchy and readability are maintained
- [ ] No regression in information display

---

### **Story 1.7: MCP Integration for Enhanced Development**
**Priority**: P2 (Feature Enhancement)
**User Story**: As a developer, I want to configure and use MCP (Model Context Protocol) servers for my Claude Code sessions, so I can access enhanced capabilities like GitHub integration, file operations, and project-specific tools.

**Current Problem**: No way to manage or configure MCP servers for individual development sessions, limiting Claude Code's potential capabilities.

**Business Value**:
- Enhanced development productivity through specialized tools
- Project-specific MCP configurations for different codebases
- Seamless integration with existing Claude Code workflow

**Acceptance Criteria**:
- AC1: Session-level MCP configuration and management
- AC2: MCP availability only for Claude-compatible commands (robust detection)
- AC3: Simple toggle interface for adding/removing MCPs per session
- AC4: Automatic session restart with MCP changes (using --continue)
- AC5: Clear indication of MCP status in session lists
- AC6: Security-conscious approach (MCPs disabled for custom commands)

**Technical Implementation**:
- Robust Claude command detection (normalize command, check suffix)
- Session-level MCP configuration generation (`--mcp-config` flag)
- MCP management UI with 'm' key binding
- Automatic session restart logic with conversation continuity
- Security boundaries for custom commands

**Architecture Requirements**:
- `Instance.SessionMCPs []string` field for MCP storage
- Claude command detection: `isClaudeCommand(program)` function
- MCP config file generation in project directory
- Integration with existing session management

**Definition of Done**:
- [ ] MCP toggle interface accessible via 'm' key on sessions
- [ ] MCPs work only with Claude-compatible commands (robust detection)
- [ ] Session restart maintains conversation history via --continue
- [ ] MCP status clearly displayed in session lists
- [ ] Security boundaries prevent MCP usage with custom commands
- [ ] Configuration persists across application restarts

---

## Technical Requirements

### **Error Handling Improvements**
- Implement comprehensive tmux session management
- Add user-friendly error messages for common scenarios
- Add logging for debugging session creation issues
- Implement retry mechanisms for transient failures

### **UI/UX Polish**
- Fix dialog positioning and centering algorithms
- Add active project indicator component
- Improve branch name truncation logic (show suffix, not prefix)
- Remove redundant project name displays
- Maintain existing keyboard shortcuts and functionality

### **MCP Integration Requirements**
- Session-level MCP configuration and management
- Robust Claude command detection for security
- Dynamic MCP configuration file generation
- Session restart logic with conversation continuity
- Clear MCP status indication in UI

### **Quality Standards**
- No regressions in existing functionality
- All new code has test coverage >80%
- Error scenarios are tested and handled
- Performance impact is minimal (<50ms for UI updates)

---

## Implementation Plan

### **Phase 1: Critical Bug Fix (Story 1.2)**
**Timeline**: 1-2 days
**Priority**: Must complete before other work

**Tasks**:
1. Investigate tmux session name collision root cause
2. Implement unique session naming strategy
3. Add session availability checking
4. Implement proper error handling and user messages
5. Add regression tests for session creation scenarios

### **Phase 2: UX Improvements (Stories 1.3, 1.4)**
**Timeline**: 1-2 days
**Priority**: High impact, low complexity

**Tasks**:
1. Fix Add Project dialog centering
2. Add active project display component
3. Integrate active project indicator with UI
4. Test across different screen sizes and resolutions

### **Phase 3: Additional UX Enhancements (Stories 1.5, 1.6, 1.7)**
**Timeline**: 1 day
**Priority**: Polish and feature enhancement

**Tasks**:
1. Fix branch name truncation logic (show suffix) - 1-2 hours
2. Remove redundant project name displays - 30 minutes
3. Implement robust Claude command detection - 1 hour
4. Build session-level MCP configuration system - 2 hours
5. Create MCP management UI interface - 1.5 hours
6. Implement session restart with --continue logic - 1 hour
7. Add MCP status display in session lists - integrated

**Total Additional Work**: 6-8.5 hours (1 development day)

---

## Success Metrics

### **Functional Success**
- **Instance Creation**: 100% success rate (vs current issues)
- **Error Handling**: 0 unclear error messages
- **Visual Consistency**: 100% proper dialog positioning

### **Usability Success**
- **Project Context**: Users immediately understand active project
- **Branch Readability**: Users can identify branches by meaningful suffix
- **Visual Clarity**: Clean displays without redundant information
- **MCP Productivity**: Enhanced development capabilities through MCP integration
- **Professional Polish**: No visual glitches or misalignments

### **Technical Success**
- **Zero Regressions**: All existing functionality preserved
- **Test Coverage**: >80% for all new/modified code
- **Performance**: No noticeable UI performance impact

---

## Risk Assessment

### **Low Risk Items**
- Dialog centering fixes (isolated UI changes)
- Active project display (additive feature)
- Visual hierarchy improvements (styling changes)

### **Medium Risk Items**
- Tmux session name fixes (affects core functionality)
- Session error handling (requires careful testing)

### **Mitigation Strategies**
- Comprehensive testing for session management changes
- Gradual rollout with fallback to previous behavior
- Extensive manual testing across different scenarios

---

## Definition of Done

### **Story Level**
- All acceptance criteria met and tested
- Code review completed and approved
- Test coverage >80% for modified code
- Manual testing completed across scenarios
- No regressions in existing functionality

### **Epic Level**
- All 6 user stories completed and validated (Stories 1.2-1.7)
- User feedback confirms improved experience
- MCP integration enhances development productivity
- Performance benchmarks maintained
- Documentation updated where necessary

---

## Future Considerations

**Explicitly Deferred**:
- Project management operations (rename, delete)
- Advanced keyboard navigation
- Project switching mechanisms
- Smart project discovery features
- Workspace persistence features
- Complex visual hierarchy (original Story 1.5)
- App-level MCP management
- MCP server process lifecycle management

**Potential Next Phase** (if validated as valuable):
- Basic project management (Epic 3.1 subset)
- Simple project switching improvement
- Enhanced error recovery mechanisms
- Advanced MCP features (server health monitoring, custom MCP discovery)
- Project-specific MCP presets and templates

---

## Conclusion

This expanded scope focuses on **fixing critical issues** and **adding high-value enhancements** with controlled complexity. The 6 stories address both immediate UX problems and valuable productivity features that directly impact user experience.

**Expected Outcome**: A robust, polished multi-project experience with enhanced development capabilities through MCP integration. Users will benefit from cleaner UI, better branch visibility, and access to powerful development tools, setting the foundation for future enhancements based on real usage patterns.

---

## Change Log

### Version 1.2 - Scope Expansion and Story Refinement
**Date**: 2025-06-17
**Changes Made**:

#### **Removed Story 1.5 (Enhanced Project Visualization)**:
- **Rationale**: Story was rejected by PO due to lack of business justification and vague acceptance criteria
- **Status**: Moved to explicitly deferred features list
- **Replacement**: Added focused UX improvement stories with clear value

#### **Added New Stories (1.5-1.7)**:
- **Story 1.5**: Fix Branch Name Truncation - show suffix instead of prefix
- **Story 1.6**: Remove Redundant Project Name Display - clean up visual redundancy
- **Story 1.7**: MCP Integration for Enhanced Development - session-level MCP management

#### **Architecture and Technical Requirements**:
- **MCP Integration**: Session-level only, robust Claude command detection
- **Security Boundaries**: MCPs disabled for custom commands
- **Implementation Timeline**: Added 1 day for new stories (3-4 days total, including original Phase 1-2)

#### **Overall Impact**:
- **Expanded Value**: From 4 to 6 stories with clear business benefits
- **Enhanced Productivity**: MCP integration adds significant development capabilities
- **Maintained Simplicity**: UX fixes remain focused and implementable
- **Clear Scope Boundaries**: Explicitly deferred complex features

### Version 1.1 - UX Expert Review Integration
**Date**: Previous
**Changes Made**:

#### **Story 1.4 - Simplified Approach**:
- **Change**: Integrated active project display into "Instances" header instead of separate component
- **Rationale**: Avoid visual clutter in terminal interface
- **New Format**: "Instances (ProjectName)" vs separate "Active: ProjectName" label

#### **Overall Impact**:
- **Reduced Implementation Risk**: Simpler changes with lower complexity
- **Better Terminal UX**: Respects terminal application constraints
- **Maintained Value**: Core user problems still addressed effectively