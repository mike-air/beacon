-- Projects — every query scoped by org_id (the tenant boundary). (Chapter 8 / sqlc)
--
-- Chapter 10: delete is soft. Every read below excludes deleted_at IS NOT
-- NULL rows; the delete itself is SoftDeleteProject, an UPDATE, not a DELETE.
--
-- deleted_at is selected/returned everywhere below even though every one of
-- these queries already guarantees it is NULL (that is what "AND deleted_at
-- IS NULL" means) — leaving it out would make sqlc's column list a strict
-- subset of the projects table's, which stops it treating the result as a
-- Project and gives every query its own near-identical Row type instead. The
-- Go field this produces is real but always nil; internal/projects tags it
-- `json:"-"` so it never reaches the API despite existing in Go.

-- name: CreateProject :one
INSERT INTO projects (org_id, name)
VALUES ($1, $2)
RETURNING id, org_id, name, created_at, updated_at, deleted_at;

-- name: ListProjectsByOrg :many
SELECT id, org_id, name, created_at, updated_at, deleted_at
FROM projects
WHERE org_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: GetProjectByID :one
SELECT id, org_id, name, created_at, updated_at, deleted_at
FROM projects
WHERE id = $1 AND org_id = $2 AND deleted_at IS NULL;

-- name: UpdateProject :one
UPDATE projects SET name = $3, updated_at = now()
WHERE id = $1 AND org_id = $2 AND deleted_at IS NULL
RETURNING id, org_id, name, created_at, updated_at, deleted_at;

-- name: SoftDeleteProject :execrows
UPDATE projects SET deleted_at = now()
WHERE id = $1 AND org_id = $2 AND deleted_at IS NULL;
