-- Chapter 29 — full-text search, in Postgres, with no new infrastructure.
--
-- The shape is the chapter's: ONE flat table with entity_kind/entity_id, a GIN
-- index on the tsvector, and a B-tree on organization_id so every query can be
-- tenant-scoped. Search results across three different entity types come out of
-- one index instead of three unioned queries.
--
-- [verbatim ch29] with the column names this repo actually has:
--   * the chapter's tasks table has a `description`; ours has `title` and
--     `status` only (migration 0001), so a task indexes its title at weight A
--     and its project's name at weight C, and the B band is left to comments;
--   * our tasks carry org_id directly, so the task trigger needs no subquery to
--     find the tenant. The chapter's does, because its tasks reach the org
--     through the project.
-- The table, the weights, the GIN index and the upsert trigger are unchanged.

CREATE TABLE search_index (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid        NOT NULL,
    entity_kind     text        NOT NULL,         -- 'task', 'project', 'comment'
    entity_id       uuid        NOT NULL,
    title           text        NOT NULL,
    body            text        NOT NULL DEFAULT '',
    search_vector   tsvector    NOT NULL,
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (entity_kind, entity_id)
);

CREATE INDEX search_index_org    ON search_index (organization_id);
CREATE INDEX search_index_vector ON search_index USING GIN (search_vector);


-- A shared delete helper. The chapter names delete_search_index('task') in its
-- trigger; this is that function.
CREATE OR REPLACE FUNCTION delete_search_index() RETURNS trigger AS $$
BEGIN
    DELETE FROM search_index
     WHERE entity_kind = TG_ARGV[0] AND entity_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;


-- Tasks. setweight labels the lexemes so ts_rank knows a hit in the title is
-- worth more than a hit in the project name it happens to sit under.
CREATE OR REPLACE FUNCTION reindex_task() RETURNS trigger AS $$
DECLARE
    proj_name text;
BEGIN
    SELECT name INTO proj_name FROM projects WHERE id = NEW.project_id;

    INSERT INTO search_index (organization_id, entity_kind, entity_id, title, body, search_vector, updated_at)
    VALUES (
        NEW.org_id,
        'task',
        NEW.id,
        NEW.title,
        '',
        setweight(to_tsvector('english', NEW.title), 'A') ||
        setweight(to_tsvector('english', COALESCE(proj_name, '')), 'C'),
        now()
    )
    ON CONFLICT (entity_kind, entity_id) DO UPDATE SET
        title         = EXCLUDED.title,
        body          = EXCLUDED.body,
        search_vector = EXCLUDED.search_vector,
        updated_at    = now();

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_task_search
AFTER INSERT OR UPDATE OF title, project_id ON tasks
FOR EACH ROW EXECUTE FUNCTION reindex_task();

CREATE TRIGGER trg_task_search_delete
AFTER DELETE ON tasks
FOR EACH ROW EXECUTE FUNCTION delete_search_index('task');


-- Projects.
CREATE OR REPLACE FUNCTION reindex_project() RETURNS trigger AS $$
BEGIN
    INSERT INTO search_index (organization_id, entity_kind, entity_id, title, body, search_vector, updated_at)
    VALUES (
        NEW.org_id,
        'project',
        NEW.id,
        NEW.name,
        '',
        setweight(to_tsvector('english', NEW.name), 'A'),
        now()
    )
    ON CONFLICT (entity_kind, entity_id) DO UPDATE SET
        title         = EXCLUDED.title,
        search_vector = EXCLUDED.search_vector,
        updated_at    = now();

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_project_search
AFTER INSERT OR UPDATE OF name ON projects
FOR EACH ROW EXECUTE FUNCTION reindex_project();

CREATE TRIGGER trg_project_search_delete
AFTER DELETE ON projects
FOR EACH ROW EXECUTE FUNCTION delete_search_index('project');


-- Comments. A comment reaches its org through its task, so this one does need
-- the subquery. Its own text is the B band; the task's title rides along at C
-- so searching for the task's words still surfaces the discussion under it.
CREATE OR REPLACE FUNCTION reindex_comment() RETURNS trigger AS $$
DECLARE
    t_org   uuid;
    t_title text;
BEGIN
    SELECT org_id, title INTO t_org, t_title FROM tasks WHERE id = NEW.task_id;

    INSERT INTO search_index (organization_id, entity_kind, entity_id, title, body, search_vector, updated_at)
    VALUES (
        t_org,
        'comment',
        NEW.id,
        COALESCE(t_title, 'comment'),
        NEW.body,
        setweight(to_tsvector('english', NEW.body), 'B') ||
        setweight(to_tsvector('english', COALESCE(t_title, '')), 'C'),
        now()
    )
    ON CONFLICT (entity_kind, entity_id) DO UPDATE SET
        title         = EXCLUDED.title,
        body          = EXCLUDED.body,
        search_vector = EXCLUDED.search_vector,
        updated_at    = now();

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_comment_search
AFTER INSERT OR UPDATE OF body ON comments
FOR EACH ROW EXECUTE FUNCTION reindex_comment();

CREATE TRIGGER trg_comment_search_delete
AFTER DELETE ON comments
FOR EACH ROW EXECUTE FUNCTION delete_search_index('comment');


-- Backfill anything that already exists. Triggers only fire on future writes;
-- a fresh index over an old table is empty until you say this once.
INSERT INTO search_index (organization_id, entity_kind, entity_id, title, body, search_vector)
SELECT p.org_id, 'project', p.id, p.name, '', setweight(to_tsvector('english', p.name), 'A')
FROM projects p
ON CONFLICT (entity_kind, entity_id) DO NOTHING;

INSERT INTO search_index (organization_id, entity_kind, entity_id, title, body, search_vector)
SELECT t.org_id, 'task', t.id, t.title, '',
       setweight(to_tsvector('english', t.title), 'A') ||
       setweight(to_tsvector('english', COALESCE(p.name, '')), 'C')
FROM tasks t LEFT JOIN projects p ON p.id = t.project_id
ON CONFLICT (entity_kind, entity_id) DO NOTHING;

INSERT INTO search_index (organization_id, entity_kind, entity_id, title, body, search_vector)
SELECT t.org_id, 'comment', c.id, t.title, c.body,
       setweight(to_tsvector('english', c.body), 'B') ||
       setweight(to_tsvector('english', t.title), 'C')
FROM comments c JOIN tasks t ON t.id = c.task_id
ON CONFLICT (entity_kind, entity_id) DO NOTHING;
