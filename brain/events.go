package brain

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

var errSubscriberNotFound = errors.New("subscriber not found")

// EventType identifies the kind of event.
type EventType string

const (
	EventStatusChanged         EventType = "status_changed"
	EventMessageReceived       EventType = "message_received"
	EventAgentRemoved          EventType = "agent_removed"
	EventWorkflowDefined       EventType = "workflow_defined"
	EventTaskCompleted         EventType = "task_completed"
	EventTaskTriggered         EventType = "task_triggered"
	EventInstanceStatusChanged EventType = "instance_status_changed"
	EventInstanceCreated       EventType = "instance_created"
	EventInstanceKilled        EventType = "instance_killed"
)

// Event is a single occurrence pushed to subscribers.
type Event struct {
	Type      EventType      `json:"type"`
	Timestamp time.Time      `json:"timestamp"`
	RepoPath  string         `json:"repo_path"`
	Source    string         `json:"source"`
	Data      map[string]any `json:"data,omitempty"`
	Sequence  uint64         `json:"sequence"`
}

// EventFilter controls which events a subscriber receives.
type EventFilter struct {
	Types       []EventType `json:"types,omitempty"`
	Instances   []string    `json:"instances,omitempty"`
	ParentTitle string      `json:"parent_title,omitempty"`
}

// Subscriber holds a buffered queue of events for a single consumer.
type Subscriber struct {
	ID       string
	Filter   EventFilter
	buffer   []Event
	notify   chan struct{}
	lastPoll time.Time
	mu       sync.Mutex
}

// EventBus fans events out to matching subscribers with per-subscriber buffering.
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[string]*Subscriber
	sequence    atomic.Uint64
	maxBuffer   int
}

// NewEventBus creates an EventBus. maxBuffer caps each subscriber's queue.
func NewEventBus(maxBuffer int) *EventBus {
	if maxBuffer <= 0 {
		maxBuffer = 1000
	}
	return &EventBus{
		subscribers: make(map[string]*Subscriber),
		maxBuffer:   maxBuffer,
	}
}

// Subscribe creates a new subscriber with the given filter and returns its ID.
func (eb *EventBus) Subscribe(filter EventFilter) string {
	id := generateID()
	sub := &Subscriber{
		ID:       id,
		Filter:   filter,
		notify:   make(chan struct{}, 1),
		lastPoll: time.Now(),
	}

	eb.mu.Lock()
	eb.subscribers[id] = sub
	eb.mu.Unlock()

	return id
}

// Emit publishes an event to all matching subscribers.
func (eb *EventBus) Emit(event Event) {
	event.Sequence = eb.sequence.Add(1)
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	eb.mu.RLock()
	defer eb.mu.RUnlock()

	for _, sub := range eb.subscribers {
		if !matchesFilter(event, sub.Filter) {
			continue
		}

		sub.mu.Lock()
		sub.buffer = append(sub.buffer, event)
		if len(sub.buffer) > eb.maxBuffer {
			sub.buffer = sub.buffer[len(sub.buffer)-eb.maxBuffer:]
		}
		sub.mu.Unlock()

		// Non-blocking signal.
		select {
		case sub.notify <- struct{}{}:
		default:
		}
	}
}

// Poll drains the subscriber's buffer. If empty, blocks until events arrive or timeout.
func (eb *EventBus) Poll(subscriberID string, timeout time.Duration) ([]Event, error) {
	eb.mu.RLock()
	sub, ok := eb.subscribers[subscriberID]
	eb.mu.RUnlock()
	if !ok {
		return nil, errSubscriberNotFound
	}

	sub.mu.Lock()
	sub.lastPoll = time.Now()
	if len(sub.buffer) > 0 {
		events := sub.buffer
		sub.buffer = nil
		sub.mu.Unlock()
		return events, nil
	}
	sub.mu.Unlock()

	// Wait for events or timeout.
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-sub.notify:
	case <-timer.C:
	}

	sub.mu.Lock()
	events := sub.buffer
	sub.buffer = nil
	sub.mu.Unlock()

	return events, nil
}

// Unsubscribe removes a subscriber.
func (eb *EventBus) Unsubscribe(subscriberID string) {
	eb.mu.Lock()
	delete(eb.subscribers, subscriberID)
	eb.mu.Unlock()
}

// PruneStale removes subscribers that haven't polled within maxAge. Returns count removed.
func (eb *EventBus) PruneStale(maxAge time.Duration) int {
	cutoff := time.Now().Add(-maxAge)
	var stale []string

	eb.mu.RLock()
	for id, sub := range eb.subscribers {
		sub.mu.Lock()
		if sub.lastPoll.Before(cutoff) {
			stale = append(stale, id)
		}
		sub.mu.Unlock()
	}
	eb.mu.RUnlock()

	if len(stale) == 0 {
		return 0
	}

	eb.mu.Lock()
	for _, id := range stale {
		delete(eb.subscribers, id)
	}
	eb.mu.Unlock()

	return len(stale)
}

// matchesFilter returns true if the event passes all filter criteria.
func matchesFilter(event Event, filter EventFilter) bool {
	if len(filter.Types) > 0 && !sliceContains(filter.Types, event.Type) {
		return false
	}
	if len(filter.Instances) > 0 && !sliceContains(filter.Instances, event.Source) {
		return false
	}
	if filter.ParentTitle != "" {
		pt, _ := event.Data["parent_title"].(string)
		if pt != filter.ParentTitle {
			return false
		}
	}
	return true
}

// sliceContains reports whether needle is present in haystack.
func sliceContains[T comparable](haystack []T, needle T) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "sub_" + hex.EncodeToString(b)
}
