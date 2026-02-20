package brain

import (
	"sync"
	"testing"
	"time"
)

func TestEventBusEmitAndPoll(t *testing.T) {
	eb := NewEventBus(100)
	subID := eb.Subscribe(EventFilter{})

	eb.Emit(Event{Type: EventStatusChanged, Source: "agent-1", RepoPath: "/repo"})

	events, err := eb.Poll(subID, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventStatusChanged {
		t.Fatalf("expected type %s, got %s", EventStatusChanged, events[0].Type)
	}
	if events[0].Source != "agent-1" {
		t.Fatalf("expected source agent-1, got %s", events[0].Source)
	}
	if events[0].Sequence == 0 {
		t.Fatal("expected non-zero sequence")
	}
}

func TestEventBusFilterByType(t *testing.T) {
	eb := NewEventBus(100)
	subID := eb.Subscribe(EventFilter{Types: []EventType{EventMessageReceived}})

	eb.Emit(Event{Type: EventStatusChanged, Source: "a"})
	eb.Emit(Event{Type: EventMessageReceived, Source: "b"})
	eb.Emit(Event{Type: EventAgentRemoved, Source: "c"})

	events, err := eb.Poll(subID, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Source != "b" {
		t.Fatalf("expected source b, got %s", events[0].Source)
	}
}

func TestEventBusFilterByInstance(t *testing.T) {
	eb := NewEventBus(100)
	subID := eb.Subscribe(EventFilter{Instances: []string{"agent-2"}})

	eb.Emit(Event{Type: EventStatusChanged, Source: "agent-1"})
	eb.Emit(Event{Type: EventStatusChanged, Source: "agent-2"})
	eb.Emit(Event{Type: EventStatusChanged, Source: "agent-3"})

	events, err := eb.Poll(subID, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Source != "agent-2" {
		t.Fatalf("expected source agent-2, got %s", events[0].Source)
	}
}

func TestEventBusFilterByParent(t *testing.T) {
	eb := NewEventBus(100)
	subID := eb.Subscribe(EventFilter{ParentTitle: "orchestrator"})

	eb.Emit(Event{Type: EventInstanceStatusChanged, Source: "child-1", Data: map[string]any{"parent_title": "orchestrator"}})
	eb.Emit(Event{Type: EventInstanceStatusChanged, Source: "child-2", Data: map[string]any{"parent_title": "other-parent"}})
	eb.Emit(Event{Type: EventInstanceStatusChanged, Source: "child-3", Data: map[string]any{"parent_title": "orchestrator"}})

	events, err := eb.Poll(subID, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Source != "child-1" || events[1].Source != "child-3" {
		t.Fatalf("unexpected sources: %s, %s", events[0].Source, events[1].Source)
	}
}

func TestEventBusPollTimeout(t *testing.T) {
	eb := NewEventBus(100)
	subID := eb.Subscribe(EventFilter{})

	start := time.Now()
	events, err := eb.Poll(subID, 100*time.Millisecond)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
	if elapsed < 80*time.Millisecond {
		t.Fatalf("poll returned too quickly: %v", elapsed)
	}
}

func TestEventBusPollImmediate(t *testing.T) {
	eb := NewEventBus(100)
	subID := eb.Subscribe(EventFilter{})

	// Emit before polling.
	eb.Emit(Event{Type: EventStatusChanged, Source: "a"})
	eb.Emit(Event{Type: EventStatusChanged, Source: "b"})

	start := time.Now()
	events, err := eb.Poll(subID, 5*time.Second)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if elapsed > 100*time.Millisecond {
		t.Fatalf("poll should have returned immediately, took %v", elapsed)
	}
}

func TestEventBusBufferCap(t *testing.T) {
	eb := NewEventBus(5)
	subID := eb.Subscribe(EventFilter{})

	for i := 0; i < 10; i++ {
		eb.Emit(Event{Type: EventStatusChanged, Source: "a"})
	}

	events, err := eb.Poll(subID, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 5 {
		t.Fatalf("expected 5 events (capped), got %d", len(events))
	}
	// Oldest should have been dropped — sequences should be 6..10.
	if events[0].Sequence != 6 {
		t.Fatalf("expected first event sequence 6, got %d", events[0].Sequence)
	}
}

func TestEventBusUnsubscribe(t *testing.T) {
	eb := NewEventBus(100)
	subID := eb.Subscribe(EventFilter{})
	eb.Unsubscribe(subID)

	_, err := eb.Poll(subID, 10*time.Millisecond)
	if err == nil {
		t.Fatal("expected error after unsubscribe")
	}
}

func TestEventBusPruneStale(t *testing.T) {
	eb := NewEventBus(100)
	subID := eb.Subscribe(EventFilter{})

	// Manually backdate lastPoll.
	eb.mu.RLock()
	sub := eb.subscribers[subID]
	eb.mu.RUnlock()
	sub.mu.Lock()
	sub.lastPoll = time.Now().Add(-10 * time.Minute)
	sub.mu.Unlock()

	removed := eb.PruneStale(5 * time.Minute)
	if removed != 1 {
		t.Fatalf("expected 1 pruned, got %d", removed)
	}

	_, err := eb.Poll(subID, 10*time.Millisecond)
	if err == nil {
		t.Fatal("expected error after prune")
	}
}

func TestEventBusConcurrent(t *testing.T) {
	eb := NewEventBus(1000)
	const numSubscribers = 5
	const numEmitters = 5
	const eventsPerEmitter = 100

	subIDs := make([]string, numSubscribers)
	for i := range subIDs {
		subIDs[i] = eb.Subscribe(EventFilter{})
	}

	// Start emitters.
	var wg sync.WaitGroup
	for i := 0; i < numEmitters; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < eventsPerEmitter; j++ {
				eb.Emit(Event{Type: EventStatusChanged, Source: "emitter"})
			}
		}()
	}

	// Start pollers in parallel.
	totalEvents := make([]int, numSubscribers)
	for i := range subIDs {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for {
				events, err := eb.Poll(subIDs[idx], 200*time.Millisecond)
				if err != nil {
					return
				}
				totalEvents[idx] += len(events)
				if totalEvents[idx] >= numEmitters*eventsPerEmitter {
					return
				}
			}
		}(i)
	}

	wg.Wait()

	expectedTotal := numEmitters * eventsPerEmitter
	for i, count := range totalEvents {
		if count != expectedTotal {
			t.Errorf("subscriber %d received %d events, expected %d", i, count, expectedTotal)
		}
	}
}
