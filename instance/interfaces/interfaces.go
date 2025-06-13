package interfaces

import "claude-squad/keys"

// Instance defines the core interface for all instance types
type Instance interface {
	StatusText() string
	MenuItems() []keys.KeyName
	Serialize() []byte
	Deserialize([]byte) error

	IsRunning() bool

	// Core instance operations
	Kill() error
	Attach() (chan struct{}, error)
	SetPreviewSize(width, height int) error
}
