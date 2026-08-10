-- Reverting only removes the enqueue; the Chapter 29 index and its triggers
-- (migration 0004) stay, because Postgres FTS does not depend on the queue.
DROP FUNCTION IF EXISTS enqueue_search_reindex(text, uuid) CASCADE;
