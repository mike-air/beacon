package http

import (
	"context"
	"encoding/json"

	"beacon/internal/jobs"
	"beacon/internal/realtime"
)

// Side-effect orchestration for domain events. When a task is created/updated/
// deleted, the HANDLER (after the service call succeeds) calls notifyTaskEvent,
// which (1) publishes to the realtime hub for live SSE subscribers and (2)
// enqueues a webhook delivery job for every active subscriber of the event.
//
// This lives in the http layer on purpose: it stitches together transport
// concerns (SSE, outbound HTTP webhooks, the job queue) so the domain packages
// (tasks, webhooks) stay transport-free and never import one another.
//
// Course mapping: Chapter 24 — enqueue webhook deliveries; Chapter 25 — publish
// to the SSE hub. The "keep orchestration in the http layer" choice is the
// build spec's, noted here so the boundary stays visible.

// Task event types.
const (
	eventTaskCreated = "task.created"
	eventTaskUpdated = "task.updated"
	eventTaskDeleted = "task.deleted"
)

// notifyTaskEvent fans a task event out to live subscribers and registered
// webhooks. Best-effort: failures are logged, never surfaced to the caller (the
// user-visible write already succeeded).
func (s *Server) notifyTaskEvent(ctx context.Context, orgID, eventType string, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		s.logger.Error("notify: marshal payload", "err", err, "event", eventType)
		return
	}

	// 1) Live fan-out to SSE subscribers for this org.
	s.realtime.Publish(orgID, realtime.Event{Type: eventType, Data: json.RawMessage(body)})

	// 2) Enqueue a webhook delivery job for each active subscriber.
	hooks, err := s.webhooks.ActiveForEvent(ctx, orgID, eventType)
	if err != nil {
		s.logger.Error("notify: list webhooks", "err", err, "event", eventType)
		return
	}
	for _, hook := range hooks {
		deliveryID, err := s.webhooks.CreateDelivery(ctx, hook.ID, eventType, body)
		if err != nil {
			s.logger.Error("notify: create delivery", "err", err, "webhook_id", hook.ID)
			continue
		}
		if err := s.jobs.Enqueue(ctx, jobs.KindDeliverWebhook, jobs.DeliverWebhookPayload{
			WebhookID:  hook.ID,
			DeliveryID: deliveryID,
			Event:      eventType,
			Body:       json.RawMessage(body),
		}); err != nil {
			s.logger.Error("notify: enqueue webhook", "err", err, "webhook_id", hook.ID)
		}
	}
}
