-- Attachments — no org_id of their own; scoped by joining through tasks.org_id.
-- (Chapter 8 / sqlc)
--
-- Chapter 10: attachments get no deleted_at of their own, for the same reason
-- comments don't (see the migration's header) — their visibility, and now
-- whether a task can receive a new one, follows tasks.deleted_at instead.

-- name: TaskInOrg :one
SELECT EXISTS (SELECT 1 FROM tasks WHERE id = $1 AND org_id = $2 AND deleted_at IS NULL);

-- name: CreateAttachment :one
INSERT INTO attachments (task_id, filename, content_type, size_bytes, storage_key)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, task_id, filename, content_type, size_bytes, storage_key, created_at;

-- name: ListAttachmentsByTask :many
SELECT a.id, a.task_id, a.filename, a.content_type, a.size_bytes, a.storage_key, a.created_at
FROM attachments a
JOIN tasks t ON t.id = a.task_id
WHERE a.task_id = $1 AND t.org_id = $2 AND t.deleted_at IS NULL
ORDER BY a.created_at DESC;

-- name: GetAttachmentByID :one
SELECT a.id, a.task_id, a.filename, a.content_type, a.size_bytes, a.storage_key, a.created_at
FROM attachments a
JOIN tasks t ON t.id = a.task_id
WHERE a.id = $1 AND t.org_id = $2 AND t.deleted_at IS NULL;
