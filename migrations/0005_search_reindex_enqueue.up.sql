-- Chapter 30's write path, expressed against this repo's queue.
--
-- The chapter's rule is precise: the transaction that writes the row must ALSO
-- enqueue the reindex job, in the same commit, so you can never end up with a
-- row in Postgres and no job to copy it into Meilisearch.
--
-- The chapter achieves that with River's InsertTx inside the service's Go
-- transaction. This repo's task and project writes are single statements with
-- no explicit transaction to hang that off (see internal/tasks, internal/
-- projects), so the enqueue happens where the write already is: inside the
-- Chapter 29 reindex trigger, which by definition runs in the writer's
-- transaction. Same guarantee, arrived at from the database side.
--
-- If Meilisearch is switched off, the worker's handler is a no-op and the job
-- completes instantly — jobs still get written, they just do nothing. That is
-- the price of putting the decision in SQL rather than in Go.

CREATE OR REPLACE FUNCTION enqueue_search_reindex(p_kind text, p_id uuid)
RETURNS void AS $$
BEGIN
    INSERT INTO jobs (kind, payload)
    VALUES ('search_reindex',
            jsonb_build_object('kind', p_kind, 'entity_id', p_id));
END;
$$ LANGUAGE plpgsql;


CREATE OR REPLACE FUNCTION reindex_task() RETURNS trigger AS $$
DECLARE
    proj_name text;
BEGIN
    SELECT name INTO proj_name FROM projects WHERE id = NEW.project_id;

    INSERT INTO search_index (organization_id, entity_kind, entity_id, title, body, search_vector, updated_at)
    VALUES (
        NEW.org_id, 'task', NEW.id, NEW.title, '',
        setweight(to_tsvector('english', NEW.title), 'A') ||
        setweight(to_tsvector('english', COALESCE(proj_name, '')), 'C'),
        now()
    )
    ON CONFLICT (entity_kind, entity_id) DO UPDATE SET
        title         = EXCLUDED.title,
        body          = EXCLUDED.body,
        search_vector = EXCLUDED.search_vector,
        updated_at    = now();

    PERFORM enqueue_search_reindex('task', NEW.id);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;


CREATE OR REPLACE FUNCTION reindex_project() RETURNS trigger AS $$
BEGIN
    INSERT INTO search_index (organization_id, entity_kind, entity_id, title, body, search_vector, updated_at)
    VALUES (
        NEW.org_id, 'project', NEW.id, NEW.name, '',
        setweight(to_tsvector('english', NEW.name), 'A'),
        now()
    )
    ON CONFLICT (entity_kind, entity_id) DO UPDATE SET
        title         = EXCLUDED.title,
        search_vector = EXCLUDED.search_vector,
        updated_at    = now();

    PERFORM enqueue_search_reindex('project', NEW.id);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;


CREATE OR REPLACE FUNCTION reindex_comment() RETURNS trigger AS $$
DECLARE
    t_org   uuid;
    t_title text;
BEGIN
    SELECT org_id, title INTO t_org, t_title FROM tasks WHERE id = NEW.task_id;

    INSERT INTO search_index (organization_id, entity_kind, entity_id, title, body, search_vector, updated_at)
    VALUES (
        t_org, 'comment', NEW.id, COALESCE(t_title, 'comment'), NEW.body,
        setweight(to_tsvector('english', NEW.body), 'B') ||
        setweight(to_tsvector('english', COALESCE(t_title, '')), 'C'),
        now()
    )
    ON CONFLICT (entity_kind, entity_id) DO UPDATE SET
        title         = EXCLUDED.title,
        body          = EXCLUDED.body,
        search_vector = EXCLUDED.search_vector,
        updated_at    = now();

    PERFORM enqueue_search_reindex('comment', NEW.id);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
