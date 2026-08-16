# Runbook: database slow / connection pool exhausted

**Triggers:** `BeaconDatabaseLatencyHigh`, `BeaconPoolExhausted`
**Owner:** platform
**Last drilled:** never — [run a fire drill](README.md#two-habits-that-keep-these-alive)

## Symptom

One or more of:

- p95 latency on `http_request_duration_seconds` is above 1s across most routes,
  not one route. When *every* endpoint is slow at once, the shared thing is
  usually the database.
- `/readyz` is flapping between 200 and 503.
- Logs are full of `context deadline exceeded` from query calls.
- Requests are timing out but CPU on the API instances is low — a queue, not a
  workload.

If only ONE route is slow, this is the wrong runbook. Profile that route
instead: `go tool pprof http://<instance>:9090/debug/pprof/profile?seconds=30`.

## Diagnostic

Every command in this section is read-only. Run all of them.

```sql
-- 1. Are there long-running queries right now?
SELECT pid, now() - query_start AS duration, state, query
FROM pg_stat_activity
WHERE state != 'idle' AND now() - query_start > interval '5 seconds'
ORDER BY duration DESC;
```

A single query running 30+ seconds is almost always the cause. Note its `pid`.

```sql
-- 2. Is there lock contention?
SELECT blocked.pid AS blocked_pid, blocking.pid AS blocking_pid,
       blocked.query AS blocked_query, blocking.query AS blocking_query
FROM pg_stat_activity AS blocked
JOIN pg_stat_activity AS blocking ON blocking.pid = ANY(pg_blocking_pids(blocked.pid));
```

Contention shows up as a chain of "blocked by". The head of the chain is the
query holding everything else up — that is the one that matters, not the fifty
victims behind it.

```sql
-- 3. Is the connection pool exhausted?
SELECT count(*) AS open_connections,
       sum(case when state = 'idle' then 1 else 0 end) AS idle,
       sum(case when state = 'active' then 1 else 0 end) AS active
FROM pg_stat_activity;
```

At or near `max_connections` (100 on a small managed plan) the pool is saturated
and new requests are simply queueing. Beacon opens at most `MaxConns = 10` per
instance — see `internal/postgres/db.go` — so this number also tells you how
many instances are actually running.

```bash
# 4. Is autovacuum running heavy?
flyctl postgres connect --app beacon-pg
\x
SELECT * FROM pg_stat_progress_vacuum;
```

A long vacuum on a hot table slows every query touching that table.

```bash
# 5. What does the application think is happening? Traces, not guesses.
# Grab a trace id from any slow request's log line and open it in the trace UI —
# the pgx spans show exactly which query, how many times, and how long.
grep 'took=[0-9]\+\.[0-9]*s' /var/log/beacon/api.log | tail -20
```

## Mitigation

**Everything from here changes state. Read before you paste.**

**Case: a runaway query (diagnostic 1).** Decide first whether it is *important*
(a customer's export, a report someone is waiting on) or *runaway* (fired by
hand and forgotten, or a bad code path). For runaway queries only:

```sql
SELECT pg_cancel_backend(<pid>);    -- polite: asks the query to stop
SELECT pg_terminate_backend(<pid>); -- forceful: kills the connection. Use only
                                    -- if cancel did nothing after ~10 seconds.
```

**Case: lock contention (diagnostic 2).** Cancel the head of the chain, not the
victims. Killing a blocked query frees nothing.

**Case: pool exhausted (diagnostic 3), with no single slow query.** The database
is being asked for more than it can do. In order of preference:

```bash
# a. Shed load first: tighten the per-org rate limit so the system protects
#    itself while you work. Ch 19.
flyctl secrets set TENANT_RATE_LIMIT_RPS=2 --app beacon-api-blue

# b. Reduce demand: scale the API DOWN, not up. More instances means more
#    connections against the same ceiling — scaling up here makes it worse.
flyctl scale count 2 --app beacon-api-blue
```

**Case: it started right after a deploy.** Stop diagnosing and roll back. The
rollback is a config flip and takes seconds; you can investigate afterwards with
the site up.

```bash
./deploy/bluegreen/rollback.sh
```

## Escalation

- **After 15 minutes with no improvement**, or if you are about to run
  `pg_terminate_backend` on something you are not sure about: page the platform
  on-call (PagerDuty service `beacon-platform`).
- **If the database itself is unreachable** rather than slow, this is not the
  right runbook — go to disaster recovery (`scripts/restore_drill.sh` and
  Chapter 45's decision tree) and page the platform lead directly.
- **Stop signal:** if p95 is back under 500ms and `/readyz` has been green for
  10 minutes, the incident is mitigated. Mitigated is not fixed — open the
  postmortem ([template](../postmortems/_template.md)) before you go back to
  bed, while you still remember the timeline.
