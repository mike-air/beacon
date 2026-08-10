// Package realtime is an in-memory pub/sub hub that fans out events to SSE
// subscribers, keyed by org_id. The HTTP layer opens a Server-Sent-Events
// stream per browser tab and subscribes it here; when something changes the
// handler publishes an event and every subscriber for that org receives it.
//
// Course mapping: Chapter 25 — real-time updates with SSE. The chapter builds a
// per-org fan-out hub guarded by a mutex, with buffered per-subscriber channels
// and a non-blocking send so one slow client can't stall the publisher. We match
// that shape. The transport (SSE framing, heartbeats, context cancellation)
// lives in the http layer; this package is transport-free so it stays testable.
package realtime

import (
	"encoding/json"
	"sync"
)

// Event is one thing that happened, ready to be serialized onto an SSE stream.
type Event struct {
	Type string          `json:"type"` // e.g. "task.created"
	Data json.RawMessage `json:"data"` // arbitrary JSON payload
}

// subscriber is one connected client. The hub sends events on ch; the http
// handler ranges over ch and writes each to the wire.
type subscriber struct {
	ch chan Event
}

// Hub fans out events to subscribers grouped by org id. Safe for concurrent use.
type Hub struct {
	mu   sync.RWMutex
	subs map[string]map[*subscriber]struct{} // orgID -> set of subscribers
}

// NewHub returns an empty hub.
func NewHub() *Hub {
	return &Hub{subs: make(map[string]map[*subscriber]struct{})}
}

// Subscription is what a caller holds: a receive-only channel of events and an
// Unsubscribe to release it. Always defer Unsubscribe.
type Subscription struct {
	hub   *Hub
	orgID string
	sub   *subscriber
}

// C returns the channel events arrive on.
func (s *Subscription) C() <-chan Event { return s.sub.ch }

// Unsubscribe removes the subscriber from the hub. Safe to call once.
func (s *Subscription) Unsubscribe() {
	s.hub.unsubscribe(s.orgID, s.sub)
}

// Subscribe registers a new subscriber for an org and returns its Subscription.
// The channel is buffered so a brief lag doesn't drop the next event; if it
// fills, Publish drops events for that subscriber rather than blocking.
func (h *Hub) Subscribe(orgID string) *Subscription {
	sub := &subscriber{ch: make(chan Event, 16)}
	h.mu.Lock()
	if h.subs[orgID] == nil {
		h.subs[orgID] = make(map[*subscriber]struct{})
	}
	h.subs[orgID][sub] = struct{}{}
	h.mu.Unlock()
	return &Subscription{hub: h, orgID: orgID, sub: sub}
}

func (h *Hub) unsubscribe(orgID string, sub *subscriber) {
	h.mu.Lock()
	defer h.mu.Unlock()
	set, ok := h.subs[orgID]
	if !ok {
		return
	}
	if _, ok := set[sub]; ok {
		delete(set, sub)
		close(sub.ch)
	}
	if len(set) == 0 {
		delete(h.subs, orgID)
	}
}

// Publish fans an event out to every subscriber of orgID. The send is
// non-blocking: a subscriber whose buffer is full misses this event rather than
// stalling the publisher (and every other subscriber). Returns how many
// subscribers received it (useful in tests).
func (h *Hub) Publish(orgID string, ev Event) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	delivered := 0
	for sub := range h.subs[orgID] {
		select {
		case sub.ch <- ev:
			delivered++
		default:
			// Slow consumer; drop rather than block the whole org's fan-out.
		}
	}
	return delivered
}

// NewEvent builds an Event, marshaling data to JSON. A marshal error yields an
// event with empty data rather than failing the caller.
func NewEvent(eventType string, data any) Event {
	raw, err := json.Marshal(data)
	if err != nil {
		raw = json.RawMessage(`null`)
	}
	return Event{Type: eventType, Data: raw}
}
