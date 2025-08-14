# Claude-Squad UI Freezing Issue

## Problem
When creating a new session in claude-squad, the UI freezes after entering the session name. This happens because several blocking operations are executed synchronously in the main UI thread, including:

1. Git worktree creation
2. Tmux session setup 
3. File system operations
4. External process execution

These operations can take several seconds to complete, during which the UI becomes unresponsive.

## Solution Approach

The solution is to make session creation asynchronous by:

1. Moving blocking operations to background goroutines
2. Adding a progress indicator or spinner to provide visual feedback during session creation
3. Updating the UI state only after background operations complete

### Implementation Steps

1. Modify the session creation flow in `app.go` to launch a goroutine for the blocking operations
2. Implement a state machine to track session creation progress
3. Add a visual progress indicator during session creation
4. Use channels to communicate between the background goroutine and the main UI thread
5. Handle errors from the background operations gracefully
6. Ensure proper cleanup if the user cancels session creation

## Benefits

- UI will remain responsive during session creation
- Users will have visual feedback on the progress
- Better error handling and recovery from failures
- Improved overall user experience