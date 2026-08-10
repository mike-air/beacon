-- Chapter 14 — idempotency. Two queries, and the first one is the whole trick.
--
-- [verbatim ch14] except for the file's location: the chapter puts this at
-- internal/http/idempotency_queries.sql behind a second sqlc target. This repo
-- generates every query into one internal/db package (see sqlc.yaml), so it
-- lives here with the rest. The SQL is unchanged.

-- name: ClaimIdempotencyKey :one
-- Atomic: insert the row if no key exists for this (user_id, key), or
-- return the existing row if one does. The "claimed" boolean tells the
-- caller which happened.
INSERT INTO idempotency_keys (user_id, key, request_hash, method, path)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id, key) DO UPDATE SET key = EXCLUDED.key
RETURNING
    request_hash,
    method,
    path,
    response_status,
    response_body,
    completed_at,
    (xmax = 0) AS claimed;

-- name: CompleteIdempotencyKey :exec
UPDATE idempotency_keys
SET response_status = $3, response_body = $4, completed_at = now()
WHERE user_id = $1 AND key = $2 AND completed_at IS NULL;

-- name: SweepIdempotencyKeys :execrows
-- The daily cron entry: 24 hours is long enough for any sane retry and short
-- enough that the table stays small (Chapter 14, the "housekeeping" section).
DELETE FROM idempotency_keys WHERE created_at < now() - interval '24 hours';
