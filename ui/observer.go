package ui

import "claude-squad/instance"

// InstanceObserver defines the interface for observing instance changes
type InstanceObserver interface {
	// OnInstancesChanged is called when the instances list changes
	OnInstancesChanged(instances []instance.Instance)
	// OnInstanceSelected is called when a different instance is selected
	OnInstanceSelected(selectedIdx int)
}
