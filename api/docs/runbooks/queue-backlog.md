# Runbook: job queue backlog

**Trigger:** `BeaconQueueBacklog` — pending jobs above 1,000 for 10 minutes
**Owner:** platform

## Symptom

- Welcome emails, webhook deliveries or search reindexes are arriving late, or
  not at all.
- `beacon_jobs_completed_total` has flattened while writes continue.
- Customers report "I invited someone and they never got the email".

The API is usually fine during this. That is what makes it easy to miss: nothing
is down, work is just silently piling up behind a queue.

## Diagnostic

Read-only. Run all of them.

```sql
-- 1. How big is the backlog, and which job kinds dominate it?
SELECT kind, status, count(*)
FROM jobs
GROUP BY kind, status
ORDER BY count(*) DESC;
```

One kind dominating means one handler is the problem. An even spread means the
workers themselves have stopped.

```sql
-- 2. Are workers actually picking up jobs?
SELECT count(*) FILTER (WHERE status = 'running')  AS running,
       count(*) FILTER (WHERE status = 'pending')  AS pending,
       min(run_at) FILTER (WHERE status = 'pending') AS oldest_pending
FROM jobs;
```

`running = 0` with a large `pending` means no worker is claiming. `oldest_pending`
tells you how far behind you are in wall-clock terms, which is the number to put
in the incident channel.

```sql
-- 3. Are jobs failing and retrying in a loop?
SELECT kind, attempts, left(last_error, 120) AS error, count(*)
FROM jobs
WHERE status IN ('pending', 'dead') AND last_error <> ''
GROUP BY kind, attempts, left(last_error, 120)
ORDER BY count(*) DESC
LIMIT 20;
```

A retry loop looks like a backlog and is not one — the queue is moving, it is
just moving in a circle.

```sql
-- 4. Anything stuck in 'running' with no worker behind it?
--    The hourly cron sweeps these back to pending; if you see many, a worker
--    died mid-job.
SELECT count(*) FROM jobs
WHERE status = 'running' AND created_at < now() - interval '10 minutes';
```

```bash
# 5. Are the worker processes alive?
flyctl status --app beacon-worker
flyctl logs --app beacon-worker | tail -50
```

## Mitigation

**From here on, things change.**

**Case: a retry loop on one kind (diagnostic 3).** Stop the loop before adding
capacity — more workers just means failing faster. If the failing kind is
webhook delivery, pause the endpoint that is failing:

```sql
-- pause one webhook endpoint so its failures stop consuming queue capacity
UPDATE webhooks SET active = false WHERE id = '<webhook_id>';
```

If the failing kind is anything else and the error is clearly a bug in a new
deploy, roll back:

```bash
./deploy/bluegreen/rollback.sh
```

**Case: workers are dead or too few (diagnostics 2, 5).** Scale out. Unlike the
database runbook, scaling UP is the right move here — the queue claims rows with
`FOR UPDATE SKIP LOCKED`, so workers do not contend with each other.

```bash
flyctl scale count 4 --app beacon-worker
```

**Case: jobs stuck in `running` (diagnostic 4).** The hourly cron sweep handles
this on its own. To do it now:

```sql
UPDATE jobs SET status = 'pending'
WHERE status = 'running' AND created_at < now() - interval '10 minutes';
```

**Case: a huge backlog of work that no longer matters** (for example, reindex
jobs queued behind a full reindex that has since run). Dropping work is a
decision, not a shortcut — say so in the incident channel first, then:

```sql
DELETE FROM jobs WHERE kind = 'search_reindex' AND status = 'pending';
-- and then queue one job that does it all properly:
INSERT INTO jobs (kind, payload) VALUES ('search_reindex_all', '{}'::jsonb);
```

Never do this for `send_email` or `deliver_webhook`. Those are promises to
somebody outside the company.

## Escalation

- **After 20 minutes**, or the moment you are considering deleting jobs that
  represent a promise to a customer: page the platform on-call.
- **If the backlog is caused by the database** (workers are alive but every
  claim is slow), you are in the wrong runbook — go to
  [database slow](db-slow.md).
- **Stop signal:** `pending` is falling steadily and `oldest_pending` is under
  five minutes. Then write the postmortem: a queue that got 1,000 jobs behind
  had a cause, and the cause is still there.
