package config

import (
	"claude-squad/log"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
)

const (
	StateFileName     = "state.json"
	InstancesFileName = "instances.json"
	stateLockFileName = "state.json.lock"
)

// InstanceStorage handles instance-related operations
type InstanceStorage interface {
	// UpdateInstances atomically updates the stored instance data. The update
	// function receives the instance data currently on disk and returns the
	// data to store. The read-modify-write runs under an exclusive file lock,
	// so instances saved by concurrent claude-squad processes are not lost.
	UpdateInstances(update func(onDisk json.RawMessage) (json.RawMessage, error)) error
	// GetInstances returns the raw instance data
	GetInstances() json.RawMessage
	// DeleteAllInstances removes all stored instances
	DeleteAllInstances() error
}

// AppState handles application-level state
type AppState interface {
	// GetHelpScreensSeen returns the bitmask of seen help screens
	GetHelpScreensSeen() uint32
	// SetHelpScreensSeen updates the bitmask of seen help screens
	SetHelpScreensSeen(seen uint32) error
}

// StateManager combines instance storage and app state management
type StateManager interface {
	InstanceStorage
	AppState
}

// State represents the application state that persists between sessions
type State struct {
	// HelpScreensSeen is a bitmask tracking which help screens have been shown
	HelpScreensSeen uint32 `json:"help_screens_seen"`
	// Instances stores the serialized instance data as raw JSON
	InstancesData json.RawMessage `json:"instances"`
}

// DefaultState returns the default state
func DefaultState() *State {
	return &State{
		HelpScreensSeen: 0,
		InstancesData:   json.RawMessage("[]"),
	}
}

// LoadState loads the state from disk. If it cannot be done, we return the default state.
func LoadState() *State {
	configDir, err := GetConfigDir()
	if err != nil {
		log.ErrorLog.Printf("failed to get config directory: %v", err)
		return DefaultState()
	}

	statePath := filepath.Join(configDir, StateFileName)
	state, err := readState(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create and save default state if file doesn't exist
			defaultState := DefaultState()
			if saveErr := SaveState(defaultState); saveErr != nil {
				log.WarningLog.Printf("failed to save default state: %v", saveErr)
			}
			return defaultState
		}

		log.WarningLog.Printf("failed to load state file: %v", err)
		return DefaultState()
	}

	return state
}

// readState reads and parses the state file at the given path.
func readState(statePath string) (*State, error) {
	data, err := os.ReadFile(statePath)
	if err != nil {
		return nil, err
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	return &state, nil
}

// SaveState saves the state to disk
func SaveState(state *State) error {
	configDir, err := GetConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	return writeState(filepath.Join(configDir, StateFileName), state)
}

// writeState writes the state file atomically via a temp file and rename, so
// a crash mid-write cannot leave a truncated state file behind.
func writeState(statePath string, state *State) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(statePath), StateFileName+".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp state file: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to write temp state file: %w", err)
	}
	if err := tmp.Chmod(0644); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to chmod temp state file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temp state file: %w", err)
	}

	if err := os.Rename(tmp.Name(), statePath); err != nil {
		return fmt.Errorf("failed to rename temp state file: %w", err)
	}
	return nil
}

// lockedUpdate re-reads the state from disk under an exclusive file lock,
// applies mutate to it, writes the result back atomically, and refreshes s
// from what was written. Merging with the on-disk state instead of writing
// this process's cached copy means concurrent claude-squad processes (other
// TUIs, the daemon) only change the fields they mean to change, rather than
// clobbering the whole file with a stale snapshot.
func (s *State) lockedUpdate(mutate func(disk *State) error) error {
	configDir, err := GetConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	fileLock := flock.New(filepath.Join(configDir, stateLockFileName))
	if err := fileLock.Lock(); err != nil {
		return fmt.Errorf("failed to lock state file: %w", err)
	}
	defer func() {
		if err := fileLock.Unlock(); err != nil {
			log.ErrorLog.Printf("failed to unlock state file: %v", err)
		}
	}()

	statePath := filepath.Join(configDir, StateFileName)
	disk, err := readState(statePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.WarningLog.Printf("failed to read state file, using defaults: %v", err)
		}
		disk = DefaultState()
	}

	if err := mutate(disk); err != nil {
		return err
	}
	if err := writeState(statePath, disk); err != nil {
		return err
	}

	*s = *disk
	return nil
}

// InstanceStorage interface implementation

// UpdateInstances atomically updates the stored instance data under a file lock.
func (s *State) UpdateInstances(update func(onDisk json.RawMessage) (json.RawMessage, error)) error {
	return s.lockedUpdate(func(disk *State) error {
		updated, err := update(disk.InstancesData)
		if err != nil {
			return err
		}
		disk.InstancesData = updated
		return nil
	})
}

// GetInstances returns the raw instance data
func (s *State) GetInstances() json.RawMessage {
	return s.InstancesData
}

// DeleteAllInstances removes all stored instances
func (s *State) DeleteAllInstances() error {
	return s.lockedUpdate(func(disk *State) error {
		disk.InstancesData = json.RawMessage("[]")
		return nil
	})
}

// AppState interface implementation

// GetHelpScreensSeen returns the bitmask of seen help screens
func (s *State) GetHelpScreensSeen() uint32 {
	return s.HelpScreensSeen
}

// SetHelpScreensSeen updates the bitmask of seen help screens
func (s *State) SetHelpScreensSeen(seen uint32) error {
	return s.lockedUpdate(func(disk *State) error {
		disk.HelpScreensSeen = seen
		return nil
	})
}
