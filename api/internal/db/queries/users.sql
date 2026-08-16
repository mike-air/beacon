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

-- Chapter 33 — the locale cascade's first two steps live in one row each.
-- One query, because resolving a locale on every request must not be two.

-- name: GetUserPreferences :one
SELECT u.locale, u.timezone, COALESCE(o.default_locale, '') AS org_default_locale
FROM users u
LEFT JOIN organizations o ON o.id = $2
WHERE u.id = $1;

-- name: SetUserPreferences :exec
UPDATE users SET locale = $2, timezone = $3 WHERE id = $1;
