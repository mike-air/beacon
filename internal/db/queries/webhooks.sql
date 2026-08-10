-- Webhooks + deliveries — webhooks scoped by org_id; deliveries by webhook_id.
-- (Chapter 8 / sqlc)

-- name: CreateWebhook :one
INSERT INTO webhooks (org_id, url, secret, events)
VALUES ($1, $2, $3, $4)
RETURNING id, org_id, url, secret, events, active, created_at;

-- name: ListWebhooksByOrg :many
SELECT id, org_id, url, secret, events, active, created_at
FROM webhooks
WHERE org_id = $1
ORDER BY created_at DESC;

-- name: ActiveWebhooksForEvent :many
SELECT id, org_id, url, secret, events, active, created_at
FROM webhooks
WHERE org_id = $1 AND active = true
  AND (cardinality(events) = 0 OR sqlc.arg('event')::text = ANY(events))
ORDER BY created_at ASC;

-- name: GetWebhookByOrg :one
SELECT id, org_id, url, secret, events, active, created_at
FROM webhooks
WHERE id = $1 AND org_id = $2;

-- name: GetWebhook :one
SELECT id, org_id, url, secret, events, active, created_at
FROM webhooks
WHERE id = $1;

-- name: DeleteWebhook :execrows
DELETE FROM webhooks WHERE id = $1 AND org_id = $2;

-- name: CreateDelivery :one
INSERT INTO webhook_deliveries (webhook_id, event, payload)
VALUES ($1, $2, $3)
RETURNING id;

-- name: MarkDeliverySuccess :exec
UPDATE webhook_deliveries
SET status = 'success', attempts = $2, last_error = '', delivered_at = now()
WHERE id = $1;

-- name: MarkDeliveryStatus :exec
UPDATE webhook_deliveries
SET status = $2, attempts = $3, last_error = $4
WHERE id = $1;
