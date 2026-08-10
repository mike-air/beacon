-- Attachments — no org_id of their own; scoped by joining through tasks.org_id.
-- (Chapter 8 / sqlc)

-- name: TaskInOrg :one
SELECT EXISTS (SELECT 1 FROM tasks WHERE id = $1 AND org_id = $2);

-- name: CreateAttachment :one
INSERT INTO attachments (task_id, filename, content_type, size_bytes, storage_key)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, task_id, filename, content_type, size_bytes, storage_key, created_at;

-- name: ListAttachmentsByTask :many
SELECT a.id, a.task_id, a.filename, a.content_type, a.size_bytes, a.storage_key, a.created_at
FROM attachments a
JOIN tasks t ON t.id = a.task_id
WHERE a.task_id = $1 AND t.org_id = $2
ORDER BY a.created_at DESC;

-- name: GetAttachmentByID :one
SELECT a.id, a.task_id, a.filename, a.content_type, a.size_bytes, a.storage_key, a.created_at
FROM attachments a
JOIN tasks t ON t.id = a.task_id
WHERE a.id = $1 AND t.org_id = $2;
