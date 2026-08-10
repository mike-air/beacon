-- Tasks + comments — every task query scoped by org_id. (Chapter 8 / sqlc)

-- name: CreateTask :one
INSERT INTO tasks (org_id, project_id, title, status, position)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, org_id, project_id, title, status, position, created_at, updated_at;

-- name: ListTasksByProject :many
-- A NULL status arg makes the status filter a no-op (any status).
SELECT id, org_id, project_id, title, status, position, created_at, updated_at
FROM tasks
WHERE org_id = $1 AND project_id = $2
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
ORDER BY position ASC, created_at ASC, id ASC
LIMIT $3 OFFSET $4;

-- name: GetTaskByID :one
SELECT id, org_id, project_id, title, status, position, created_at, updated_at
FROM tasks
WHERE id = $1 AND org_id = $2;

-- name: UpdateTask :one
UPDATE tasks SET title = $3, status = $4, position = $5, updated_at = now()
WHERE id = $1 AND org_id = $2
RETURNING id, org_id, project_id, title, status, position, created_at, updated_at;

-- name: DeleteTask :execrows
DELETE FROM tasks WHERE id = $1 AND org_id = $2;

-- name: CreateComment :one
INSERT INTO comments (task_id, author_id, body)
VALUES ($1, $2, $3)
RETURNING id, task_id, author_id, body, created_at;

-- name: ListCommentsByTask :many
SELECT id, task_id, author_id, body, created_at
FROM comments
WHERE task_id = $1
ORDER BY created_at ASC;
