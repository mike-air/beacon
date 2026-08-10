-- Chapter 32 — experiments. The lookup is cached; the assignment insert is the
-- audit trail and runs off the request's critical path.
--
-- [verbatim ch32, expressed as the sqlc queries the chapter's service calls.]

-- name: GetExperiment :one
SELECT key, description, status, variants, started_at, stopped_at, created_at
FROM experiments
WHERE key = $1;

-- name: InsertAssignmentIfAbsent :exec
INSERT INTO experiment_assignments (experiment_key, user_id, variant)
VALUES ($1, $2, $3)
ON CONFLICT (experiment_key, user_id) DO NOTHING;

-- name: CountAssignmentsByVariant :many
-- How you actually read an experiment: join the assignment to whatever outcome
-- you care about. This is the shape without the outcome, so the split itself
-- can be sanity-checked first.
SELECT variant, count(*) AS n
FROM experiment_assignments
WHERE experiment_key = $1
GROUP BY variant
ORDER BY variant;

-- name: SetExperimentStatus :exec
UPDATE experiments
SET status = $2,
    started_at = CASE WHEN $2 = 'running' AND started_at IS NULL THEN now() ELSE started_at END,
    stopped_at = CASE WHEN $2 = 'stopped' THEN now() ELSE stopped_at END
WHERE key = $1;
