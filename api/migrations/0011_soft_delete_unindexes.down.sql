DROP TRIGGER IF EXISTS trg_comment_search_soft_delete ON comments;
DROP TRIGGER IF EXISTS trg_project_search_soft_delete ON projects;
DROP TRIGGER IF EXISTS trg_task_search_soft_delete ON tasks;
DROP FUNCTION IF EXISTS sync_search_on_soft_delete();
DROP FUNCTION IF EXISTS reindex_comment_row(uuid);
DROP FUNCTION IF EXISTS reindex_project_row(uuid);
DROP FUNCTION IF EXISTS reindex_task_row(uuid);
