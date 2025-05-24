package tmux

import (
	"os"
	"os/exec"

	"github.com/creack/pty"
)

type PtyFactory interface {
	Start(cmd *exec.Cmd) (*os.File, error)
	Close()
}

// Pty starts a "real" pseudo-terminal (PTY) using the creack/pty package.
type Pty struct{}

func (pt Pty) Start(cmd *exec.Cmd) (*os.File, error) {
	return pty.Start(cmd)
}

func (pt Pty) Close() {}

func MakePtyFactory() PtyFactory {
	return Pty{}
}
