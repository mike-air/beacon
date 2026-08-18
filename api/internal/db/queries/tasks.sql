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
-- Comments now have a deleted_at of their own — 0010 added it along with the
-- DELETE endpoint whose absence was 0009's stated reason for withholding one.
-- So a comment read checks BOTH: the parent task's deleted_at through the
-- join, and the comment's own. The join is also where org_id is checked, and
-- it closes a latent gap this query had before Chapter 10 touched it: it
-- filtered by task_id alone, so an org member who knew another org's task id
-- could read its comments.

-- name: CreateTask :one
-- INSERT ... SELECT, not VALUES, so the org owns the project or nothing is
-- written.
--
-- The plain VALUES form checked two foreign keys independently — org_id names
-- a real org, project_id names a real project — and never that the two belong
-- together. A member of org A could therefore POST a task to org B's project
-- id and have it accepted: the row landed with A's org_id and B's project_id.
-- It leaked no data (every read here is scoped by org_id, so B never saw it
-- and A never saw B's tasks), but it let one tenant write rows referencing
-- another tenant's project, and it made the endpoint an existence oracle for
-- project ids.
--
-- The guard belongs here rather than in a handler for the same reason the rest
-- of the org scoping does: a SELECT that finds no matching project inserts no
-- row, atomically, on every path that reaches this query — the single-task
-- create and the bulk import both — with no extra round trip and nothing for a
-- future caller to forget. sqlc's :one then returns pgx.ErrNoRows, which the
-- repo maps to ErrNotFound: from outside, a project you cannot write to is
-- indistinguishable from one that does not exist, which is the right answer.
INSERT INTO tasks (org_id, project_id, title, status, position)
SELECT $1, $2, $3, $4, $5
FROM projects
WHERE projects.id = $2
  AND projects.org_id = $1
  AND projects.deleted_at IS NULL
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

-- name: MaxTaskPosition :one
-- The highest position currently in a project, for appending imported tasks
-- after the existing cards. COALESCE makes an empty project answer 0 rather
-- than NULL, so the caller gets a float64 and not a null-handling branch.
SELECT COALESCE(MAX(position), 0)::float8
FROM tasks
WHERE org_id = $1 AND project_id = $2 AND deleted_at IS NULL;

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
RETURNING id, task_id, author_id, body, created_at, updated_at, deleted_at;

-- name: ListCommentsByTask :many
SELECT comments.id, comments.task_id, comments.author_id, comments.body,
       comments.created_at, comments.updated_at, comments.deleted_at
FROM comments
JOIN tasks ON tasks.id = comments.task_id
WHERE comments.task_id = $1 AND tasks.org_id = $2
  AND tasks.deleted_at IS NULL AND comments.deleted_at IS NULL
ORDER BY comments.created_at ASC;

-- name: GetCommentByID :one
-- Scoped through the parent task's org, the same join every other comment
-- query uses. The service reads this BEFORE an edit or a delete, because both
-- decisions need the author id and neither can be made from the request alone.
SELECT comments.id, comments.task_id, comments.author_id, comments.body,
       comments.created_at, comments.updated_at, comments.deleted_at
FROM comments
JOIN tasks ON tasks.id = comments.task_id
WHERE comments.id = $1 AND tasks.org_id = $2
  AND tasks.deleted_at IS NULL AND comments.deleted_at IS NULL;

-- name: UpdateComment :one
-- The author_id in the WHERE clause is the authorisation, not just a filter.
-- The service checks it too and returns a clearer error, but repeating it here
-- means no future caller can edit somebody else's words by reaching the repo
-- directly — the same argument as CreateTask's INSERT ... SELECT.
UPDATE comments SET body = $4, updated_at = now()
FROM tasks
WHERE comments.id = $1 AND comments.task_id = tasks.id
  AND tasks.org_id = $2 AND comments.author_id = $3
  AND tasks.deleted_at IS NULL AND comments.deleted_at IS NULL
RETURNING comments.id, comments.task_id, comments.author_id, comments.body,
          comments.created_at, comments.updated_at, comments.deleted_at;

-- name: SoftDeleteComment :execrows
-- No author_id here, unlike UpdateComment: an org admin may delete a comment
-- they did not write (moderation), so the caller check cannot live in the SQL.
-- The service decides, and :execrows lets it tell "not yours" from "not there".
UPDATE comments SET deleted_at = now()
FROM tasks
WHERE comments.id = $1 AND comments.task_id = tasks.id
  AND tasks.org_id = $2
  AND tasks.deleted_at IS NULL AND comments.deleted_at IS NULL;
