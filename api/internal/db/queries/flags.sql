-- Chapter 31 — feature flags. Two reads on the hot path, both cached for 30
-- seconds; the rest are the admin surface.
--
-- [verbatim ch31, expressed as the sqlc queries the chapter's service calls.]

-- name: GetFeatureFlag :one
SELECT name, description, default_value, created_at, updated_at
FROM feature_flags
WHERE name = $1;

-- name: ListFeatureFlagOverrides :many
SELECT flag_name, org_id, user_id, value, created_at
FROM feature_flag_overrides
WHERE flag_name = $1;

-- name: ListFeatureFlags :many
SELECT name, description, default_value, created_at, updated_at
FROM feature_flags
ORDER BY name;

-- name: SetFeatureFlagDefault :exec
UPDATE feature_flags SET default_value = $2, updated_at = now() WHERE name = $1;

-- name: UpsertOrgFlagOverride :exec
INSERT INTO feature_flag_overrides (flag_name, org_id, value)
VALUES ($1, $2, $3)
ON CONFLICT (flag_name, org_id) WHERE org_id IS NOT NULL
DO UPDATE SET value = EXCLUDED.value;

-- name: UpsertUserFlagOverride :exec
INSERT INTO feature_flag_overrides (flag_name, user_id, value)
VALUES ($1, $2, $3)
ON CONFLICT (flag_name, user_id) WHERE user_id IS NOT NULL
DO UPDATE SET value = EXCLUDED.value;
