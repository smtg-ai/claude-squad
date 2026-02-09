//go:build windows

package session

import (
	"claude-squad/cmd"
	"claude-squad/session/winterminal"
)

// NewTerminalSession creates a new terminal session for the current platform.
// On Windows, this returns a Windows Terminal-backed session.
func NewTerminalSession(name, program string) TerminalSession {
	return winterminal.NewWindowsTerminalSession(name, program)
}

// NewTerminalSessionWithDeps creates a new terminal session with provided dependencies for testing.
func NewTerminalSessionWithDeps(name, program string, cmdExec cmd.Executor) TerminalSession {
	return winterminal.NewWindowsTerminalSessionWithDeps(name, program, cmdExec)
}

// CleanupTerminalSessions cleans up all terminal sessions created by claude-squad.
func CleanupTerminalSessions(cmdExec cmd.Executor) error {
	return winterminal.CleanupSessions(cmdExec)
}
