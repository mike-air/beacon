package realtime

import (
	"encoding/json"
	"testing"
	"time"
)

// Course mapping: Chapter 25 — the per-org fan-out hub. Subscribe, publish,
// receive — and confirm the org keying isolates tenants.
func TestHubSubscribePublishReceive(t *testing.T) {
	hub := NewHub()
	sub := hub.Subscribe("org-1")
	defer sub.Unsubscribe()

	ev := NewEvent("task.created", map[string]string{"id": "t1"})
	delivered := hub.Publish("org-1", ev)
	if delivered != 1 {
		t.Fatalf("expected delivery to 1 subscriber, got %d", delivered)
	}

	select {
	case got := <-sub.C():
		if got.Type != "task.created" {
			t.Fatalf("wrong event type: %q", got.Type)
		}
		var data map[string]string
		if err := json.Unmarshal(got.Data, &data); err != nil {
			t.Fatalf("bad event data: %v", err)
		}
		if data["id"] != "t1" {
			t.Fatalf("wrong event payload: %v", data)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for published event")
	}
}

func TestHubIsolatesOrgs(t *testing.T) {
	hub := NewHub()
	sub := hub.Subscribe("org-1")
	defer sub.Unsubscribe()

	// An event for a different org must not reach org-1's subscriber.
	if n := hub.Publish("org-2", NewEvent("task.created", nil)); n != 0 {
		t.Fatalf("expected 0 deliveries to org-2 (no subscribers), got %d", n)
	}

	select {
	case got := <-sub.C():
		t.Fatalf("org-1 subscriber received a cross-tenant event: %+v", got)
	case <-time.After(100 * time.Millisecond):
		// Correct: nothing arrived.
	}
}

func TestHubUnsubscribeStopsDelivery(t *testing.T) {
	hub := NewHub()
	sub := hub.Subscribe("org-1")
	sub.Unsubscribe()

	if n := hub.Publish("org-1", NewEvent("task.updated", nil)); n != 0 {
		t.Fatalf("expected 0 deliveries after unsubscribe, got %d", n)
	}
}

func TestHubDropsWhenBufferFull(t *testing.T) {
	hub := NewHub()
	sub := hub.Subscribe("org-1")
	defer sub.Unsubscribe()

	// Fill the buffer (16) plus extra; Publish must never block.
	done := make(chan int, 1)
	go func() {
		total := 0
		for i := 0; i < 100; i++ {
			total += hub.Publish("org-1", NewEvent("noise", nil))
		}
		done <- total
	}()

	select {
	case <-done:
		// Publish returned without blocking — the non-blocking send works.
	case <-time.After(time.Second):
		t.Fatal("Publish blocked on a full subscriber buffer")
	}
}
