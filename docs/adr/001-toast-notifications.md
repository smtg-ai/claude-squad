# ADR-001: Toast Notification System for Git Operations

## Status
Accepted

## Context
Users requested better feedback during git operations to improve perceived performance and user confidence. The existing error handling system only showed errors, but provided no feedback during long-running operations like branch sync, worktree creation, and git push operations.

GitHub Issue: [#209 - Add toast notifications for better user feedback during git operations](https://github.com/smtg-ai/claude-squad/issues/209)

## Decision
We will implement a toast notification system that provides real-time, non-intrusive feedback during git operations.

### Architecture
```
ui/overlay/toast.go
├── ToastManager - Queue management and lifecycle
├── Toast - Individual notification with type, duration, message
└── ToastStyles - Visual styling for different types
```

### Integration Points
- **Git Push Operations** (`app.go:756-774`) - Progress and result feedback
- **Branch Sync Operations** (`app.go:974-987`) - Source/target sync status  
- **Branch Creation** (`app.go:932-967`) - Worktree creation feedback
- **Session Deletion** (`app.go:720-754`) - Cleanup progress
- **Resume Operations** (`app.go:811-817`) - Worktree setup feedback during session resume
- **Pause/Checkout Operations** (`app.go:796-802`) - Worktree cleanup feedback during session pause
- **Error States** - User-friendly error communication

### Toast Types
1. **Info** (blue) - Operation in progress (3s duration, configurable)
2. **Success** (green) - Successful completion (3s duration, configurable)  
3. **Warning** (orange) - Non-critical issues (3s duration, configurable)
4. **Error** (red) - Failures requiring attention (3s duration, configurable)

### Configuration
Toast timeout durations are configurable through `~/.claude-squad/config.json`:

```json
{
  "toast_timeouts": {
    "info": 3000,    // milliseconds (3 seconds default)
    "success": 3000, // milliseconds (3 seconds default) 
    "warning": 3000, // milliseconds (3 seconds default)
    "error": 3000    // milliseconds (3 seconds default)
  }
}
```

### Technical Implementation
- **Queue System**: Maximum 5 toasts, newest first
- **Auto-dismiss**: Type-specific durations with smooth removal
- **Positioning**: Fixed coordinate (80, 1) for reliable visibility
- **Update Cycle**: 100ms refresh rate for smooth animations
- **Integration**: Added to main UI render cycle with overlay system

## Alternatives Considered

### 1. Progress Bars
- **Pros**: More detailed progress indication
- **Cons**: Complex to implement, requires operation progress tracking
- **Decision**: Rejected due to complexity and inconsistent git operation progress

### 2. Status Line Updates  
- **Pros**: Simple to implement
- **Cons**: Not prominent enough for important feedback, easily missed
- **Decision**: Rejected for insufficient user visibility

### 3. Modal Dialogs
- **Pros**: Guaranteed user attention
- **Cons**: Intrusive, blocks user interaction during operations
- **Decision**: Rejected as too disruptive to workflow

### 4. Logging Panel
- **Pros**: Persistent history
- **Cons**: Takes screen real estate, requires new UI component
- **Decision**: Rejected for complexity and space constraints

## Benefits
- **Improved UX**: Users receive immediate feedback on long-running operations
- **Error Clarity**: Better error communication with context-specific messages
- **Non-intrusive**: Toasts don't block user interaction
- **Consistent**: Unified feedback approach across all git operations
- **Accessible**: Built on existing lipgloss styling system for consistency

## Risks and Mitigation
- **Screen Space**: Fixed positioning may not work on all terminal sizes
  - *Mitigation*: Fallback to left edge if toast is too wide
- **Performance**: 100ms update cycle could impact performance
  - *Mitigation*: Lightweight string operations, bounded queue size
- **Visibility**: Toasts may be missed by users
  - *Mitigation*: Appropriate durations, high contrast colors

## Implementation Details

### File Structure
```
ui/overlay/toast.go           # Core toast system
app/app.go                   # Integration and rendering
app/branch_sync_test.go      # Updated tests with toast manager
app/app_test.go             # Updated tests with toast manager  
```

### Key Functions
- `NewToastManager(config)` - Initialize toast system with configurable timeouts
- `AddInfoToast(message)` - Add info notification
- `AddSuccessToast(message)` - Add success notification  
- `AddErrorToast(message)` - Add error notification
- `UpdateToasts()` - Remove expired toasts
- `Render()` - Generate visual output

### Testing Strategy
- Unit tests verify toast lifecycle and queue management
- Integration tests ensure toasts appear during git operations
- All existing tests updated with toast manager initialization

## Future Considerations
- **Dynamic Positioning**: Calculate optimal position based on terminal size
- **Toast Interactions**: Click-to-dismiss or action buttons
- **Persistence**: Optional toast history panel
- **Advanced Configuration**: User-configurable positioning and styles
- **Sound**: Audio feedback for accessibility (terminal permitting)

## References
- [GitHub Issue #209](https://github.com/smtg-ai/claude-squad/issues/209)
- [Bubble Tea Documentation](https://github.com/charmbracelet/bubbletea)
- [Lipgloss Styling](https://github.com/charmbracelet/lipgloss)