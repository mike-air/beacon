-- Editing and deleting a comment.
--
-- This migration invalidates a decision 0009 wrote down, so it says so rather
-- than leaving the two files disagreeing. 0009 gave projects, tasks and
-- webhooks a deleted_at and deliberately withheld one from comments, with this
-- reasoning: comments "have no delete endpoint of their own — nothing in this
-- API lets a caller remove one comment — so there is no independent 'undelete'
-- concept to support."
--
-- That reasoning was sound and is now obsolete: comments get their own DELETE
-- endpoint in this change, which is exactly the condition 0009 named. So they
-- get the same treatment every other directly-deletable resource has — a
-- deleted_at, not a DELETE — and for the same reason: a comment somebody
-- removes is still evidence of what was said, and a hard delete throws away
-- the ability to answer "what happened here" a week later.
ALTER TABLE comments ADD COLUMN deleted_at TIMESTAMPTZ;

-- updated_at exists so an edited comment can be SHOWN as edited. A body that
-- silently changes under a reader is the thing that makes comment editing feel
-- untrustworthy, and the client cannot mark it without knowing.
--
-- It defaults to created_at rather than now() for the rows that already exist:
-- backfilling every historical comment with the moment this migration ran
-- would claim they were all edited today, which is false and is the sort of
-- wrong-but-plausible data nobody questions later. The NOT NULL then holds
-- from the start, so no reader needs a null branch.
ALTER TABLE comments ADD COLUMN updated_at TIMESTAMPTZ;
UPDATE comments SET updated_at = created_at WHERE updated_at IS NULL;
ALTER TABLE comments ALTER COLUMN updated_at SET NOT NULL;
ALTER TABLE comments ALTER COLUMN updated_at SET DEFAULT now();

-- The list query filters on (task_id, deleted_at) now. The existing
-- comments_task_idx covers task_id alone; this replaces it with the pair so
-- the common read stays a single index scan instead of filtering deleted rows
-- out afterwards.
DROP INDEX IF EXISTS comments_task_idx;
CREATE INDEX comments_task_live_idx ON comments (task_id) WHERE deleted_at IS NULL;
