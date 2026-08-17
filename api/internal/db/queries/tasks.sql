-- Tasks + comments — every task query scoped by org_id. (Chapter 8 / sqlc)
--
-- Chapter 10: delete is soft. Every task read below excludes deleted_at IS
-- NOT NULL rows; SoftDeleteTask replaces DeleteTask, and
-- SoftDeleteTasksByProject is the cascade a project's own soft delete uses —
-- see internal/projects's Delete for why that cascade has to be explicit now
-- instead of the ON DELETE CASCADE foreign key it replaces.
--
-- deleted_at is selected/returned everywhere below for the same reason
-- projects.sql does it — see that file's header — even though every query
-- here already guarantees it is NULL.
--
-- Comments have no deleted_at of their own (see the migration's header for
-- why); ListCommentsByTask instead joins tasks and checks it there, which is
-- also where it now checks org_id — that join closes a latent gap this query
-- had before Chapter 10 touched it: it filtered by task_id alone, so an org
-- member who knew another org's task id could read its comments.

-- name: CreateTask :one
INSERT INTO tasks (org_id, project_id, title, status, position)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, org_id, project_id, title, status, position, created_at, updated_at, deleted_at;

-- name: ListTasksByProject :many
-- A NULL status arg makes the status filter a no-op (any status).
SELECT id, org_id, project_id, title, status, position, created_at, updated_at, deleted_at
FROM tasks
WHERE org_id = $1 AND project_id = $2 AND deleted_at IS NULL
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
ORDER BY position ASC, created_at ASC, id ASC
LIMIT $3 OFFSET $4;

-- name: GetTaskByID :one
SELECT id, org_id, project_id, title, status, position, created_at, updated_at, deleted_at
FROM tasks
WHERE id = $1 AND org_id = $2 AND deleted_at IS NULL;

-- name: UpdateTask :one
UPDATE tasks SET title = $3, status = $4, position = $5, updated_at = now()
WHERE id = $1 AND org_id = $2 AND deleted_at IS NULL
RETURNING id, org_id, project_id, title, status, position, created_at, updated_at, deleted_at;

-- name: SoftDeleteTask :execrows
UPDATE tasks SET deleted_at = now()
WHERE id = $1 AND org_id = $2 AND deleted_at IS NULL;

-- name: SoftDeleteTasksByProject :exec
-- The cascade a project's soft delete performs explicitly, in the same
-- transaction, in place of the ON DELETE CASCADE the hard-delete version got
-- for free from the foreign key.
UPDATE tasks SET deleted_at = now()
WHERE project_id = $1 AND deleted_at IS NULL;

-- name: CreateComment :one
INSERT INTO comments (task_id, author_id, body)
VALUES ($1, $2, $3)
RETURNING id, task_id, author_id, body, created_at;

-- name: ListCommentsByTask :many
SELECT comments.id, comments.task_id, comments.author_id, comments.body, comments.created_at
FROM comments
JOIN tasks ON tasks.id = comments.task_id
WHERE comments.task_id = $1 AND tasks.org_id = $2 AND tasks.deleted_at IS NULL
ORDER BY comments.created_at ASC;
