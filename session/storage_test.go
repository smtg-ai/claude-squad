package session

import (
	"claude-squad/config"
	"testing"
)

// newTestStorage simulates a claude-squad process starting up: it loads the
// state file from disk and wraps it in a Storage.
func newTestStorage(t *testing.T) *Storage {
	t.Helper()
	storage, err := NewStorage(config.LoadState())
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	return storage
}

// newPausedInstance builds a started instance without requiring tmux or git.
// Paused instances restore without touching either subsystem.
func newPausedInstance(t *testing.T, title string) *Instance {
	t.Helper()
	instance, err := FromInstanceData(InstanceData{
		Title:   title,
		Path:    t.TempDir(),
		Status:  Paused,
		Program: "claude",
	})
	if err != nil {
		t.Fatalf("failed to create instance %q: %v", title, err)
	}
	return instance
}

func loadTitles(t *testing.T) []string {
	t.Helper()
	instances, err := newTestStorage(t).LoadInstances()
	if err != nil {
		t.Fatalf("failed to load instances: %v", err)
	}
	titles := make([]string, len(instances))
	for i, instance := range instances {
		titles[i] = instance.Title
	}
	return titles
}

func TestSaveInstancesPreservesInstancesSavedByOtherProcesses(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// Two processes load state at startup, before either has saved anything.
	storageA := newTestStorage(t)
	storageB := newTestStorage(t)

	if err := storageA.SaveInstances([]*Instance{newPausedInstance(t, "one")}); err != nil {
		t.Fatalf("storage A failed to save: %v", err)
	}

	// Process B never saw "one"; saving its own instance must not erase it.
	if err := storageB.SaveInstances([]*Instance{newPausedInstance(t, "two")}); err != nil {
		t.Fatalf("storage B failed to save: %v", err)
	}

	titles := loadTitles(t)
	if len(titles) != 2 || titles[0] != "one" || titles[1] != "two" {
		t.Fatalf("expected instances [one two] to survive, got %v", titles)
	}
}

func TestSaveInstancesUpdatesExistingInstance(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	storage := newTestStorage(t)
	instance := newPausedInstance(t, "one")
	if err := storage.SaveInstances([]*Instance{instance}); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	instance.Height = 42
	if err := storage.SaveInstances([]*Instance{instance}); err != nil {
		t.Fatalf("failed to re-save: %v", err)
	}

	instances, err := newTestStorage(t).LoadInstances()
	if err != nil {
		t.Fatalf("failed to load instances: %v", err)
	}
	if len(instances) != 1 {
		t.Fatalf("expected 1 instance after re-save, got %d", len(instances))
	}
	if instances[0].Height != 42 {
		t.Fatalf("expected re-save to update the stored instance")
	}
}

func TestDeleteInstanceRemovesInstanceSavedByThisProcess(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	storage := newTestStorage(t)
	err := storage.SaveInstances([]*Instance{
		newPausedInstance(t, "one"),
		newPausedInstance(t, "two"),
	})
	if err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	if err := storage.DeleteInstance("one"); err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	titles := loadTitles(t)
	if len(titles) != 1 || titles[0] != "two" {
		t.Fatalf("expected only [two] after delete, got %v", titles)
	}
}

func TestDeleteInstanceRemovesInstanceSavedByAnotherProcess(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	storageA := newTestStorage(t)
	if err := storageA.SaveInstances([]*Instance{newPausedInstance(t, "one")}); err != nil {
		t.Fatalf("storage A failed to save: %v", err)
	}

	// Process B starts after A's save, so it sees "one" on disk.
	storageB := newTestStorage(t)
	if err := storageB.DeleteInstance("one"); err != nil {
		t.Fatalf("storage B failed to delete: %v", err)
	}

	if titles := loadTitles(t); len(titles) != 0 {
		t.Fatalf("expected no instances after delete, got %v", titles)
	}
}
