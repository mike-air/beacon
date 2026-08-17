-- Chapter 10 — soft deletes and an audit trail.
--
-- projects, tasks and webhooks are the three resources a user can directly
-- delete. Each gets a deleted_at instead of losing the row: every read query
-- gains "AND deleted_at IS NULL", and delete becomes an UPDATE.
--
-- Comments and attachments deliberately do NOT get their own deleted_at. They
-- have no delete endpoint of their own — nothing in this API lets a caller
-- remove one comment or one attachment — so there is no independent
-- "undelete" concept to support. Their visibility instead follows their
-- parent task's: reading them now joins to tasks and checks tasks.deleted_at,
-- which is also where the same join closes a latent gap — those two list
-- queries were never scoped by org_id, only by task_id, so an org member who
-- knew another org's task id could read its comments today. Fixed as part of
-- the same join, not as a separate change.
ALTER TABLE projects ADD COLUMN deleted_at TIMESTAMPTZ;
ALTER TABLE tasks    ADD COLUMN deleted_at TIMESTAMPTZ;
ALTER TABLE webhooks ADD COLUMN deleted_at TIMESTAMPTZ;

-- One row per mutation an org's owner or admin would want to be able to
-- answer "who did this, and when" about. Today that is exactly the three
-- deletes above — see internal/audit's header for why the audit write and
-- the soft delete it describes share one transaction.
CREATE TABLE audit_log (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    actor_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    action        TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id   UUID NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX audit_log_org_idx ON audit_log (org_id, created_at DESC);
