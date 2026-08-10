-- Users — identity. The one table with no org_id. (Chapter 8 / sqlc)

-- name: CreateUser :one
INSERT INTO users (email, name, password_hash)
VALUES ($1, $2, $3)
RETURNING id, email, name, password_hash, created_at;

-- name: GetUserByEmail :one
SELECT id, email, name, password_hash, created_at
FROM users
WHERE lower(email) = lower($1);

-- name: GetUserByID :one
SELECT id, email, name, password_hash, created_at
FROM users
WHERE id = $1;
