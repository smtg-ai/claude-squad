package session

// TerminalSession is the interface for managing terminal sessions.
// On Unix this is backed by tmux, on Windows by Windows Terminal.
type TerminalSession interface {
	Start(workDir string) error
	Restore() error
	Close() error
	CapturePaneContent() (string, error)
	CapturePaneContentWithOptions(start, end string) (string, error)
	HasUpdated() (updated bool, hasPrompt bool)
	TapEnter() error
	SendKeys(keys string) error
	Attach() (chan struct{}, error)
	SetDetachedSize(width, height int) error
	DoesSessionExist() bool
	DetachSafely() error
}
