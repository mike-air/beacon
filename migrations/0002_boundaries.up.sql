-- Phase 3 — the "boundaries" tables: outgoing webhooks, their delivery log, and
-- a Postgres-backed background-job queue.
--
-- Course mapping: Chapter 24 — webhooks + webhook_deliveries (the course names
-- the delivery states pending/success/failed/dead; we keep a plain TEXT status
-- with a CHECK so the hand-written pgx repo stays simple). Chapter 26 — the job
-- queue: the course uses River, which creates its own `river_job` table via its
-- migration; we model the same shape in a hand-written `jobs` table polled with
-- FOR UPDATE SKIP LOCKED (the very Postgres trick the chapter explains).

-- Outgoing webhooks. One row per registered endpoint, per org.
CREATE TABLE webhooks (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    url        TEXT NOT NULL,
    secret     TEXT NOT NULL,
    events     TEXT[] NOT NULL DEFAULT '{}',
    active     BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX webhooks_org_idx ON webhooks (org_id);

-- Every delivery attempt's record: the payload we signed, the latest status,
-- how many tries it has had, and the last error. This is both the audit log and
-- the dead-letter queue (status = 'dead').
CREATE TABLE webhook_deliveries (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id   UUID NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event        TEXT NOT NULL,
    payload      JSONB NOT NULL,
    status       TEXT NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending', 'success', 'failed', 'dead')),
    attempts     INT NOT NULL DEFAULT 0,
    last_error   TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    delivered_at TIMESTAMPTZ
);
CREATE INDEX webhook_deliveries_webhook_idx ON webhook_deliveries (webhook_id);
CREATE INDEX webhook_deliveries_status_idx  ON webhook_deliveries (status);

-- The background-job queue. A worker claims a due, pending row with
-- FOR UPDATE SKIP LOCKED, runs the registered handler for its `kind`, and either
-- marks it done or bumps `attempts` and reschedules `run_at` with backoff. When
-- attempts reaches max_attempts the row is parked as 'dead'.
CREATE TABLE jobs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind         TEXT NOT NULL,
    payload      JSONB NOT NULL DEFAULT '{}',
    run_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    attempts     INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 5,
    status       TEXT NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending', 'running', 'done', 'dead')),
    last_error   TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- The hot path of the poller: cheapest way to find the next due pending job.
CREATE INDEX jobs_due_idx ON jobs (run_at) WHERE status = 'pending';
