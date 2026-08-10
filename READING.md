# How to read this codebase

You did not write this code. That is the point. This repository is your
practice ground for the one skill no course teaches directly: entering a
system somebody else built, orienting in it, and changing it safely.

Work through it with the **Reading Code You Didn't Write** course
(`/docs/reading-code` on Stencil). This file answers the first three of its
four questions so you can start moving. The fourth — tracing one path end to
end — is deliberately left to you, because doing it is the exercise. A fifth
has been added below: trace a path that leaves the process entirely.

---

## Question 1 — What does this thing do?

Beacon is a team task board API. Organizations own projects, projects own
tasks, tasks own comments and attachments. Users exist *outside*
organizations and reach in through a membership that carries a role
(owner / admin / member). Almost every interesting bug in multi-tenant
systems lives in that separation.

Two binaries ship from one module:

- `beacon-api` — the HTTP server (chi), which also runs an in-process
  worker and cron scheduler in dev.
- `beacon-worker` — the same worker and cron, runnable alone, so the queue
  can scale separately from the API.

## Question 2 — How is it arranged?

Behaviour lives in two places: `internal/` (all of it) and `migrations/`
(the schema, embedded into the binary). Everything else is packaging.

| Directory | What it is | Course chapters |
|---|---|---|
| `internal/config` | env vars → one typed Config; nothing else reads env | 3 |
| `internal/observability` | slog setup | 34 |
| `internal/postgres` | pgx pool + migrator | 5–6 |
| `internal/db` | sqlc-generated queries — **never hand-edited** | 8 |
| `internal/http` | router, middleware, handlers, error envelope | 4, 11–13, 16–17, 20 |
| `internal/auth` | argon2id hashing, JWT access + refresh | 15–16 |
| `internal/orgs` `projects` `tasks` `users` | domain services | 21 |
| `internal/storage` / `attachments` | presigned S3 (MinIO locally) | 22 |
| `internal/email` | SMTP sender or log-only sender | 23 |
| `internal/webhooks` | outgoing webhooks, HMAC-signed | 24 |
| `internal/realtime` | per-org SSE fan-out hub | 25 |
| `internal/jobs` / `cron` | Postgres-backed queue + scheduler; nightly backup | 26–27, 45 |
| `internal/cache` | in-process TTL-LRU → Redis → Postgres, singleflight | 28 |
| `internal/search` | Postgres FTS; `meili/` puts Meilisearch in front | 29–30 |
| `internal/flags` | feature flags, user → org → default cascade | 31 |
| `internal/experiments` | stable hash assignment + the audit trail | 32 |
| `internal/i18n` | locale cascade, money as minor units, IANA zones | 33 |
| `internal/pgerr` | Postgres error → domain sentinel translation | 12 |
| `internal/testsupport` | env-gated integration-test harness | 39 |
| `cmd/canary-controller` | the progressive-rollout gate | 47 |
| `docs/runbooks` / `docs/postmortems` | what to do at 3am, and after | 48–49 |

The tenancy decision (chapter 7): a plain `org_id` column on every
multi-tenant table, `WHERE org_id = $1` in every query. No RLS, no
schema-per-tenant. Boring on purpose.

## Question 3 — Where does execution start?

`cmd/beacon-api/main.go`. Read it top to bottom once, without following
anything (chapter 4 of reading-code explains why). It is the table of
contents: config → logger → pool → migrations → services → router → serve.

Run it:

```bash
make db-up        # Postgres + Mailpit + MinIO via docker compose
make run          # migrates on boot, serves on :8080
curl localhost:8080/healthz
```

Tests without Docker: `make test` (integration tests skip themselves
unless `TEST_DATABASE_URL` is set — see `internal/testsupport/db.go` for
why).

## Question 4 — Trace one path. This one is yours.

Do not read about it. Do it, with MAP.md open, per the course:

1. Register a user, create an org, create a project, create a task —
   four curl calls (the handlers in `internal/http/` name the routes).
2. Pick **create task** and trace it: route → middleware chain →
   handler → service → query → row. Write every hop down.
3. Where does `org_id` get checked? Who turns `pgx.ErrNoRows` into a 404?
   Which middleware would reject you before the handler ever ran?

If you can answer those three from your own trace, you know this codebase
better than most people know the one they work in.

---

## Question 5 — trace one path that crosses a boundary

The Question 4 trace stays inside one process. This one does not, and the
difference is most of what makes production systems hard to reason about.

**Trace a signup's welcome email.** It starts in an HTTP handler and finishes in
a worker, minutes later, possibly on a different machine.

1. Start the stack with tracing on:
   `docker compose -f deploy/docker-compose.yml up -d` and `make run`.
2. Sign up a user.
3. Find the request in the log — it carries a `trace_id`.
4. `docker logs beacon-otel | grep <that trace id>`.

You should see the HTTP span, the database spans under it, and — arriving
seconds later — a `job send_email` span with **the same trace ID** and the HTTP
span as its parent. Then answer:

- A `context.Context` cannot be written to a database row. So how did the trace
  get from the handler to the worker? (`internal/jobs/jobs.go`, and the
  `trace_context` column added in migration `0008`.)
- What happens to that span if the worker restarts between the enqueue and the
  run? Read `extractTraceContext` and say what the trace looks like then.
- The route pattern is the span name, not the URL. Find where that is set, and
  work out why it is not set where the chapter puts it. The answer is a comment
  in `internal/http/server.go` and it is about *when* middleware runs.

## What is deliberately missing — your work-list

These course chapters are **not built yet**. Each is a real change to an
inherited codebase — the exact practice the reading-code course ends with:

| Missing | Chapter | Size |
|---|---|---|
| Soft deletes (`deleted_at`) and an audit trail | 10 | medium — **start here** |
| API keys as a second credential type | 18 | small |
| CSRF protection for cookie-authenticated clients | 20 | small |
| Production docker-compose and the deploy itself | 42, 44 | medium |
| Cursor pagination to replace offset | 13 | medium |
| Refresh-token rotation to replace the single access token | 16 | medium |

Rule from the course: write the failing test first, keep the first change in one
file, match the conventions you find (chapter 13 of reading-code — the last
twenty merged changes are the style guide; here, the existing handlers are).

## Five things in here worth reading closely

Not because they are clever. Because each one is a decision with a reason, and
the reason is the part that transfers.

1. **`internal/http/idempotency.go` + `internal/db/queries/idempotency.sql`** —
   one SQL statement does claim-or-find atomically, using `xmax = 0` to tell
   which happened. The unique index is the synchronisation primitive; the Go
   code just reads a boolean. Ask yourself what the two-step version (SELECT
   then INSERT) would do under two simultaneous retries.
2. **`internal/cache/withcache.go`** — the `singleflight` call. Delete it in
   your head and work out what 200 concurrent requests do to Postgres the
   instant a hot key expires.
3. **`internal/search/search.go`** — `Search` tries Meilisearch and falls back
   to Postgres on *any* error. Find every place that decision is protected
   (hint: the write path, the delete path, and what is authoritative) and work
   out what would break if a handler wrote to Meili directly.
4. **`internal/experiments/assignment.go`** — twenty lines, no I/O, no clock,
   no randomness. The null byte between the two inputs is not decoration;
   `assignment_test.go` says what it prevents.
5. **`cmd/canary-controller/main.go`** — `or vector(0)` in the error-rate
   query, and the minimum-traffic guard on the latency check. Both exist
   because the obvious version of the gate fails on healthy deploys. That is
   the usual reason odd-looking code exists.

## House rules this codebase already follows

- Only `internal/config` touches `os.Getenv`. Everything else takes the
  typed Config.
- Every error crossing the HTTP boundary becomes the one envelope:
  `{ "error": { code, message, fields, details } }` — see
  `internal/http/errors.go`.
- Domain packages export sentinels (`ErrNotFound`, `ErrConflict`);
  driver errors are translated at the repository edge (`internal/pgerr`).
- `internal/db` is generated by `sqlc generate` from `migrations/` +
  `*.sql` query files. Edit the SQL, regenerate, commit both.
- Soft deletes (`deleted_at`), optimistic locking (`version`), and
  `org_id` scoping on every tenant-owned table.

## What has actually been run

Read this before trusting anything above. Everything in Phases 1–5 that can run
on a laptop has been run against real infrastructure — real Postgres, real
Redis, real Meilisearch, a real Prometheus, a real OpenTelemetry collector — and
watched doing the thing the chapter says it does, including the failure cases.

Two things have **not** been run: the blue-green and canary routers
(`deploy/bluegreen/worker.js`, `deploy/canary/worker.js`) and the deploy
scripts. They need a Fly.io account and a Cloudflare Worker with a KV namespace.
The controller that drives them was tested against a real Prometheus and made to
both pass and fail its gate, but no cutover has happened here. Treat those two
files as read-and-understand, not as verified.
