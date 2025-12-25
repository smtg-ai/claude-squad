package concurrency

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// MockSubscriber is a test subscriber
type MockSubscriber struct {
	id       string
	events   []*Event
	mu       sync.Mutex
	onEvent  func(*Event)
	received atomic.Int64
}

func NewMockSubscriber(id string) *MockSubscriber {
	return &MockSubscriber{
		id:     id,
		events: make([]*Event, 0),
	}
}

func (m *MockSubscriber) OnEvent(event *Event) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.events = append(m.events, event)
	m.received.Add(1)

	if m.onEvent != nil {
		m.onEvent(event)
	}
}

func (m *MockSubscriber) ID() string {
	return m.id
}

func (m *MockSubscriber) GetEvents() []*Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]*Event(nil), m.events...)
}

func (m *MockSubscriber) GetEventCount() int {
	return int(m.received.Load())
}

func TestEventBusBasicPublishSubscribe(t *testing.T) {
	bus := NewEventBus(DefaultEventBusConfig())
	defer bus.Close()

	sub := NewMockSubscriber("sub1")
	err := bus.Subscribe(sub, SubscribeOptions{
		Topics:     []string{"test.event"},
		Strategy:   Drop,
		BufferSize: 10,
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Publish an event
	event := &Event{
		Type:    "test.event",
		Payload: "test payload",
		Source:  "test",
	}

	err = bus.Publish(event)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Wait for delivery
	time.Sleep(100 * time.Millisecond)

	events := sub.GetEvents()
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	if events[0].Type != "test.event" {
		t.Errorf("Expected event type 'test.event', got '%s'", events[0].Type)
	}
}

func TestEventBusMultipleSubscribers(t *testing.T) {
	bus := NewEventBus(DefaultEventBusConfig())
	defer bus.Close()

	sub1 := NewMockSubscriber("sub1")
	sub2 := NewMockSubscriber("sub2")
	sub3 := NewMockSubscriber("sub3")

	// Subscribe all to the same topic
	for _, sub := range []Subscriber{sub1, sub2, sub3} {
		err := bus.Subscribe(sub, SubscribeOptions{
			Topics:     []string{"broadcast"},
			Strategy:   Drop,
			BufferSize: 10,
		})
		if err != nil {
			t.Fatalf("Subscribe failed: %v", err)
		}
	}

	// Publish event
	bus.Publish(&Event{
		Type:    "broadcast",
		Payload: "hello all",
		Source:  "test",
	})

	// Wait for delivery
	time.Sleep(100 * time.Millisecond)

	// All should receive the event
	for _, sub := range []*MockSubscriber{sub1, sub2, sub3} {
		if sub.GetEventCount() != 1 {
			t.Errorf("Subscriber %s: expected 1 event, got %d", sub.ID(), sub.GetEventCount())
		}
	}
}

func TestEventBusWildcardTopics(t *testing.T) {
	bus := NewEventBus(DefaultEventBusConfig())
	defer bus.Close()

	sub := NewMockSubscriber("sub1")
	err := bus.Subscribe(sub, SubscribeOptions{
		Topics:     []string{"user.*", "order.*.created"},
		Strategy:   Drop,
		BufferSize: 10,
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Publish matching events
	bus.Publish(&Event{Type: "user.login", Payload: "data1", Source: "test"})
	bus.Publish(&Event{Type: "user.logout", Payload: "data2", Source: "test"})
	bus.Publish(&Event{Type: "order.123.created", Payload: "data3", Source: "test"})
	bus.Publish(&Event{Type: "order.456.created", Payload: "data4", Source: "test"})

	// Publish non-matching events
	bus.Publish(&Event{Type: "product.view", Payload: "data5", Source: "test"})
	bus.Publish(&Event{Type: "order.123.updated", Payload: "data6", Source: "test"})

	time.Sleep(100 * time.Millisecond)

	count := sub.GetEventCount()
	if count != 4 {
		t.Errorf("Expected 4 events, got %d", count)
	}
}

func TestEventBusFiltering(t *testing.T) {
	bus := NewEventBus(DefaultEventBusConfig())
	defer bus.Close()

	sub := NewMockSubscriber("sub1")
	err := bus.Subscribe(sub, SubscribeOptions{
		Topics: []string{"*"},
		Filter: func(e *Event) bool {
			// Only accept events from "important" source
			return e.Source == "important"
		},
		Strategy:   Drop,
		BufferSize: 10,
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Publish events from different sources
	bus.Publish(&Event{Type: "event1", Source: "important", Payload: "data1"})
	bus.Publish(&Event{Type: "event2", Source: "normal", Payload: "data2"})
	bus.Publish(&Event{Type: "event3", Source: "important", Payload: "data3"})
	bus.Publish(&Event{Type: "event4", Source: "low", Payload: "data4"})

	time.Sleep(100 * time.Millisecond)

	count := sub.GetEventCount()
	if count != 2 {
		t.Errorf("Expected 2 events (filtered), got %d", count)
	}
}

func TestEventBusBackpressureDrop(t *testing.T) {
	bus := NewEventBus(DefaultEventBusConfig())
	defer bus.Close()

	sub := NewMockSubscriber("sub1")
	// Slow subscriber - sleep on each event
	sub.onEvent = func(e *Event) {
		time.Sleep(50 * time.Millisecond)
	}

	err := bus.Subscribe(sub, SubscribeOptions{
		Topics:     []string{"test"},
		Strategy:   Drop,
		BufferSize: 2, // Small buffer
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Publish many events quickly
	for i := 0; i < 10; i++ {
		bus.Publish(&Event{
			Type:    "test",
			Payload: fmt.Sprintf("event-%d", i),
			Source:  "test",
		})
	}

	// Wait for some processing
	time.Sleep(300 * time.Millisecond)

	stats, err := bus.GetSubscriptionStats("sub1")
	if err != nil {
		t.Fatalf("GetSubscriptionStats failed: %v", err)
	}

	// Should have dropped some events
	if stats.Dropped == 0 {
		t.Errorf("Expected some dropped events with Drop strategy, got %d", stats.Dropped)
	}

	t.Logf("Delivered: %d, Dropped: %d", stats.Delivered, stats.Dropped)
}

func TestEventBusBackpressureBuffer(t *testing.T) {
	bus := NewEventBus(DefaultEventBusConfig())
	defer bus.Close()

	sub := NewMockSubscriber("sub1")
	// Slow subscriber
	sub.onEvent = func(e *Event) {
		time.Sleep(20 * time.Millisecond)
	}

	err := bus.Subscribe(sub, SubscribeOptions{
		Topics:     []string{"test"},
		Strategy:   Buffer,
		BufferSize: 5,
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Publish events
	for i := 0; i < 10; i++ {
		bus.Publish(&Event{
			Type:    "test",
			Payload: fmt.Sprintf("event-%d", i),
			Source:  "test",
		})
	}

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	stats, err := bus.GetSubscriptionStats("sub1")
	if err != nil {
		t.Fatalf("GetSubscriptionStats failed: %v", err)
	}

	t.Logf("Delivered: %d, Dropped: %d, BufferSize: %d", stats.Delivered, stats.Dropped, stats.BufferSize)
}

func TestEventBusReplay(t *testing.T) {
	bus := NewEventBus(EventBusConfig{
		HistorySize: 10,
		DeadTimeout: 30 * time.Second,
	})
	defer bus.Close()

	// Publish some events before subscription
	for i := 0; i < 5; i++ {
		bus.Publish(&Event{
			Type:    "history",
			Payload: fmt.Sprintf("event-%d", i),
			Source:  "test",
		})
	}

	// Now subscribe
	sub := NewMockSubscriber("sub1")
	err := bus.Subscribe(sub, SubscribeOptions{
		Topics:     []string{"history"},
		Strategy:   Drop,
		BufferSize: 20,
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Replay history
	err = bus.Replay("sub1", nil)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	// Publish new event
	bus.Publish(&Event{
		Type:    "history",
		Payload: "new-event",
		Source:  "test",
	})

	time.Sleep(100 * time.Millisecond)

	count := sub.GetEventCount()
	if count != 6 { // 5 replayed + 1 new
		t.Errorf("Expected 6 events (5 replayed + 1 new), got %d", count)
	}
}

func TestEventBusReplaySince(t *testing.T) {
	bus := NewEventBus(DefaultEventBusConfig())
	defer bus.Close()

	startTime := time.Now()

	// Publish old events
	for i := 0; i < 3; i++ {
		bus.Publish(&Event{
			Type:      "test",
			Payload:   fmt.Sprintf("old-%d", i),
			Timestamp: startTime.Add(-time.Hour),
			Source:    "test",
		})
	}

	cutoff := time.Now()
	time.Sleep(10 * time.Millisecond)

	// Publish recent events
	for i := 0; i < 3; i++ {
		bus.Publish(&Event{
			Type:    "test",
			Payload: fmt.Sprintf("new-%d", i),
			Source:  "test",
		})
	}

	// Subscribe
	sub := NewMockSubscriber("sub1")
	bus.Subscribe(sub, SubscribeOptions{
		Topics:     []string{"test"},
		Strategy:   Drop,
		BufferSize: 20,
	})

	// Replay only recent events
	err := bus.ReplaySince("sub1", cutoff, nil)
	if err != nil {
		t.Fatalf("ReplaySince failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	count := sub.GetEventCount()
	if count != 3 {
		t.Errorf("Expected 3 recent events, got %d", count)
	}
}

func TestEventBusUnsubscribe(t *testing.T) {
	bus := NewEventBus(DefaultEventBusConfig())
	defer bus.Close()

	sub := NewMockSubscriber("sub1")
	bus.Subscribe(sub, SubscribeOptions{
		Topics:     []string{"test"},
		Strategy:   Drop,
		BufferSize: 10,
	})

	// Publish event
	bus.Publish(&Event{Type: "test", Payload: "data1", Source: "test"})
	time.Sleep(50 * time.Millisecond)

	// Unsubscribe
	err := bus.Unsubscribe("sub1")
	if err != nil {
		t.Fatalf("Unsubscribe failed: %v", err)
	}

	// Publish another event
	bus.Publish(&Event{Type: "test", Payload: "data2", Source: "test"})
	time.Sleep(50 * time.Millisecond)

	// Should only have received first event
	count := sub.GetEventCount()
	if count != 1 {
		t.Errorf("Expected 1 event (before unsubscribe), got %d", count)
	}
}

func TestEventBusDeadSubscriberDetection(t *testing.T) {
	bus := NewEventBus(EventBusConfig{
		HistorySize: 100,
		DeadTimeout: 200 * time.Millisecond,
	})
	defer bus.Close()

	sub := NewMockSubscriber("sub1")
	bus.Subscribe(sub, SubscribeOptions{
		Topics:     []string{"test"},
		Strategy:   Drop,
		BufferSize: 10,
	})

	// Publish event to keep subscriber active
	bus.Publish(&Event{Type: "test", Payload: "data1", Source: "test"})
	time.Sleep(50 * time.Millisecond)

	// Wait longer than dead timeout without publishing
	time.Sleep(300 * time.Millisecond)

	// Subscriber should be removed
	_, err := bus.GetSubscriptionStats("sub1")
	if err == nil {
		t.Error("Expected subscriber to be removed as dead, but it still exists")
	}
}

func TestCircularBuffer(t *testing.T) {
	cb := NewCircularBuffer(3)

	// Add events
	for i := 0; i < 5; i++ {
		cb.Add(&Event{
			Type:    fmt.Sprintf("event-%d", i),
			Payload: i,
		})
	}

	events := cb.GetAll()
	if len(events) != 3 {
		t.Errorf("Expected 3 events in buffer, got %d", len(events))
	}

	// Should have the 3 most recent events (2, 3, 4)
	if events[0].Payload != 2 {
		t.Errorf("Expected first event payload 2, got %v", events[0].Payload)
	}
	if events[2].Payload != 4 {
		t.Errorf("Expected last event payload 4, got %v", events[2].Payload)
	}

	// Test Len
	if cb.Len() != 3 {
		t.Errorf("Expected length 3, got %d", cb.Len())
	}
}

func TestEventFilters(t *testing.T) {
	// Test TypeFilter
	filter := TypeFilter("type1", "type2")
	if !filter(&Event{Type: "type1"}) {
		t.Error("TypeFilter should match type1")
	}
	if filter(&Event{Type: "type3"}) {
		t.Error("TypeFilter should not match type3")
	}

	// Test SourceFilter
	filter = SourceFilter("source1")
	if !filter(&Event{Source: "source1"}) {
		t.Error("SourceFilter should match source1")
	}
	if filter(&Event{Source: "source2"}) {
		t.Error("SourceFilter should not match source2")
	}

	// Test TimeRangeFilter
	now := time.Now()
	filter = TimeRangeFilter(now.Add(-time.Hour), now.Add(time.Hour))
	if !filter(&Event{Timestamp: now}) {
		t.Error("TimeRangeFilter should match current time")
	}
	if filter(&Event{Timestamp: now.Add(-2 * time.Hour)}) {
		t.Error("TimeRangeFilter should not match time outside range")
	}

	// Test AndFilter
	filter = AndFilter(
		TypeFilter("type1"),
		SourceFilter("source1"),
	)
	if !filter(&Event{Type: "type1", Source: "source1"}) {
		t.Error("AndFilter should match when all conditions are met")
	}
	if filter(&Event{Type: "type1", Source: "source2"}) {
		t.Error("AndFilter should not match when any condition fails")
	}

	// Test OrFilter
	filter = OrFilter(
		TypeFilter("type1"),
		TypeFilter("type2"),
	)
	if !filter(&Event{Type: "type1"}) {
		t.Error("OrFilter should match type1")
	}
	if !filter(&Event{Type: "type2"}) {
		t.Error("OrFilter should match type2")
	}
	if filter(&Event{Type: "type3"}) {
		t.Error("OrFilter should not match type3")
	}

	// Test NotFilter
	filter = NotFilter(TypeFilter("type1"))
	if filter(&Event{Type: "type1"}) {
		t.Error("NotFilter should not match type1")
	}
	if !filter(&Event{Type: "type2"}) {
		t.Error("NotFilter should match type2")
	}
}

func TestEventBusConcurrency(t *testing.T) {
	bus := NewEventBus(DefaultEventBusConfig())
	defer bus.Close()

	numSubscribers := 10
	numEvents := 100

	var wg sync.WaitGroup
	subscribers := make([]*MockSubscriber, numSubscribers)

	// Create and subscribe multiple subscribers
	for i := 0; i < numSubscribers; i++ {
		sub := NewMockSubscriber(fmt.Sprintf("sub-%d", i))
		subscribers[i] = sub

		bus.Subscribe(sub, SubscribeOptions{
			Topics:     []string{"concurrent.*"},
			Strategy:   Buffer,
			BufferSize: 200,
		})
	}

	// Publish events concurrently
	wg.Add(numEvents)
	for i := 0; i < numEvents; i++ {
		go func(n int) {
			defer wg.Done()
			bus.Publish(&Event{
				Type:    "concurrent.test",
				Payload: n,
				Source:  "test",
			})
		}(i)
	}

	wg.Wait()
	time.Sleep(500 * time.Millisecond)

	// Check all subscribers received all events
	for _, sub := range subscribers {
		count := sub.GetEventCount()
		if count != numEvents {
			t.Errorf("Subscriber %s: expected %d events, got %d", sub.ID(), numEvents, count)
		}
	}
}

func TestEventBusPublishSync(t *testing.T) {
	bus := NewEventBus(DefaultEventBusConfig())
	defer bus.Close()

	sub := NewMockSubscriber("sub1")
	bus.Subscribe(sub, SubscribeOptions{
		Topics:     []string{"sync"},
		Strategy:   Block,
		BufferSize: 10,
	})

	// PublishSync with timeout
	err := bus.PublishSync(&Event{
		Type:    "sync",
		Payload: "test",
		Source:  "test",
	}, 1*time.Second)

	if err != nil {
		t.Fatalf("PublishSync failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if sub.GetEventCount() != 1 {
		t.Errorf("Expected 1 event, got %d", sub.GetEventCount())
	}
}

func BenchmarkEventBusPublish(b *testing.B) {
	bus := NewEventBus(DefaultEventBusConfig())
	defer bus.Close()

	sub := NewMockSubscriber("sub1")
	bus.Subscribe(sub, SubscribeOptions{
		Topics:     []string{"benchmark"},
		Strategy:   Drop,
		BufferSize: 1000,
	})

	event := &Event{
		Type:    "benchmark",
		Payload: "test",
		Source:  "bench",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.Publish(event)
	}
}

func BenchmarkEventBusMultipleSubscribers(b *testing.B) {
	bus := NewEventBus(DefaultEventBusConfig())
	defer bus.Close()

	// Create 10 subscribers
	for i := 0; i < 10; i++ {
		sub := NewMockSubscriber(fmt.Sprintf("sub-%d", i))
		bus.Subscribe(sub, SubscribeOptions{
			Topics:     []string{"benchmark"},
			Strategy:   Drop,
			BufferSize: 1000,
		})
	}

	event := &Event{
		Type:    "benchmark",
		Payload: "test",
		Source:  "bench",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.Publish(event)
	}
}
