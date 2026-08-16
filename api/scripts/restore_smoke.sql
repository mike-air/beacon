-- Chapter 45 — the smoke queries the restore drill runs against the restored
-- copy. They are deliberately boring: the question is not "is the data
-- perfect", it is "did a real database come back, with the tables and the rows
-- and the constraints, or did we restore an empty shell and not notice".
--
-- ON_ERROR_STOP makes any failure exit non-zero, which is what makes the drill
-- a pass/fail rather than a wall of text somebody skims.
\set ON_ERROR_STOP on

-- 1. Every table we expect exists.
DO $$
DECLARE
    missing text;
BEGIN
    SELECT string_agg(t, ', ') INTO missing
    FROM unnest(ARRAY[
        'users','organizations','memberships','projects','tasks','comments',
        'attachments','webhooks','webhook_deliveries','jobs',
        'idempotency_keys','search_index','feature_flags','experiments'
    ]) AS t
    WHERE to_regclass('public.' || t) IS NULL;

    IF missing IS NOT NULL THEN
        RAISE EXCEPTION 'restored database is missing tables: %', missing;
    END IF;
END $$;

-- 2. The core tables are not empty. An empty restore "succeeds" silently.
DO $$
DECLARE
    n bigint;
BEGIN
    SELECT count(*) INTO n FROM users;
    IF n = 0 THEN RAISE EXCEPTION 'restored database has no users'; END IF;
    RAISE NOTICE 'users: %', n;

    SELECT count(*) INTO n FROM organizations;
    IF n = 0 THEN RAISE EXCEPTION 'restored database has no organizations'; END IF;
    RAISE NOTICE 'organizations: %', n;
END $$;

-- 3. Referential integrity survived: no orphans anywhere it matters.
DO $$
DECLARE
    n bigint;
BEGIN
    SELECT count(*) INTO n FROM tasks t
     LEFT JOIN projects p ON p.id = t.project_id WHERE p.id IS NULL;
    IF n > 0 THEN RAISE EXCEPTION '% orphaned tasks', n; END IF;

    SELECT count(*) INTO n FROM memberships m
     LEFT JOIN users u ON u.id = m.user_id WHERE u.id IS NULL;
    IF n > 0 THEN RAISE EXCEPTION '% orphaned memberships', n; END IF;
END $$;

-- 4. The migration ledger came back, so the restored copy can be migrated
--    forward rather than being a dead end.
SELECT count(*) AS migrations_applied FROM schema_migrations;

-- 5. How fresh is it? This is the RPO, measured rather than assumed.
SELECT
    max(created_at)                   AS newest_row,
    now() - max(created_at)           AS data_age
FROM tasks;
