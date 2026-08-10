-- Projects — every query scoped by org_id (the tenant boundary). (Chapter 8 / sqlc)

-- name: CreateProject :one
INSERT INTO projects (org_id, name)
VALUES ($1, $2)
RETURNING id, org_id, name, created_at, updated_at;

-- name: ListProjectsByOrg :many
SELECT id, org_id, name, created_at, updated_at
FROM projects
WHERE org_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: GetProjectByID :one
SELECT id, org_id, name, created_at, updated_at
FROM projects
WHERE id = $1 AND org_id = $2;

-- name: UpdateProject :one
UPDATE projects SET name = $3, updated_at = now()
WHERE id = $1 AND org_id = $2
RETURNING id, org_id, name, created_at, updated_at;

-- name: DeleteProject :execrows
DELETE FROM projects WHERE id = $1 AND org_id = $2;
