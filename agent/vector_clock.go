package agent

import (
	"sync"
	"time"
)

// VectorClock implements distributed causality tracking
type VectorClock struct {
	mu        sync.RWMutex
	clocks    map[string]uint64
	squadID   string
	logical   uint64
	StartTime time.Time
}

// VectorClockTimestamp represents a point in the vector clock
type VectorClockTimestamp struct {
	SquadID   string            `json:"squad_id"`
	Logical   uint64            `json:"logical"`
	Clocks    map[string]uint64 `json:"clocks"`
	Physical  time.Time         `json:"physical"`
}

// NewVectorClock creates a new vector clock for the given squad
func NewVectorClock(squadID string) *VectorClock {
	return &VectorClock{
		clocks:    make(map[string]uint64),
		squadID:   squadID,
		logical:   0,
		StartTime: time.Now(),
	}
}

// Tick increments the logical clock
func (vc *VectorClock) Tick() VectorClockTimestamp {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	
	vc.logical++
	vc.clocks[vc.squadID] = vc.logical
	
	return VectorClockTimestamp{
		SquadID:  vc.squadID,
		Logical:  vc.logical,
		Clocks:   vc.copyClocks(),
		Physical: time.Now(),
	}
}

// Now returns the current timestamp
func (vc *VectorClock) Now() VectorClockTimestamp {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	
	return VectorClockTimestamp{
		SquadID:  vc.squadID,
		Logical:  vc.logical,
		Clocks:   vc.copyClocks(),
		Physical: time.Now(),
	}
}

// Update updates the vector clock with a received timestamp
func (vc *VectorClock) Update(timestamp VectorClockTimestamp) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	
	// Update each clock to the maximum
	for squad, clock := range timestamp.Clocks {
		if current, exists := vc.clocks[squad]; !exists || clock > current {
			vc.clocks[squad] = clock
		}
	}
	
	// Increment our own logical clock
	vc.logical++
	vc.clocks[vc.squadID] = vc.logical
}

// GetState returns the current state of the vector clock
func (vc *VectorClock) GetState() map[string]interface{} {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	
	return map[string]interface{}{
		"squad_id": vc.squadID,
		"logical":  vc.logical,
		"clocks":   vc.copyClocks(),
		"uptime":   time.Since(vc.StartTime),
	}
}

// Optimize performs vector clock optimization
func (vc *VectorClock) Optimize() {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	
	// Remove stale entries (older than 1 hour)
	cutoff := time.Now().Add(-1 * time.Hour)
	for squad := range vc.clocks {
		if squad != vc.squadID {
			// This is a simplified optimization
			// In practice, we'd track last seen times
			if time.Since(vc.StartTime) > time.Hour {
				delete(vc.clocks, squad)
			}
		}
	}
}

// copyClocks creates a copy of the current clocks map
func (vc *VectorClock) copyClocks() map[string]uint64 {
	copy := make(map[string]uint64)
	for k, v := range vc.clocks {
		copy[k] = v
	}
	return copy
}

// CompareTimestamps compares two vector clock timestamps
func CompareTimestamps(a, b VectorClockTimestamp) Relation {
	aLessB := true
	bLessA := true
	
	// Collect all squad IDs
	allSquads := make(map[string]bool)
	for squad := range a.Clocks {
		allSquads[squad] = true
	}
	for squad := range b.Clocks {
		allSquads[squad] = true
	}
	
	// Compare each squad's clock
	for squad := range allSquads {
		aValue := a.Clocks[squad]
		bValue := b.Clocks[squad]
		
		if aValue > bValue {
			bLessA = false
		}
		if bValue > aValue {
			aLessB = false
		}
	}
	
	if aLessB && bLessA {
		return Equal
	} else if aLessB {
		return Before
	} else if bLessA {
		return After
	} else {
		return Concurrent
	}
}

// Relation represents the relationship between two timestamps
type Relation int

const (
	Before Relation = iota
	After
	Equal
	Concurrent
)