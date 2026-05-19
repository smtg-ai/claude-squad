package server

import (
	"sync"
	"sync/atomic"
	"time"
)

// eventBus is a small broadcast ring for Server-Sent-Events subscribers.
// Producers call `publish` with any event payload; subscribers receive
// a copy via a channel. Historical events up to a caller-supplied `since`
// seq are replayed on subscribe.
type eventBus struct {
	mu      sync.RWMutex
	seq     atomic.Int64
	history []Event
	maxHist int
	subs    map[chan Event]struct{}
}

func newEventBus() *eventBus {
	return &eventBus{
		maxHist: 512,
		subs:    make(map[chan Event]struct{}),
	}
}

func (b *eventBus) publish(evtType, instanceID string, data map[string]string) Event {
	e := Event{
		Seq:        b.seq.Add(1),
		Type:       evtType,
		InstanceID: instanceID,
		Timestamp:  time.Now().UTC(),
		Data:       data,
	}
	b.mu.Lock()
	b.history = append(b.history, e)
	if len(b.history) > b.maxHist {
		b.history = b.history[len(b.history)-b.maxHist:]
	}
	// Fan out to subscribers with non-blocking send; a slow consumer
	// drops events rather than blocking the producer.
	for ch := range b.subs {
		select {
		case ch <- e:
		default:
		}
	}
	b.mu.Unlock()
	return e
}

// subscribe returns a channel of new events plus a replay of any events
// with seq > since. Cancel by calling the returned unsubscribe fn.
func (b *eventBus) subscribe(since int64, buf int) (<-chan Event, func()) {
	ch := make(chan Event, buf)
	b.mu.Lock()
	for _, e := range b.history {
		if e.Seq > since {
			// Non-blocking on history replay; if the caller's buf is
			// smaller than history size, older events spill.
			select {
			case ch <- e:
			default:
			}
		}
	}
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		delete(b.subs, ch)
		b.mu.Unlock()
		close(ch)
	}
}
