# How to read this codebase

You did not write this code. That is the point. This repository is your
practice ground for the one skill no course teaches directly: entering a
system somebody else built, orienting in it, and changing it safely.

Work through it with the **Reading Code You Didn't Write** course
(`/docs/reading-code` on Stencil). This file answers the first three of its
four questions so you can start moving. The fourth — tracing one path end to
end — is deliberately left to you, because doing it is the exercise.

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
| `internal/jobs` / `cron` | Postgres-backed queue + scheduler | 26–27 |
| `internal/pgerr` | Postgres error → domain sentinel translation | 12 |
| `internal/testsupport` | env-gated integration-test harness | 39 |

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

## What is deliberately missing — your work-list

These course chapters are **not built yet**. Each one is a real change to
an inherited codebase — the exact practice the reading-code course ends
with, and each maps to a chapter you can follow while implementing:

| Missing | Chapter | Size |
|---|---|---|
| Idempotency keys on mutating endpoints | 14 | small, self-contained — **start here** |
| Rate limiting (per-tenant, per-IP) | 19 | small |
| Caching (HTTP + in-process) | 28 | medium |
| Postgres full-text search | 29 | medium |
| Feature flags | 31 | small |
| Metrics, pprof, tracing | 35–36 | medium |
| Deploy story (Fly/K8s, CI gates, backups) | 41–50 | large |

Rule from the course: write the failing test first, keep the first change
in one file, match the conventions you find (chapter 13 of reading-code —
the last twenty merged changes are the style guide; here, the existing
handlers are).

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

First commit is yours to make. The repo is initialised; `.env` is
gitignored; nothing is staged.
