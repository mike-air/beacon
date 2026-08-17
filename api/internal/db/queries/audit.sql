-- Audit log — one row per mutation an org needs to be able to answer "who,
-- and when" about. (Chapter 10 / sqlc)

-- name: InsertAuditEntry :exec
INSERT INTO audit_log (org_id, actor_id, action, resource_type, resource_id)
VALUES ($1, $2, $3, $4, $5);

-- name: ListAuditLog :many
SELECT id, org_id, actor_id, action, resource_type, resource_id, created_at
FROM audit_log
WHERE org_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;
