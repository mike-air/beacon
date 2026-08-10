-- Chapter 14 — idempotency. One small table is the whole feature: it remembers
-- which keys a user has already spent, what request they went with, and what we
-- answered, so a retry can be replayed instead of re-run.
--
-- [verbatim ch14] with one adaptation: the chapter numbers this file 0014
-- because its Beacon has one migration per chapter. This repo migrates in
-- phases, so it is 0003 here and the rest is unchanged.

CREATE TABLE idempotency_keys (
    -- The key the client sent, scoped to the user. Two different users
    -- can pick the same key and not collide.
    key            text   NOT NULL,
    user_id        uuid   NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- SHA-256 of the request body, so we detect "same key, different body".
    -- 32 bytes, stored as bytea.
    request_hash   bytea  NOT NULL,

    -- The method and path lock the key to one operation. A client cannot
    -- reuse a key from POST /tasks for a DELETE /tasks/{id}.
    method         text   NOT NULL,
    path           text   NOT NULL,

    -- The stored response. Null while the request is still in flight.
    response_status int,
    response_body   bytea,

    -- Lifecycle. Created when the request first arrives, completed when
    -- the handler returns. Stale rows older than 24h are swept by cron.
    created_at     timestamptz NOT NULL DEFAULT now(),
    completed_at   timestamptz,

    PRIMARY KEY (user_id, key)
);

-- Sweep index: the cleanup job deletes rows where created_at < now() - 24h.
CREATE INDEX idempotency_keys_created_at_idx ON idempotency_keys (created_at);
