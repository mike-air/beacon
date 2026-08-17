// Job handlers: the actual work behind each job kind. Registered on the worker
// in NewServer. These bridge the queue to the email and webhook packages.
//
// Course mapping: Chapter 23 — the send_email handler does the SMTP call off the
// request path; Chapter 24 — the deliver_webhook handler signs and POSTs the
// payload, and records the attempt in webhook_deliveries (the worker's
// retry/backoff drives the DLQ).

package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"beacon/internal/email"
	"beacon/internal/webhooks"
)

// SendEmailPayload is the JSON body of a send_email job.
type SendEmailPayload struct {
	To       string `json:"to"`
	Subject  string `json:"subject"`
	HTMLBody string `json:"html_body"`
	TextBody string `json:"text_body"`
}

// DeliverWebhookPayload is the JSON body of a deliver_webhook job. DeliveryID is
// the pre-created webhook_deliveries row this attempt updates.
type DeliverWebhookPayload struct {
	WebhookID  string          `json:"webhook_id"`
	DeliveryID string          `json:"delivery_id"`
	Event      string          `json:"event"`
	Body       json.RawMessage `json:"body"`
}

// EmailHandler returns a Handler that sends the email via the Sender.
func EmailHandler(sender email.Sender) Handler {
	return func(ctx context.Context, raw json.RawMessage) error {
		var p SendEmailPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return fmt.Errorf("send_email: bad payload: %w", err)
		}
		return sender.Send(ctx, p.To, p.Subject, p.HTMLBody, p.TextBody)
	}
}

// WebhookHandler returns a Handler that signs and POSTs the payload to the
// webhook's URL and records the outcome on the delivery row. A non-2xx response
// or transport error returns an error so the worker retries with backoff; when
// retries are exhausted the worker parks the job dead and we also mark the
// delivery row dead.
func WebhookHandler(repo *webhooks.Repo, client *http.Client) Handler {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return func(ctx context.Context, raw json.RawMessage) error {
		var p DeliverWebhookPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return fmt.Errorf("deliver_webhook: bad payload: %w", err)
		}

		hook, err := repo.GetWebhook(ctx, p.WebhookID)
		if err != nil {
			// Webhook deleted between enqueue and delivery: nothing to do, and
			// retrying won't help — treat as terminal success for the job.
			_ = repo.MarkDeliveryFailed(ctx, p.DeliveryID, 1, "webhook no longer exists", true)
			return nil
		}

		signature := webhooks.Sign(hook.Secret, p.Body)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, hook.URL, bytes.NewReader(p.Body))
		if err != nil {
			return w(repo, ctx, p.DeliveryID, fmt.Errorf("deliver_webhook: build request: %w", err))
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(webhooks.SignatureHeader, signature)
		req.Header.Set("X-Beacon-Event", p.Event)

		resp, err := client.Do(req)
		if err != nil {
			return w(repo, ctx, p.DeliveryID, fmt.Errorf("deliver_webhook: POST: %w", err))
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return w(repo, ctx, p.DeliveryID, fmt.Errorf("deliver_webhook: receiver returned %d", resp.StatusCode))
		}

		// Success. attempts is recorded best-effort; the canonical attempt count
		// is the job's, but mirroring it onto the delivery row keeps the audit
		// log readable.
		_ = repo.MarkDeliverySuccess(ctx, p.DeliveryID, 1)
		return nil
	}
}

// w records a failed delivery attempt (non-terminal — the worker decides when to
// give up) and returns the cause so the worker retries.
func w(repo *webhooks.Repo, ctx context.Context, deliveryID string, cause error) error {
	_ = repo.MarkDeliveryFailed(ctx, deliveryID, 1, cause.Error(), false)
	return cause
}
