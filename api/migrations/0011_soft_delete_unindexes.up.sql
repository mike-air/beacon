-- A soft delete must leave the search index, and did not.
--
-- Chapter 29 wired every searchable table to search_index with two triggers:
-- reindex on INSERT/UPDATE of the indexed columns, and remove on DELETE.
-- Chapter 10 then replaced deletion with a deleted_at flag. Nothing connected
-- the two, so "delete" stopped meaning "delete" as far as search was
-- concerned: the DELETE trigger no longer fires for anything a user does, and
-- the reindex triggers do not watch deleted_at.
--
-- The result is a row a user deleted, still returned by search, with its title
-- and a body snippet. SearchOrg reads search_index alone — no join back to the
-- source table — so nothing downstream filters it either. Confirmed against a
-- real database before writing this: two soft-deleted tasks and one
-- soft-deleted project were still indexed, and still findable.
--
-- Search is org-scoped, so this was never a cross-tenant leak. It is still a
-- user deleting something and then finding it again.
--
-- The fix belongs in a trigger rather than in the Go delete paths for one
-- concrete reason: deleting a project cascades to its tasks through
-- SoftDeleteTasksByProject, a single bulk UPDATE. Go code would have to
-- enumerate the affected task ids to clean the index after it; a row-level
-- trigger sees every one of them for free, in the same transaction, on every
-- path that exists today or is added later.
--
-- Restore is handled too. Nothing exposes an undelete today, but deleted_at is
-- nullable and the pair should be symmetric: a row that ever comes back comes
-- back searchable, rather than being silently absent from search in a way
-- nobody would think to check.

-- The reindex functions in 0004 are trigger functions — they read NEW and
-- cannot be called for an arbitrary id — so the restore branch needs these.
-- Each mirrors its 0004 counterpart EXACTLY, including which text lands in
-- `body` and which only reaches the vector: a task's body is empty and its
-- project name appears at weight C alone, which is easy to get wrong by
-- reading the vector and assuming body matches it.
CREATE OR REPLACE FUNCTION reindex_task_row(p_task_id uuid) RETURNS void AS $$
BEGIN
    INSERT INTO search_index (organization_id, entity_kind, entity_id, title, body, search_vector, updated_at)
    SELECT t.org_id, 'task', t.id, t.title, '',
           setweight(to_tsvector('english', t.title), 'A') ||
           setweight(to_tsvector('english', COALESCE(p.name, '')), 'C'),
           now()
    FROM tasks t LEFT JOIN projects p ON p.id = t.project_id
    WHERE t.id = p_task_id
    ON CONFLICT (entity_kind, entity_id) DO UPDATE SET
        title         = EXCLUDED.title,
        body          = EXCLUDED.body,
        search_vector = EXCLUDED.search_vector,
        updated_at    = now();
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION reindex_project_row(p_project_id uuid) RETURNS void AS $$
BEGIN
    INSERT INTO search_index (organization_id, entity_kind, entity_id, title, body, search_vector, updated_at)
    SELECT p.org_id, 'project', p.id, p.name, '',
           setweight(to_tsvector('english', p.name), 'A'),
           now()
    FROM projects p
    WHERE p.id = p_project_id
    ON CONFLICT (entity_kind, entity_id) DO UPDATE SET
        title         = EXCLUDED.title,
        search_vector = EXCLUDED.search_vector,
        updated_at    = now();
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION reindex_comment_row(p_comment_id uuid) RETURNS void AS $$
BEGIN
    INSERT INTO search_index (organization_id, entity_kind, entity_id, title, body, search_vector, updated_at)
    SELECT t.org_id, 'comment', c.id, COALESCE(t.title, 'comment'), c.body,
           setweight(to_tsvector('english', c.body), 'B') ||
           setweight(to_tsvector('english', COALESCE(t.title, '')), 'C'),
           now()
    FROM comments c JOIN tasks t ON t.id = c.task_id
    WHERE c.id = p_comment_id
    ON CONFLICT (entity_kind, entity_id) DO UPDATE SET
        title         = EXCLUDED.title,
        body          = EXCLUDED.body,
        search_vector = EXCLUDED.search_vector,
        updated_at    = now();
END;
$$ LANGUAGE plpgsql;

-- One trigger function for all three tables; the entity kind arrives as a
-- trigger argument. The transitions are explicit rather than "deleted_at IS
-- NOT NULL": an UPDATE that touches deleted_at without changing it — the
-- cascade re-running, a backfill — should do nothing, not re-delete or
-- re-insert an index row on every write.
CREATE OR REPLACE FUNCTION sync_search_on_soft_delete() RETURNS trigger AS $$
BEGIN
    IF NEW.deleted_at IS NOT NULL AND OLD.deleted_at IS NULL THEN
        DELETE FROM search_index
        WHERE entity_kind = TG_ARGV[0] AND entity_id = NEW.id;
    ELSIF NEW.deleted_at IS NULL AND OLD.deleted_at IS NOT NULL THEN
        IF TG_ARGV[0] = 'task' THEN
            PERFORM reindex_task_row(NEW.id);
        ELSIF TG_ARGV[0] = 'project' THEN
            PERFORM reindex_project_row(NEW.id);
        ELSIF TG_ARGV[0] = 'comment' THEN
            PERFORM reindex_comment_row(NEW.id);
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_task_search_soft_delete
AFTER UPDATE OF deleted_at ON tasks
FOR EACH ROW EXECUTE FUNCTION sync_search_on_soft_delete('task');

CREATE TRIGGER trg_project_search_soft_delete
AFTER UPDATE OF deleted_at ON projects
FOR EACH ROW EXECUTE FUNCTION sync_search_on_soft_delete('project');

CREATE TRIGGER trg_comment_search_soft_delete
AFTER UPDATE OF deleted_at ON comments
FOR EACH ROW EXECUTE FUNCTION sync_search_on_soft_delete('comment');

-- Clean up what the gap already let through. Without this the fix applies only
-- to future deletes, and everything deleted before today stays searchable —
-- which is the state this migration exists to end.
DELETE FROM search_index si
WHERE (si.entity_kind = 'task'
       AND EXISTS (SELECT 1 FROM tasks t WHERE t.id = si.entity_id AND t.deleted_at IS NOT NULL))
   OR (si.entity_kind = 'project'
       AND EXISTS (SELECT 1 FROM projects p WHERE p.id = si.entity_id AND p.deleted_at IS NOT NULL))
   OR (si.entity_kind = 'comment'
       AND EXISTS (SELECT 1 FROM comments c WHERE c.id = si.entity_id AND c.deleted_at IS NOT NULL));
