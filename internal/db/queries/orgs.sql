-- Organizations + memberships — the multi-tenant core. (Chapter 8 / sqlc)

-- name: CreateOrg :one
INSERT INTO organizations (name, slug)
VALUES ($1, $2)
RETURNING id, name, slug, created_at;

-- name: AddMembership :one
INSERT INTO memberships (user_id, org_id, role)
VALUES ($1, $2, $3)
RETURNING id, user_id, org_id, role, created_at;

-- name: GetMembership :one
SELECT id, user_id, org_id, role, created_at
FROM memberships
WHERE user_id = $1 AND org_id = $2;

-- name: ListOrgsForUser :many
SELECT o.id, o.name, o.slug, o.created_at, m.role
FROM organizations o
JOIN memberships m ON m.org_id = o.id
WHERE m.user_id = $1
ORDER BY o.created_at DESC;

-- name: ListMembers :many
SELECT u.id, u.email, u.name, m.role
FROM memberships m
JOIN users u ON u.id = m.user_id
WHERE m.org_id = $1
ORDER BY m.created_at ASC;

-- name: FindUserIDByEmail :one
SELECT id
FROM users
WHERE lower(email) = lower($1);
