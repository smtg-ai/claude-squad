package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// SharedKnowledgeRegistry implements a CRDT-based distributed knowledge store
type SharedKnowledgeRegistry struct {
	mu           sync.RWMutex
	entries      map[string]*KnowledgeEntry
	subscriptions map[string][]Subscription
	activeSquads  map[string]time.Time
}

// KnowledgeEntry represents an entry in the shared knowledge registry
type KnowledgeEntry struct {
	Key       string                `json:"key"`
	Value     interface{}           `json:"value"`
	Timestamp VectorClockTimestamp  `json:"timestamp"`
	SquadID   string                `json:"squad_id"`
	Version   uint64                `json:"version"`
	Deleted   bool                  `json:"deleted"`
}

// Subscription represents a subscription to knowledge changes
type Subscription struct {
	ID      string
	Pattern string
	Handler func(string, interface{}, VectorClockTimestamp)
}

// NewSharedKnowledgeRegistry creates a new shared knowledge registry
func NewSharedKnowledgeRegistry() *SharedKnowledgeRegistry {
	return &SharedKnowledgeRegistry{
		entries:       make(map[string]*KnowledgeEntry),
		subscriptions: make(map[string][]Subscription),
		activeSquads:  make(map[string]time.Time),
	}
}

// Put stores a value in the registry with CRDT conflict resolution
func (skr *SharedKnowledgeRegistry) Put(key string, value interface{}, timestamp VectorClockTimestamp) bool {
	skr.mu.Lock()
	defer skr.mu.Unlock()
	
	// Update active squads
	skr.activeSquads[timestamp.SquadID] = time.Now()
	
	entry := &KnowledgeEntry{
		Key:       key,
		Value:     value,
		Timestamp: timestamp,
		SquadID:   timestamp.SquadID,
		Version:   timestamp.Logical,
		Deleted:   false,
	}
	
	// Check if we need to resolve conflicts
	if existing, exists := skr.entries[key]; exists {
		relation := CompareTimestamps(existing.Timestamp, timestamp)
		
		switch relation {
		case Before:
			// New entry is newer, update
			skr.entries[key] = entry
		case After:
			// Existing entry is newer, keep it
			return false
		case Concurrent:
			// Concurrent updates, use deterministic resolution
			if skr.resolveConcurrentUpdate(existing, entry) {
				skr.entries[key] = entry
			} else {
				return false
			}
		case Equal:
			// Same timestamp, update if different squad (shouldn't happen)
			if existing.SquadID != entry.SquadID {
				skr.entries[key] = entry
			}
		}
	} else {
		// New entry
		skr.entries[key] = entry
	}
	
	// Notify subscribers
	skr.notifySubscribers(key, value, timestamp)
	
	return true
}

// Get retrieves a value from the registry
func (skr *SharedKnowledgeRegistry) Get(key string) (interface{}, VectorClockTimestamp, bool) {
	skr.mu.RLock()
	defer skr.mu.RUnlock()
	
	if entry, exists := skr.entries[key]; exists && !entry.Deleted {
		return entry.Value, entry.Timestamp, true
	}
	
	return nil, VectorClockTimestamp{}, false
}

// GetAllEntries returns all entries in the registry
func (skr *SharedKnowledgeRegistry) GetAllEntries() map[string]interface{} {
	skr.mu.RLock()
	defer skr.mu.RUnlock()
	
	result := make(map[string]interface{})
	for key, entry := range skr.entries {
		if !entry.Deleted {
			result[key] = entry.Value
		}
	}
	
	return result
}

// Subscribe subscribes to changes matching a pattern
func (skr *SharedKnowledgeRegistry) Subscribe(pattern string, handler func(string, interface{}, VectorClockTimestamp)) string {
	skr.mu.Lock()
	defer skr.mu.Unlock()
	
	subscriptionID := generateSubscriptionID()
	subscription := Subscription{
		ID:      subscriptionID,
		Pattern: pattern,
		Handler: handler,
	}
	
	skr.subscriptions[pattern] = append(skr.subscriptions[pattern], subscription)
	
	return subscriptionID
}

// Unsubscribe removes a subscription
func (skr *SharedKnowledgeRegistry) Unsubscribe(subscriptionID string) {
	skr.mu.Lock()
	defer skr.mu.Unlock()
	
	for pattern, subs := range skr.subscriptions {
		for i, sub := range subs {
			if sub.ID == subscriptionID {
				// Remove subscription
				skr.subscriptions[pattern] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
	}
}

// GetActiveSquads returns the list of currently active squads
func (skr *SharedKnowledgeRegistry) GetActiveSquads() []string {
	skr.mu.RLock()
	defer skr.mu.RUnlock()
	
	cutoff := time.Now().Add(-5 * time.Minute)
	var active []string
	
	for squad, lastSeen := range skr.activeSquads {
		if lastSeen.After(cutoff) {
			active = append(active, squad)
		}
	}
	
	return active
}

// GetGlobalMessageQueueDepth returns the total message queue depth across all squads
func (skr *SharedKnowledgeRegistry) GetGlobalMessageQueueDepth() int {
	total := 0
	
	// Get message queue depths from all squads
	for _, squad := range skr.GetActiveSquads() {
		if value, _, exists := skr.Get("message_queue:" + squad); exists {
			if depth, ok := value.(int); ok {
				total += depth
			}
		}
	}
	
	return total
}

// Merge merges another registry into this one using CRDT semantics
func (skr *SharedKnowledgeRegistry) Merge(other *SharedKnowledgeRegistry) {
	other.mu.RLock()
	otherEntries := make(map[string]*KnowledgeEntry)
	for k, v := range other.entries {
		otherEntries[k] = v
	}
	other.mu.RUnlock()
	
	for key, entry := range otherEntries {
		skr.Put(key, entry.Value, entry.Timestamp)
	}
}

// Export exports the registry to JSON
func (skr *SharedKnowledgeRegistry) Export() ([]byte, error) {
	skr.mu.RLock()
	defer skr.mu.RUnlock()
	
	export := struct {
		Entries      map[string]*KnowledgeEntry `json:"entries"`
		ActiveSquads map[string]time.Time       `json:"active_squads"`
	}{
		Entries:      skr.entries,
		ActiveSquads: skr.activeSquads,
	}
	
	return json.Marshal(export)
}

// Import imports registry data from JSON
func (skr *SharedKnowledgeRegistry) Import(data []byte) error {
	var imported struct {
		Entries      map[string]*KnowledgeEntry `json:"entries"`
		ActiveSquads map[string]time.Time       `json:"active_squads"`
	}
	
	if err := json.Unmarshal(data, &imported); err != nil {
		return err
	}
	
	skr.mu.Lock()
	defer skr.mu.Unlock()
	
	// Merge entries using CRDT semantics
	for key, entry := range imported.Entries {
		if existing, exists := skr.entries[key]; exists {
			relation := CompareTimestamps(existing.Timestamp, entry.Timestamp)
			if relation == Before || relation == Concurrent {
				skr.entries[key] = entry
			}
		} else {
			skr.entries[key] = entry
		}
	}
	
	// Update active squads
	for squad, lastSeen := range imported.ActiveSquads {
		if existing, exists := skr.activeSquads[squad]; !exists || lastSeen.After(existing) {
			skr.activeSquads[squad] = lastSeen
		}
	}
	
	return nil
}

// Cleanup removes old entries and inactive squads
func (skr *SharedKnowledgeRegistry) Cleanup() {
	skr.mu.Lock()
	defer skr.mu.Unlock()
	
	cutoff := time.Now().Add(-1 * time.Hour)
	
	// Remove inactive squads
	for squad, lastSeen := range skr.activeSquads {
		if lastSeen.Before(cutoff) {
			delete(skr.activeSquads, squad)
		}
	}
	
	// Remove old entries (keep only last 1000 per squad)
	squadCounts := make(map[string]int)
	for _, entry := range skr.entries {
		squadCounts[entry.SquadID]++
	}
	
	// If any squad has more than 1000 entries, remove oldest
	for squad, count := range squadCounts {
		if count > 1000 {
			// This is a simplified cleanup - in practice we'd sort by timestamp
			removed := 0
			for key, entry := range skr.entries {
				if entry.SquadID == squad && removed < (count-1000) {
					delete(skr.entries, key)
					removed++
				}
			}
		}
	}
}

// resolveConcurrentUpdate resolves concurrent updates using deterministic rules
func (skr *SharedKnowledgeRegistry) resolveConcurrentUpdate(existing, new *KnowledgeEntry) bool {
	// Use lexicographic ordering of squad IDs as tiebreaker
	if existing.SquadID < new.SquadID {
		return false // Keep existing
	} else if existing.SquadID > new.SquadID {
		return true // Use new
	}
	
	// Same squad ID (shouldn't happen), use higher version
	return new.Version > existing.Version
}

// notifySubscribers notifies all matching subscribers of a change
func (skr *SharedKnowledgeRegistry) notifySubscribers(key string, value interface{}, timestamp VectorClockTimestamp) {
	for pattern, subs := range skr.subscriptions {
		if matchesPattern(key, pattern) {
			for _, sub := range subs {
				go sub.Handler(key, value, timestamp)
			}
		}
	}
}

// Helper functions

func generateSubscriptionID() string {
	return fmt.Sprintf("sub_%d", time.Now().UnixNano())
}

func matchesPattern(key, pattern string) bool {
	// Simple pattern matching - in practice this would be more sophisticated
	if pattern == "*" {
		return true
	}
	
	if strings.HasSuffix(pattern, "*") {
		prefix := pattern[:len(pattern)-1]
		return strings.HasPrefix(key, prefix)
	}
	
	return key == pattern
}