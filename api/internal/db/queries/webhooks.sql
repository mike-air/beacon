-- Webhooks + deliveries — webhooks scoped by org_id; deliveries by webhook_id.
-- (Chapter 8 / sqlc)
--
-- Chapter 10: delete is soft. ActiveWebhooksForEvent and GetWebhook both gain
-- the deleted_at check — the first so a soft-deleted webhook stops receiving
-- new events, the second so the delivery worker (which loads a webhook by id
-- alone, unscoped by org, to sign a payload) stops finding one that should no
-- longer exist.
--
-- deleted_at is selected/returned everywhere below for the same reason
-- projects.sql does it — see that file's header — even though every query
-- here already guarantees it is NULL.

-- name: CreateWebhook :one
INSERT INTO webhooks (org_id, url, secret, events)
VALUES ($1, $2, $3, $4)
RETURNING id, org_id, url, secret, events, active, created_at, deleted_at;

-- name: ListWebhooksByOrg :many
SELECT id, org_id, url, secret, events, active, created_at, deleted_at
FROM webhooks
WHERE org_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: ActiveWebhooksForEvent :many
SELECT id, org_id, url, secret, events, active, created_at, deleted_at
FROM webhooks
WHERE org_id = $1 AND active = true AND deleted_at IS NULL
  AND (cardinality(events) = 0 OR sqlc.arg('event')::text = ANY(events))
ORDER BY created_at ASC;

-- name: GetWebhookByOrg :one
SELECT id, org_id, url, secret, events, active, created_at, deleted_at
FROM webhooks
WHERE id = $1 AND org_id = $2 AND deleted_at IS NULL;

-- name: GetWebhook :one
SELECT id, org_id, url, secret, events, active, created_at, deleted_at
FROM webhooks
WHERE id = $1 AND deleted_at IS NULL;

-- name: SoftDeleteWebhook :execrows
UPDATE webhooks SET deleted_at = now()
WHERE id = $1 AND org_id = $2 AND deleted_at IS NULL;

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
