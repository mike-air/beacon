DROP INDEX IF EXISTS comments_task_live_idx;
CREATE INDEX comments_task_idx ON comments (task_id);
ALTER TABLE comments DROP COLUMN IF EXISTS updated_at;
ALTER TABLE comments DROP COLUMN IF EXISTS deleted_at;
