package concurrency

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Event represents a single event in the system
type Event struct {
	ID        string
	Type      string
	Payload   interface{}
	Timestamp time.Time
	Source    string
}

// Subscriber interface for event consumers
type Subscriber interface {
	// OnEvent is called when an event is delivered to this subscriber
	OnEvent(event *Event)
	// ID returns a unique identifier for this subscriber
	ID() string
}

// EventFilter is a function that determines if an event should be delivered
type EventFilter func(*Event) bool

// BackpressureStrategy defines how to handle slow consumers
type BackpressureStrategy int

const (
	// Drop - drop events when subscriber is slow
	Drop BackpressureStrategy = iota
	// Block - block publisher when subscriber is slow
	Block
	// Buffer - buffer events up to a limit, dropping oldest when full
	Buffer
)

// CircularBuffer stores a fixed number of recent events
type CircularBuffer struct {
	mu     sync.RWMutex
	events []*Event
	head   int
	tail   int
	size   int
	count  int
}

// NewCircularBuffer creates a new circular buffer with the specified capacity
func NewCircularBuffer(size int) *CircularBuffer {
	if size <= 0 {
		size = 100
	}
	return &CircularBuffer{
		events: make([]*Event, size),
		size:   size,
	}
}

// Add adds an event to the buffer, overwriting oldest if full
func (cb *CircularBuffer) Add(event *Event) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.events[cb.tail] = event
	cb.tail = (cb.tail + 1) % cb.size

	if cb.count < cb.size {
		cb.count++
	} else {
		// Buffer is full, move head forward
		cb.head = (cb.head + 1) % cb.size
	}
}

// GetAll returns all events in the buffer in chronological order
func (cb *CircularBuffer) GetAll() []*Event {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.count == 0 {
		return nil
	}

	result := make([]*Event, 0, cb.count)

	if cb.head < cb.tail {
		// Simple case: head to tail
		result = append(result, cb.events[cb.head:cb.tail]...)
	} else if cb.count == cb.size {
		// Buffer wrapped around
		result = append(result, cb.events[cb.head:]...)
		result = append(result, cb.events[:cb.tail]...)
	} else {
		// Not wrapped yet
		result = append(result, cb.events[cb.head:cb.tail]...)
	}

	return result
}

// GetSince returns events since a specific timestamp
func (cb *CircularBuffer) GetSince(since time.Time) []*Event {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	var result []*Event
	all := cb.getAllUnsafe()

	for _, event := range all {
		if event != nil && event.Timestamp.After(since) {
			result = append(result, event)
		}
	}

	return result
}

// getAllUnsafe returns all events without locking (caller must hold lock)
func (cb *CircularBuffer) getAllUnsafe() []*Event {
	if cb.count == 0 {
		return nil
	}

	result := make([]*Event, 0, cb.count)

	if cb.head < cb.tail {
		result = append(result, cb.events[cb.head:cb.tail]...)
	} else if cb.count == cb.size {
		result = append(result, cb.events[cb.head:]...)
		result = append(result, cb.events[:cb.tail]...)
	} else {
		result = append(result, cb.events[cb.head:cb.tail]...)
	}

	return result
}

// Len returns the number of events currently stored
func (cb *CircularBuffer) Len() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.count
}

// subscription represents an internal subscription
type subscription struct {
	id            string
	subscriber    Subscriber
	topics        map[string]bool
	topicPatterns []*regexp.Regexp
	filter        EventFilter
	strategy      BackpressureStrategy
	bufferSize    int
	eventChan     chan *Event
	ctx           context.Context
	cancel        context.CancelFunc
	lastActive    atomic.Value // stores time.Time
	dropped       atomic.Int64
	delivered     atomic.Int64
}

// SubscriptionStats holds statistics about a subscription
type SubscriptionStats struct {
	ID          string
	Delivered   int64
	Dropped     int64
	LastActive  time.Time
	BufferUsage int
	BufferSize  int
}

// EventBus is the main pub/sub event bus
type EventBus struct {
	mu            sync.RWMutex
	subscriptions map[string]*subscription
	history       *CircularBuffer
	deadTimeout   time.Duration
	wg            sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
	published     atomic.Int64
	eventIDGen    atomic.Int64
}

// EventBusConfig configures the event bus
type EventBusConfig struct {
	HistorySize int
	DeadTimeout time.Duration
}

// DefaultConfig returns default configuration
func DefaultEventBusConfig() EventBusConfig {
	return EventBusConfig{
		HistorySize: 1000,
		DeadTimeout: 30 * time.Second,
	}
}

// NewEventBus creates a new event bus with the given configuration
func NewEventBus(config EventBusConfig) *EventBus {
	if config.HistorySize <= 0 {
		config.HistorySize = 1000
	}
	if config.DeadTimeout <= 0 {
		config.DeadTimeout = 30 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())
	eb := &EventBus{
		subscriptions: make(map[string]*subscription),
		history:       NewCircularBuffer(config.HistorySize),
		deadTimeout:   config.DeadTimeout,
		ctx:           ctx,
		cancel:        cancel,
	}

	// Start dead subscriber detector
	eb.wg.Add(1)
	go eb.detectDeadSubscribers()

	return eb
}

// SubscribeOptions configures a subscription
type SubscribeOptions struct {
	Topics     []string
	Filter     EventFilter
	Strategy   BackpressureStrategy
	BufferSize int
}

// Subscribe adds a subscriber to the event bus
func (eb *EventBus) Subscribe(subscriber Subscriber, opts SubscribeOptions) error {
	if subscriber == nil {
		return fmt.Errorf("subscriber cannot be nil")
	}

	if opts.BufferSize <= 0 {
		opts.BufferSize = 100
	}

	eb.mu.Lock()
	defer eb.mu.Unlock()

	id := subscriber.ID()
	if _, exists := eb.subscriptions[id]; exists {
		return fmt.Errorf("subscriber %s already exists", id)
	}

	ctx, cancel := context.WithCancel(eb.ctx)
	sub := &subscription{
		id:         id,
		subscriber: subscriber,
		topics:     make(map[string]bool),
		filter:     opts.Filter,
		strategy:   opts.Strategy,
		bufferSize: opts.BufferSize,
		eventChan:  make(chan *Event, opts.BufferSize),
		ctx:        ctx,
		cancel:     cancel,
	}
	sub.lastActive.Store(time.Now())

	// Parse topics and wildcard patterns
	for _, topic := range opts.Topics {
		if containsWildcard(topic) {
			pattern := topicToRegex(topic)
			sub.topicPatterns = append(sub.topicPatterns, pattern)
		} else {
			sub.topics[topic] = true
		}
	}

	eb.subscriptions[id] = sub

	// Start event delivery goroutine
	eb.wg.Add(1)
	go eb.deliverEvents(sub)

	return nil
}

// Unsubscribe removes a subscriber from the event bus
func (eb *EventBus) Unsubscribe(subscriberID string) error {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	sub, exists := eb.subscriptions[subscriberID]
	if !exists {
		return fmt.Errorf("subscriber %s not found", subscriberID)
	}
	delete(eb.subscriptions, subscriberID)

	// Cancel the subscription context and close channel
	// Note: These operations are safe to do while holding the lock
	sub.cancel()
	close(sub.eventChan)

	return nil
}

// Publish publishes an event to all matching subscribers
func (eb *EventBus) Publish(event *Event) error {
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}

	// Set timestamp and ID if not already set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if event.ID == "" {
		event.ID = fmt.Sprintf("evt-%d", eb.eventIDGen.Add(1))
	}

	// Add to history
	eb.history.Add(event)
	eb.published.Add(1)

	eb.mu.RLock()
	defer eb.mu.RUnlock()

	// Deliver to matching subscribers
	for _, sub := range eb.subscriptions {
		if eb.shouldDeliver(sub, event) {
			eb.sendToSubscriber(sub, event)
		}
	}

	return nil
}

// PublishSync publishes an event and waits for all subscribers to process it
// Only works with Block strategy subscribers
func (eb *EventBus) PublishSync(event *Event, timeout time.Duration) error {
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}

	// Set timestamp and ID if not already set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if event.ID == "" {
		event.ID = fmt.Sprintf("evt-%d", eb.eventIDGen.Add(1))
	}

	// Add to history
	eb.history.Add(event)
	eb.published.Add(1)

	eb.mu.RLock()
	matchingSubs := make([]*subscription, 0)
	for _, sub := range eb.subscriptions {
		if eb.shouldDeliver(sub, event) {
			matchingSubs = append(matchingSubs, sub)
		}
	}
	eb.mu.RUnlock()

	// Send to all matching subscribers with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for _, sub := range matchingSubs {
		select {
		case sub.eventChan <- event:
			sub.lastActive.Store(time.Now())
			sub.delivered.Add(1)
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for subscribers")
		case <-sub.ctx.Done():
			// Subscriber was unsubscribed
			continue
		}
	}

	return nil
}

// Replay sends historical events to a subscriber
func (eb *EventBus) Replay(subscriberID string, filter EventFilter) error {
	eb.mu.RLock()
	sub, exists := eb.subscriptions[subscriberID]
	eb.mu.RUnlock()

	if !exists {
		return fmt.Errorf("subscriber %s not found", subscriberID)
	}

	events := eb.history.GetAll()
	for _, event := range events {
		if filter == nil || filter(event) {
			if eb.shouldDeliver(sub, event) {
				eb.sendToSubscriber(sub, event)
			}
		}
	}

	return nil
}

// ReplaySince sends historical events since a timestamp to a subscriber
func (eb *EventBus) ReplaySince(subscriberID string, since time.Time, filter EventFilter) error {
	eb.mu.RLock()
	sub, exists := eb.subscriptions[subscriberID]
	eb.mu.RUnlock()

	if !exists {
		return fmt.Errorf("subscriber %s not found", subscriberID)
	}

	events := eb.history.GetSince(since)
	for _, event := range events {
		if filter == nil || filter(event) {
			if eb.shouldDeliver(sub, event) {
				eb.sendToSubscriber(sub, event)
			}
		}
	}

	return nil
}

// GetSubscriptionStats returns statistics for a subscription
func (eb *EventBus) GetSubscriptionStats(subscriberID string) (*SubscriptionStats, error) {
	eb.mu.RLock()
	sub, exists := eb.subscriptions[subscriberID]
	eb.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("subscriber %s not found", subscriberID)
	}

	return &SubscriptionStats{
		ID:          sub.id,
		Delivered:   sub.delivered.Load(),
		Dropped:     sub.dropped.Load(),
		LastActive:  sub.lastActive.Load().(time.Time),
		BufferUsage: len(sub.eventChan),
		BufferSize:  sub.bufferSize,
	}, nil
}

// GetAllStats returns statistics for all subscriptions
func (eb *EventBus) GetAllStats() map[string]*SubscriptionStats {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	stats := make(map[string]*SubscriptionStats)
	for id, sub := range eb.subscriptions {
		stats[id] = &SubscriptionStats{
			ID:          sub.id,
			Delivered:   sub.delivered.Load(),
			Dropped:     sub.dropped.Load(),
			LastActive:  sub.lastActive.Load().(time.Time),
			BufferUsage: len(sub.eventChan),
			BufferSize:  sub.bufferSize,
		}
	}
	return stats
}

// GetPublishedCount returns the total number of events published
func (eb *EventBus) GetPublishedCount() int64 {
	return eb.published.Load()
}

// GetHistorySize returns the number of events in history
func (eb *EventBus) GetHistorySize() int {
	return eb.history.Len()
}

// Close shuts down the event bus gracefully
func (eb *EventBus) Close() {
	eb.cancel()

	eb.mu.Lock()
	// Close all subscriptions
	for _, sub := range eb.subscriptions {
		sub.cancel()
		close(sub.eventChan)
	}
	eb.mu.Unlock()

	// Wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		eb.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Graceful shutdown completed
	case <-time.After(10 * time.Second):
		// Timeout - goroutines may still be running
		// Log warning but continue (best effort)
	}
}

// shouldDeliver checks if an event should be delivered to a subscription
func (eb *EventBus) shouldDeliver(sub *subscription, event *Event) bool {
	// Check topic match
	if !eb.topicMatches(sub, event.Type) {
		return false
	}

	// Check filter
	if sub.filter != nil && !sub.filter(event) {
		return false
	}

	return true
}

// topicMatches checks if an event type matches a subscription's topics
func (eb *EventBus) topicMatches(sub *subscription, eventType string) bool {
	// Check exact match
	if sub.topics[eventType] {
		return true
	}

	// Check wildcard patterns
	for _, pattern := range sub.topicPatterns {
		if pattern.MatchString(eventType) {
			return true
		}
	}

	return false
}

// sendToSubscriber sends an event to a subscriber based on backpressure strategy
func (eb *EventBus) sendToSubscriber(sub *subscription, event *Event) {
	switch sub.strategy {
	case Drop:
		select {
		case sub.eventChan <- event:
			sub.lastActive.Store(time.Now())
			sub.delivered.Add(1)
		default:
			// Drop the event if channel is full
			sub.dropped.Add(1)
		}

	case Block:
		select {
		case sub.eventChan <- event:
			sub.lastActive.Store(time.Now())
			sub.delivered.Add(1)
		case <-sub.ctx.Done():
			return
		}

	case Buffer:
		select {
		case sub.eventChan <- event:
			sub.lastActive.Store(time.Now())
			sub.delivered.Add(1)
		default:
			// If buffer is full, drop the oldest event
			select {
			case <-sub.eventChan:
				sub.dropped.Add(1)
			default:
			}
			// Try to send the new event
			select {
			case sub.eventChan <- event:
				sub.lastActive.Store(time.Now())
				sub.delivered.Add(1)
			default:
				sub.dropped.Add(1)
			}
		}
	}
}

// deliverEvents delivers events to a subscriber
func (eb *EventBus) deliverEvents(sub *subscription) {
	defer eb.wg.Done()

	for {
		select {
		case event, ok := <-sub.eventChan:
			if !ok {
				return
			}

			// Deliver event to subscriber
			// Use a goroutine to prevent blocking if subscriber's OnEvent is slow
			// and we're using Drop or Buffer strategy
			if sub.strategy != Block {
				go func(e *Event) {
					sub.subscriber.OnEvent(e)
					sub.lastActive.Store(time.Now())
				}(event)
			} else {
				sub.subscriber.OnEvent(event)
				sub.lastActive.Store(time.Now())
			}

		case <-sub.ctx.Done():
			return
		}
	}
}

// detectDeadSubscribers periodically checks for and removes dead subscribers
func (eb *EventBus) detectDeadSubscribers() {
	defer eb.wg.Done()

	ticker := time.NewTicker(eb.deadTimeout / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			eb.checkDeadSubscribers()
		case <-eb.ctx.Done():
			return
		}
	}
}

// checkDeadSubscribers removes subscribers that haven't been active
func (eb *EventBus) checkDeadSubscribers() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	now := time.Now()
	for id, sub := range eb.subscriptions {
		lastActive := sub.lastActive.Load().(time.Time)

		if now.Sub(lastActive) > eb.deadTimeout {
			// Remove dead subscriber
			delete(eb.subscriptions, id)
			sub.cancel()
			close(sub.eventChan)
		}
	}
}

// Helper functions

// containsWildcard checks if a topic pattern contains wildcard characters
func containsWildcard(topic string) bool {
	for _, ch := range topic {
		if ch == '*' || ch == '?' {
			return true
		}
	}
	return false
}

// topicToRegex converts a topic pattern with wildcards to a regex
// Supports * (match any characters) and ? (match single character)
func topicToRegex(topic string) *regexp.Regexp {
	var builder strings.Builder
	for _, ch := range topic {
		switch ch {
		case '*':
			builder.WriteString(".*")
		case '?':
			builder.WriteString(".")
		case '.', '+', '(', ')', '[', ']', '{', '}', '^', '$', '|', '\\':
			// Escape special regex characters
			builder.WriteString("\\")
			builder.WriteRune(ch)
		default:
			builder.WriteRune(ch)
		}
	}

	// Anchor the pattern to match the entire string
	pattern := "^" + builder.String() + "$"

	return regexp.MustCompile(pattern)
}

// Common event filters

// TypeFilter creates a filter that matches specific event types
func TypeFilter(types ...string) EventFilter {
	typeMap := make(map[string]bool)
	for _, t := range types {
		typeMap[t] = true
	}
	return func(e *Event) bool {
		return typeMap[e.Type]
	}
}

// SourceFilter creates a filter that matches specific event sources
func SourceFilter(sources ...string) EventFilter {
	sourceMap := make(map[string]bool)
	for _, s := range sources {
		sourceMap[s] = true
	}
	return func(e *Event) bool {
		return sourceMap[e.Source]
	}
}

// TimeRangeFilter creates a filter that matches events within a time range
func TimeRangeFilter(start, end time.Time) EventFilter {
	return func(e *Event) bool {
		return !e.Timestamp.Before(start) && !e.Timestamp.After(end)
	}
}

// AndFilter combines multiple filters with AND logic
func AndFilter(filters ...EventFilter) EventFilter {
	return func(e *Event) bool {
		for _, f := range filters {
			if !f(e) {
				return false
			}
		}
		return true
	}
}

// OrFilter combines multiple filters with OR logic
func OrFilter(filters ...EventFilter) EventFilter {
	return func(e *Event) bool {
		for _, f := range filters {
			if f(e) {
				return true
			}
		}
		return false
	}
}

// NotFilter negates a filter
func NotFilter(filter EventFilter) EventFilter {
	return func(e *Event) bool {
		return !filter(e)
	}
}
