package concurrency

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// Example 1: Basic Event Bus Usage
func ExampleEventStreamBasicUsage() {
	// Create event bus
	bus := NewEventBus(DefaultEventBusConfig())
	defer bus.Close()

	// Create a simple subscriber
	logger := &LoggerSubscriber{name: "logger"}

	// Subscribe to specific topics
	err := bus.Subscribe(logger, SubscribeOptions{
		Topics:     []string{"user.login", "user.logout"},
		Strategy:   Drop,
		BufferSize: 100,
	})
	if err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}

	// Publish events
	bus.Publish(&Event{
		Type:    "user.login",
		Payload: map[string]interface{}{"user_id": "123", "ip": "192.168.1.1"},
		Source:  "auth-service",
	})

	bus.Publish(&Event{
		Type:    "user.logout",
		Payload: map[string]interface{}{"user_id": "123"},
		Source:  "auth-service",
	})

	time.Sleep(100 * time.Millisecond)
	fmt.Println("Basic usage example completed")
}

// Example 2: Wildcard Topics and Filtering
func ExampleWildcardAndFiltering() {
	bus := NewEventBus(DefaultEventBusConfig())
	defer bus.Close()

	// Subscribe with wildcard patterns and filter
	analytics := &AnalyticsSubscriber{name: "analytics"}
	bus.Subscribe(analytics, SubscribeOptions{
		Topics: []string{"order.*", "payment.*"},
		Filter: func(e *Event) bool {
			// Only track production events
			return e.Source == "production"
		},
		Strategy:   Buffer,
		BufferSize: 500,
	})

	// Publish various events
	events := []struct {
		eventType string
		source    string
	}{
		{"order.created", "production"},
		{"order.updated", "staging"},      // filtered out
		{"order.completed", "production"},
		{"payment.initiated", "production"},
		{"payment.failed", "production"},
		{"inventory.updated", "production"}, // not matching topic
	}

	for _, evt := range events {
		bus.Publish(&Event{
			Type:    evt.eventType,
			Payload: map[string]interface{}{"data": "sample"},
			Source:  evt.source,
		})
	}

	time.Sleep(100 * time.Millisecond)
	fmt.Println("Wildcard and filtering example completed")
}

// Example 3: Event Replay
func ExampleEventReplay() {
	bus := NewEventBus(EventBusConfig{
		HistorySize: 100,
		DeadTimeout: 30 * time.Second,
	})
	defer bus.Close()

	// Publish some events before any subscriber exists
	for i := 0; i < 5; i++ {
		bus.Publish(&Event{
			Type:    "system.startup",
			Payload: fmt.Sprintf("Init step %d", i),
			Source:  "system",
		})
	}

	// Later, a new subscriber joins and wants historical events
	monitor := &MonitorSubscriber{name: "monitor"}
	bus.Subscribe(monitor, SubscribeOptions{
		Topics:     []string{"system.*"},
		Strategy:   Drop,
		BufferSize: 100,
	})

	// Replay historical events
	bus.Replay("monitor", nil)

	// Continue receiving new events
	bus.Publish(&Event{
		Type:    "system.ready",
		Payload: "System is now operational",
		Source:  "system",
	})

	time.Sleep(100 * time.Millisecond)
	fmt.Println("Event replay example completed")
}

// Example 4: Backpressure Handling
func ExampleBackpressureHandling() {
	bus := NewEventBus(DefaultEventBusConfig())
	defer bus.Close()

	// Fast subscriber with Drop strategy
	fastSub := &FastSubscriber{name: "fast"}
	bus.Subscribe(fastSub, SubscribeOptions{
		Topics:     []string{"metrics"},
		Strategy:   Drop, // Drop events if subscriber is slow
		BufferSize: 10,
	})

	// Slow subscriber with Buffer strategy
	slowSub := &SlowSubscriber{name: "slow", delay: 50 * time.Millisecond}
	bus.Subscribe(slowSub, SubscribeOptions{
		Topics:     []string{"metrics"},
		Strategy:   Buffer, // Keep buffer, drop oldest when full
		BufferSize: 20,
	})

	// Critical subscriber with Block strategy
	criticalSub := &CriticalSubscriber{name: "critical"}
	bus.Subscribe(criticalSub, SubscribeOptions{
		Topics:     []string{"metrics"},
		Strategy:   Block, // Block publisher if subscriber can't keep up
		BufferSize: 50,
	})

	// Publish many events rapidly
	for i := 0; i < 100; i++ {
		bus.Publish(&Event{
			Type:    "metrics",
			Payload: map[string]interface{}{"value": i},
			Source:  "sensor",
		})
	}

	time.Sleep(2 * time.Second)

	// Check stats
	stats, _ := bus.GetSubscriptionStats("fast")
	fmt.Printf("Fast subscriber - Delivered: %d, Dropped: %d\n", stats.Delivered, stats.Dropped)

	stats, _ = bus.GetSubscriptionStats("slow")
	fmt.Printf("Slow subscriber - Delivered: %d, Dropped: %d\n", stats.Delivered, stats.Dropped)

	stats, _ = bus.GetSubscriptionStats("critical")
	fmt.Printf("Critical subscriber - Delivered: %d, Dropped: %d\n", stats.Delivered, stats.Dropped)
}

// Example 5: Multi-tenant Event Bus
func ExampleMultiTenant() {
	bus := NewEventBus(DefaultEventBusConfig())
	defer bus.Close()

	tenants := []string{"tenant-A", "tenant-B", "tenant-C"}

	// Create subscribers for each tenant
	for _, tenant := range tenants {
		sub := &TenantSubscriber{tenantID: tenant}
		bus.Subscribe(sub, SubscribeOptions{
			Topics: []string{fmt.Sprintf("%s.*", tenant)},
			Filter: func(e *Event) bool {
				// Additional filtering based on payload
				if payload, ok := e.Payload.(map[string]interface{}); ok {
					return payload["tenant_id"] == tenant
				}
				return false
			},
			Strategy:   Drop,
			BufferSize: 100,
		})
	}

	// Publish events for different tenants
	for i := 0; i < 10; i++ {
		tenant := tenants[i%len(tenants)]
		bus.Publish(&Event{
			Type: fmt.Sprintf("%s.event", tenant),
			Payload: map[string]interface{}{
				"tenant_id": tenant,
				"data":      fmt.Sprintf("Event %d", i),
			},
			Source: "api",
		})
	}

	time.Sleep(100 * time.Millisecond)
	fmt.Println("Multi-tenant example completed")
}

// Example 6: Event Transformation and Enrichment
func ExampleEventTransformation() {
	bus := NewEventBus(DefaultEventBusConfig())
	defer bus.Close()

	// Subscriber that enriches events
	enricher := &EnrichmentSubscriber{
		name:        "enricher",
		userCache:   map[string]string{"123": "John Doe", "456": "Jane Smith"},
		transformFn: func(e *Event) *Event {
			if payload, ok := e.Payload.(map[string]interface{}); ok {
				if userID, ok := payload["user_id"].(string); ok {
					// Enrich with user name (this is just an example)
					// In real scenarios, you might publish a new enriched event
					fmt.Printf("Enriched event for user %s\n", userID)
				}
			}
			return e
		},
	}

	bus.Subscribe(enricher, SubscribeOptions{
		Topics:     []string{"user.*"},
		Strategy:   Drop,
		BufferSize: 100,
	})

	bus.Publish(&Event{
		Type:    "user.action",
		Payload: map[string]interface{}{"user_id": "123", "action": "login"},
		Source:  "app",
	})

	time.Sleep(100 * time.Millisecond)
	fmt.Println("Event transformation example completed")
}

// Example 7: Monitoring and Metrics
func ExampleMonitoring() {
	bus := NewEventBus(DefaultEventBusConfig())
	defer bus.Close()

	// Create monitoring subscriber
	monitor := &MetricsSubscriber{
		name:    "metrics",
		counter: make(map[string]int),
	}

	bus.Subscribe(monitor, SubscribeOptions{
		Topics:     []string{"*"}, // Subscribe to all events
		Strategy:   Drop,
		BufferSize: 1000,
	})

	// Simulate various events
	eventTypes := []string{"api.request", "api.response", "api.error", "db.query", "cache.hit", "cache.miss"}
	for i := 0; i < 100; i++ {
		bus.Publish(&Event{
			Type:    eventTypes[i%len(eventTypes)],
			Payload: map[string]interface{}{"count": i},
			Source:  "system",
		})
	}

	time.Sleep(200 * time.Millisecond)

	// Print metrics
	monitor.PrintMetrics()

	// Get overall bus statistics
	allStats := bus.GetAllStats()
	fmt.Printf("\nTotal subscribers: %d\n", len(allStats))
	fmt.Printf("Total events published: %d\n", bus.GetPublishedCount())
	fmt.Printf("History size: %d\n", bus.GetHistorySize())
}

// Example Subscriber Implementations

type LoggerSubscriber struct {
	name string
	mu   sync.Mutex
}

func (l *LoggerSubscriber) ID() string {
	return l.name
}

func (l *LoggerSubscriber) OnEvent(event *Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Printf("[%s] %s: %s - %v\n", l.name, event.Timestamp.Format("15:04:05"), event.Type, event.Payload)
}

type AnalyticsSubscriber struct {
	name   string
	events []*Event
	mu     sync.Mutex
}

func (a *AnalyticsSubscriber) ID() string {
	return a.name
}

func (a *AnalyticsSubscriber) OnEvent(event *Event) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = append(a.events, event)
	fmt.Printf("[%s] Tracked: %s from %s\n", a.name, event.Type, event.Source)
}

type MonitorSubscriber struct {
	name   string
	events []*Event
	mu     sync.Mutex
}

func (m *MonitorSubscriber) ID() string {
	return m.name
}

func (m *MonitorSubscriber) OnEvent(event *Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	fmt.Printf("[%s] Monitored: %s - %v\n", m.name, event.Type, event.Payload)
}

type FastSubscriber struct {
	name  string
	count int
	mu    sync.Mutex
}

func (f *FastSubscriber) ID() string {
	return f.name
}

func (f *FastSubscriber) OnEvent(event *Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.count++
	// Process quickly
}

type SlowSubscriber struct {
	name  string
	delay time.Duration
	count int
	mu    sync.Mutex
}

func (s *SlowSubscriber) ID() string {
	return s.name
}

func (s *SlowSubscriber) OnEvent(event *Event) {
	time.Sleep(s.delay) // Simulate slow processing
	s.mu.Lock()
	defer s.mu.Unlock()
	s.count++
}

type CriticalSubscriber struct {
	name  string
	count int
	mu    sync.Mutex
}

func (c *CriticalSubscriber) ID() string {
	return c.name
}

func (c *CriticalSubscriber) OnEvent(event *Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count++
	// Critical processing - must not drop events
}

type TenantSubscriber struct {
	tenantID string
	events   []*Event
	mu       sync.Mutex
}

func (t *TenantSubscriber) ID() string {
	return t.tenantID
}

func (t *TenantSubscriber) OnEvent(event *Event) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, event)
	fmt.Printf("[%s] Received: %s\n", t.tenantID, event.Type)
}

type EnrichmentSubscriber struct {
	name        string
	userCache   map[string]string
	transformFn func(*Event) *Event
}

func (e *EnrichmentSubscriber) ID() string {
	return e.name
}

func (e *EnrichmentSubscriber) OnEvent(event *Event) {
	enriched := e.transformFn(event)
	_ = enriched // Use enriched event
}

type MetricsSubscriber struct {
	name    string
	counter map[string]int
	mu      sync.Mutex
}

func (m *MetricsSubscriber) ID() string {
	return m.name
}

func (m *MetricsSubscriber) OnEvent(event *Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counter[event.Type]++
}

func (m *MetricsSubscriber) PrintMetrics() {
	m.mu.Lock()
	defer m.mu.Unlock()

	fmt.Println("\n=== Event Metrics ===")
	for eventType, count := range m.counter {
		fmt.Printf("%s: %d\n", eventType, count)
	}
	fmt.Println("====================")
}

// Example 8: Complex Event Processing Pipeline
func ExampleComplexPipeline() {
	bus := NewEventBus(EventBusConfig{
		HistorySize: 1000,
		DeadTimeout: 60 * time.Second,
	})
	defer bus.Close()

	// Stage 1: Raw event collector
	collector := &LoggerSubscriber{name: "collector"}
	bus.Subscribe(collector, SubscribeOptions{
		Topics:     []string{"raw.*"},
		Strategy:   Buffer,
		BufferSize: 500,
	})

	// Stage 2: Event validator with filter
	validator := &LoggerSubscriber{name: "validator"}
	bus.Subscribe(validator, SubscribeOptions{
		Topics: []string{"raw.*"},
		Filter: func(e *Event) bool {
			// Validate event structure
			payload, ok := e.Payload.(map[string]interface{})
			if !ok {
				return false
			}
			_, hasData := payload["data"]
			return hasData
		},
		Strategy:   Drop,
		BufferSize: 200,
	})

	// Stage 3: Event aggregator
	aggregator := &MetricsSubscriber{name: "aggregator", counter: make(map[string]int)}
	bus.Subscribe(aggregator, SubscribeOptions{
		Topics:     []string{"raw.*"},
		Strategy:   Buffer,
		BufferSize: 300,
	})

	// Publish events through the pipeline
	for i := 0; i < 50; i++ {
		var payload interface{}
		if i%3 == 0 {
			// Valid event
			payload = map[string]interface{}{"data": fmt.Sprintf("value-%d", i)}
		} else {
			// Invalid event (no data field)
			payload = map[string]interface{}{"other": "field"}
		}

		bus.Publish(&Event{
			Type:    fmt.Sprintf("raw.sensor.%d", i%5),
			Payload: payload,
			Source:  "iot-device",
		})
	}

	time.Sleep(200 * time.Millisecond)
	aggregator.PrintMetrics()
	fmt.Println("Complex pipeline example completed")
}

// Example 9: Time-based Event Replay
func ExampleTimeBasedReplay() {
	bus := NewEventBus(EventBusConfig{
		HistorySize: 100,
		DeadTimeout: 30 * time.Second,
	})
	defer bus.Close()

	// Publish events over time
	startTime := time.Now()

	for i := 0; i < 10; i++ {
		bus.Publish(&Event{
			Type:    "time.event",
			Payload: fmt.Sprintf("Event %d", i),
			Source:  "timer",
		})
		time.Sleep(10 * time.Millisecond)
	}

	cutoffTime := time.Now()
	time.Sleep(50 * time.Millisecond)

	for i := 10; i < 20; i++ {
		bus.Publish(&Event{
			Type:    "time.event",
			Payload: fmt.Sprintf("Event %d", i),
			Source:  "timer",
		})
		time.Sleep(10 * time.Millisecond)
	}

	// Subscribe and replay only recent events
	recent := &LoggerSubscriber{name: "recent-only"}
	bus.Subscribe(recent, SubscribeOptions{
		Topics:     []string{"time.*"},
		Strategy:   Drop,
		BufferSize: 100,
	})

	// Replay events since cutoff
	bus.ReplaySince("recent-only", cutoffTime, nil)

	time.Sleep(100 * time.Millisecond)
	fmt.Printf("Time-based replay completed (started at %s, cutoff at %s)\n",
		startTime.Format("15:04:05.000"),
		cutoffTime.Format("15:04:05.000"))
}

// Example 10: Event Fan-out Pattern
func ExampleFanOut() {
	bus := NewEventBus(DefaultEventBusConfig())
	defer bus.Close()

	// Create multiple specialized subscribers
	subscribers := []struct {
		name   string
		topics []string
	}{
		{"email-notifier", []string{"user.registered", "order.completed"}},
		{"sms-notifier", []string{"order.shipped", "payment.failed"}},
		{"slack-notifier", []string{"error.*", "alert.*"}},
		{"database-writer", []string{"*"}}, // Catch all
		{"audit-logger", []string{"user.*", "admin.*"}},
	}

	for _, s := range subscribers {
		sub := &LoggerSubscriber{name: s.name}
		bus.Subscribe(sub, SubscribeOptions{
			Topics:     s.topics,
			Strategy:   Buffer,
			BufferSize: 100,
		})
	}

	// Publish various events - they'll be fanned out to appropriate subscribers
	events := []Event{
		{Type: "user.registered", Payload: "user@example.com", Source: "auth"},
		{Type: "order.completed", Payload: "order-123", Source: "checkout"},
		{Type: "order.shipped", Payload: "order-123", Source: "fulfillment"},
		{Type: "error.database", Payload: "connection timeout", Source: "db"},
		{Type: "payment.failed", Payload: "insufficient funds", Source: "payment"},
		{Type: "admin.login", Payload: "admin@example.com", Source: "admin-panel"},
	}

	for _, evt := range events {
		bus.Publish(&evt)
	}

	time.Sleep(200 * time.Millisecond)

	// Check statistics
	fmt.Println("\n=== Fan-out Statistics ===")
	allStats := bus.GetAllStats()
	for name, stats := range allStats {
		fmt.Printf("%s: Delivered=%d, Dropped=%d, Buffer=%d/%d\n",
			name, stats.Delivered, stats.Dropped, stats.BufferUsage, stats.BufferSize)
	}
}
