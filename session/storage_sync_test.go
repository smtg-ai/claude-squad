package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"claude-squad/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// realStorage returns a Storage backed by the REAL config.State against a
// temporary home directory.
//
// It deliberately does not use a fake. An earlier attempt at this merge passed
// six tests against a fake whose GetInstances returned whatever had been seeded
// -- i.e. a fake that behaved like a fresh disk read. The real State returns an
// in-memory field loaded once, so the fake encoded the assumption under test and
// the suite measured nothing. Only the real type can catch that.
func realStorage(t *testing.T) *Storage {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	s, err := NewStorage(config.LoadState())
	require.NoError(t, err)
	return s
}

// writeStateFileDirectly simulates a second writer: another interface, or the
// `new` subcommand, appending to the state file behind this process's back.
func writeStateFileDirectly(t *testing.T, data ...InstanceData) {
	t.Helper()
	dir, err := config.GetConfigDir()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(dir, 0o755))

	raw, err := json.Marshal(data)
	require.NoError(t, err)

	state := config.LoadState()
	require.NoError(t, state.SaveInstances(raw))
}

// titlesOnDisk reads the state FILE, bypassing any in-memory copy.
func titlesOnDisk(t *testing.T) []string {
	t.Helper()
	dir, err := config.GetConfigDir()
	require.NoError(t, err)

	raw, err := os.ReadFile(filepath.Join(dir, config.StateFileName))
	require.NoError(t, err)

	var wrapper struct {
		Instances json.RawMessage `json:"instances"`
	}
	require.NoError(t, json.Unmarshal(raw, &wrapper))
	if len(wrapper.Instances) == 0 {
		return nil
	}

	var data []InstanceData
	require.NoError(t, json.Unmarshal(wrapper.Instances, &data))
	titles := make([]string, 0, len(data))
	for _, d := range data {
		titles = append(titles, d.Title)
	}
	return titles
}

func started(title, branch string) *Instance {
	return &Instance{Title: title, Branch: branch, Status: Paused, started: true}
}

func TestSyncInstancesAgainstRealState(t *testing.T) {
	t.Run("keeps a session a second writer added after this process loaded", func(t *testing.T) {
		s := realStorage(t) // loads state ONCE, as the interface does at startup

		// Second writer appends while this process holds its own view.
		writeStateFileDirectly(t, InstanceData{Title: "fromCLI", Status: Paused})

		require.NoError(t, s.SyncInstances([]*Instance{started("mine", "b1")}))

		assert.ElementsMatch(t, []string{"mine", "fromCLI"}, titlesOnDisk(t),
			"a session created by another writer must survive this process's save")
	})

	t.Run("MUTATION CONTROL: SaveInstances loses it", func(t *testing.T) {
		// Proves the test above can fail. If this ever reports both titles, the
		// test above has stopped measuring anything.
		s := realStorage(t)
		writeStateFileDirectly(t, InstanceData{Title: "fromCLI", Status: Paused})

		require.NoError(t, s.SaveInstances([]*Instance{started("mine", "b1")}))

		assert.Equal(t, []string{"mine"}, titlesOnDisk(t),
			"SaveInstances is verbatim by design -- DeleteInstance depends on it")
	})

	t.Run("the caller's copy wins for a title it holds", func(t *testing.T) {
		s := realStorage(t)
		writeStateFileDirectly(t, InstanceData{Title: "shared", Branch: "stale", Status: Paused})

		require.NoError(t, s.SyncInstances([]*Instance{started("shared", "fresh")}))

		assert.Equal(t, []string{"shared"}, titlesOnDisk(t), "no duplicate for one title")
	})

	t.Run("unstarted instances are skipped, as SaveInstances does", func(t *testing.T) {
		s := realStorage(t)

		require.NoError(t, s.SyncInstances([]*Instance{{Title: "half-built", Status: Paused}}))

		assert.Empty(t, titlesOnDisk(t))
	})

	t.Run("empty state is not an error", func(t *testing.T) {
		s := realStorage(t)

		require.NoError(t, s.SyncInstances([]*Instance{started("first", "b1")}))

		assert.Equal(t, []string{"first"}, titlesOnDisk(t))
	})

	t.Run("REGRESSION: merging inside SaveInstances would resurrect deletions", func(t *testing.T) {
		// DeleteInstance computes the exact list it wants and hands it to
		// SaveInstances. If that merged with disk the deleted entry would return,
		// which is why the merge lives in SyncInstances instead.
		// Seeded THROUGH this Storage, not behind its back: DeleteInstance reads
		// via s.state, which is an in-memory copy, so an external write would not
		// be visible to it. That staleness is real but it is a separate defect --
		// this guard is about SaveInstances staying verbatim.
		s := realStorage(t)
		require.NoError(t, s.SaveInstances([]*Instance{
			started("keep", "b1"),
			started("drop", "b2"),
		}))

		require.NoError(t, s.DeleteInstance("drop"))

		assert.Equal(t, []string{"keep"}, titlesOnDisk(t),
			"delete must remove the entry, not merge it back")
	})
}
