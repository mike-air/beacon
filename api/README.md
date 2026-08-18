# Beacon

The production-ready Go API from the **go-production-api** course — a team task board (orgs → projects → tasks → comments → attachments). This repo is the project you'd have at the end of the course, built so you can read it, run it, and break it while you study.

> How to use it: don't just read. Open a file, retype it into your own copy, run it, change something, see what breaks. The reps are the point.

## The seven nouns

`users` live outside organizations. A `membership` ties a user to an `organization` with a role (owner / admin / member). `projects` belong to orgs; `tasks` to projects; `comments` and `attachments` to tasks. Everything a team owns is scoped by `org_id`.

## Layout

```
cmd/beacon-api/main.go     ← entry point; read it top-to-bottom to see what the binary does
cmd/beacon-worker/main.go  ← the background worker + cron, runnable on its own (Ch 26–27)
internal/
  config/                ← env vars → typed Config            (Ch 3)
  observability/         ← structured logger (slog)            (Ch 34)
  postgres/             ← pgx pool + migrator                 (Ch 5, 6)
  db/                    ← sqlc-generated queries (one package) (Ch 8)
  http/                  ← router, middleware, handlers        (Ch 4, 12, 20)
  storage/               ← presigned S3 upload/download URLs   (Ch 22)
  attachments/           ← attachment metadata over storage    (Ch 22)
  email/                 ← Sender: SMTP (go-mail) or LogSender  (Ch 23)
  webhooks/              ← outgoing webhooks + HMAC signing     (Ch 24)
  realtime/              ← in-memory per-org SSE fan-out hub     (Ch 25)
  jobs/                  ← Postgres-backed job queue + worker   (Ch 26, 45)
  cron/                  ← in-process scheduler (robfig/cron)   (Ch 27)
  cache/                 ← in-process LRU + Redis + singleflight (Ch 28)
  search/                ← Postgres FTS, then Meilisearch        (Ch 29–30)
  flags/                 ← feature flags with a 30s TTL cache    (Ch 31)
  experiments/           ← stable hash-based A/B assignment      (Ch 32)
  i18n/                  ← locale cascade, money, time zones     (Ch 33)
cmd/canary-controller/   ← the progressive-rollout gate          (Ch 47)
migrations/              ← SQL schema, embedded into the binary (Ch 5–10, 14, 24, 26, 29–33, 36)
deploy/                  ← docker-compose + blue-green, canary, prometheus, otel configs
docs/runbooks/           ← incident runbooks                     (Ch 48)
docs/postmortems/        ← the postmortem template + how to run one (Ch 49)
scripts/                 ← restore drill, smoke test, k6 load test (Ch 45, 46, 50)
```

The one import rule: `cmd/` may import `internal/`; nothing in `internal/` imports `cmd/`. Domain packages never import `internal/http`.

## Run it

```bash
cp .env.example .env
make db-up          # start Postgres in Docker
make run            # migrates on boot, then serves on :8080
```

Postgres is the only hard requirement. Everything else degrades cleanly when it
is not configured — no Redis means the cache is per-process, no `MEILI_URL`
means search runs on Postgres, no OTLP endpoint means tracing is off. To get the
whole stack:

```bash
docker compose -f deploy/docker-compose.yml up -d
```

That adds Redis (Ch 28), Meilisearch (Ch 30), Prometheus at :9091 (Ch 35, 47),
an OpenTelemetry collector (Ch 36), Mailpit and MinIO.

Then:

```bash
curl localhost:8080/healthz   # {"status":"ok"}
curl localhost:8080/readyz    # {"status":"ready"}  (pings the DB)
curl localhost:8080/v1/       # {"service":"beacon-api","status":"ok"}
curl localhost:9090/metrics   # Prometheus metrics — the INTERNAL port (Ch 35)
```

First run, `make tidy` once to download dependencies and write `go.sum`.

## Build status — by chapter

This repo is built in verified passes. Phase 1 is the bootable core; the rest land in order.

**Done — Phase 1 (the spine):**
- [x] Ch 1–2 — project + layout
- [x] Ch 3 — configuration (`internal/config`)
- [x] Ch 4 — HTTP server + middleware chain (`internal/http`)
- [x] Ch 5 — Postgres + pgx pool (`internal/postgres/db.go`)
- [x] Ch 6 — migrations (simple embedded migrator; swaps to golang-migrate)
- [x] Ch 34 — structured logging (brought forward; everything needs a logger)
- [x] Ch 37 — health checks (`/healthz`, `/readyz`)
- [x] Graceful shutdown (`cmd/beacon-api/main.go`)
- [x] The seven-table schema (`migrations/0001_init.up.sql`)

**Done — Phase 2 (auth, multi-tenancy, domain CRUD):**
- [x] Ch 7 — multi-tenancy (every org-owned query scoped by `org_id`)
- [x] Ch 11 — request validation (`go-playground/validator`, `decodeAndValidate`)
- [x] Ch 12 — error handling (domain sentinels → one HTTP translator, `errorEnvelope` kept)
- [x] Ch 13 — pagination (offset `?limit=&offset=`, default 20 / max 100)
- [x] Ch 15 — password hashing (argon2id, `internal/auth/password.go`)
- [x] Ch 16 — JWT auth (HS256 issue/verify, `requireAuth` middleware)
- [x] Ch 17 — RBAC (role on membership; `requireOrg` + `requireRole`)
- [x] Ch 21 — service layer (one service per domain noun)
- [x] Ch 8 — sqlc repositories (`users`, `orgs`, `projects`, `tasks`, `attachments`, `webhooks`): queries generated by sqlc into `internal/db`; each repo maps the generated rows into its domain struct

**Done — Phase 3 (the boundaries):**
- [x] Ch 22 — file uploads to S3 (`internal/storage`, `internal/attachments`; presigned PUT/GET, MinIO-compatible, 501 when unconfigured)
- [x] Ch 23 — transactional email (`internal/email`; SMTP via go-mail or a LogSender; sent from a background job, never inline)
- [x] Ch 24 — outgoing webhooks (`internal/webhooks`; register/list/delete, HMAC-SHA256 `X-Beacon-Signature`, delivered + retried by the worker, DLQ via `webhook_deliveries`)
- [x] Ch 25 — real-time SSE (`internal/realtime`; in-memory per-org hub; `GET /v1/orgs/{orgID}/events`)
- [x] Ch 26 — background jobs (`internal/jobs`; **Postgres-backed queue**, not River — see the note below; in-process worker + standalone `cmd/beacon-worker`)
- [x] Ch 27 — cron (`internal/cron`; robfig/cron, hourly sweep + heartbeat, started from the API and the worker)

**Done — Phase 4 (job-readiness):**
- [x] Ch 38 — unit tests (config parsing, pagination, error mapping, validation flatten, Backoff, RoleRank — pure logic, no DB)
- [x] Ch 39 — integration tests (repos + services against a real Postgres, incl. the cross-tenant isolation showcase test)
- [x] Ch 40 — e2e tests (the real router over `httptest.Server`: signup → login → org → project → task → list, plus 401 and cross-tenant 403)
- [x] Ch 41 — `Dockerfile` (multi-stage, static CGO-free binary, distroless non-root final stage) + `.dockerignore`
- [x] Ch 43 — CI (`.github/workflows/ci.yml`: lint/build/unit, integration on a postgres:16 service container, docker build)

**Done — Phase 5 (scale, switches, and operations):**
- [x] Ch 14 — idempotency keys (`internal/http/idempotency.go`; `Idempotency-Key` on mutating routes, four cases, atomic claim-or-find via `ON CONFLICT ... RETURNING (xmax = 0)`, daily cron sweep)
- [x] Ch 19 — rate limiting (`internal/http/middleware_ratelimit.go`; token bucket per org for authenticated traffic, per IP for `/auth/*`, `Retry-After` on 429, GC loop so the IP map can't leak)
- [x] Ch 28 — caching (`internal/cache`; in-process TTL-LRU → Redis → Postgres with `singleflight`, `Cache-Control`/`ETag`/`Vary` + 304 on `GET` one project, invalidate-after-commit)
- [x] Ch 29 — Postgres full-text search (`search_index` + GIN, weighted `tsvector` maintained by triggers, `plainto_tsquery`, `ts_rank`, `ts_headline`)
- [x] Ch 30 — Meilisearch in front of it (`internal/search/meili`; typo tolerance, tenant filter, reindex jobs enqueued in the writer's transaction, **falls back to Postgres** when Meili is down)
- [x] Ch 31 — feature flags (`internal/flags`; user → org → default cascade, 30s TTL cache, fails closed)
- [x] Ch 32 — A/B testing (`internal/experiments`; FNV-64 stable assignment, async audit row, flag gates then experiment splits)
- [x] Ch 33 — i18n (`internal/i18n`; four-source locale cascade, message catalog with English as key and fallback, `go-money`, IANA zones)
- [x] Ch 35 — Prometheus metrics (`internal/observability/metrics.go` + middleware; route PATTERN labels, on a separate internal port)
- [x] Ch 36 — OpenTelemetry (`internal/observability/trace.go`; HTTP → service → pgx spans, `trace_id` on every log line, trace context carried across the job queue)
- [x] Ch 45 — backups & DR (`internal/jobs/backup.go` pipes `pg_dump | gzip | age` straight to off-provider storage; `scripts/restore_drill.sh` + `scripts/restore_smoke.sql`)
- [x] Ch 46 — blue-green (`deploy/bluegreen/`; Cloudflare Worker reading `live_color`, deploy + rollback scripts, `scripts/smoke.sh`)
- [x] Ch 47 — canary (`deploy/canary/worker.js` + `cmd/canary-controller`; 1→10→50→100 ladder, four gate signals, back to 0% and exit non-zero on failure)
- [x] Ch 48 — runbooks (`docs/runbooks/`; database slow, queue backlog, upstream 429s — Symptom / Diagnostic / Mitigation / Escalation)
- [x] Ch 49 — postmortems (`docs/postmortems/`; the template and how to run the meeting)
- [x] Ch 50 — profiling & load (`/debug/pprof` behind the internal port + a token, `scripts/load/list_tasks.js` for k6)

**Next passes (not yet built):**
- [ ] Ch 9–10 — transactions, soft deletes & audit
- [ ] Ch 18 — API keys
- [ ] Ch 20 — CORS/CSRF hardening beyond the current CORS middleware
- [ ] Ch 42, 44 — docker-compose for production, deploying

> A note on fidelity: each pass makes independent calls where a later chapter formalizes things. The CRUD repositories are now **sqlc-generated** (Ch 8): the queries live in `internal/db/queries/*.sql` and sqlc emits the row structs and query functions into a **single `internal/db` package** (`package db`) rather than one generated package per domain. That single-package layout is a layout choice — still plain sqlc-generated code; it just keeps one importable `Queries` type. Each domain repo (`users`, `orgs`, `projects`, `tasks`, `attachments`, `webhooks`) calls those generated functions and maps the rows into its existing domain struct, so the service APIs, handlers, and tests are unchanged. Other deviations remain: a single JWT access token instead of the course's refresh-token rotation, and offset pagination instead of cursors.
>
> **Phase 3, the jobs path:** the course runs background jobs on [River](https://riverqueue.com). We use a small **Postgres-backed queue** instead — a `jobs` table polled with `SELECT … FOR UPDATE SKIP LOCKED` (exactly the mechanism Chapter 26 says River uses internally), with `Enqueue` + a handler registry + a polling worker. The reason: River's current releases need Go 1.24+, and even its older go-1.22 line drags in a large dependency tree (a riverpgxv5 driver, a migration framework) that fights this project's `go 1.23` pin. The queue keeps the same surface (atomic enqueue inside a transaction, retries with backoff, a dead-letter state) with a fraction of the dependencies. The `jobs` queue is also the one repository left **hand-written pgx** rather than sqlc: its worker claims a row with `FOR UPDATE SKIP LOCKED` inside a transaction and then updates it, which is awkward to express through sqlc — so sqlc covers the CRUD repos and the queue stays explicit. Each deviation is noted in the relevant file's header, and a later pass can swap in River once the toolchain moves to Go 1.24+.

## Testing

Three layers, the standard pyramid (Ch 38–40):

- **Unit tests** — pure logic, no database, fast and deterministic. Config parsing, pagination clamping, the `handleError` sentinel→status map, the validation flatten helper, the job `Backoff` curve, `RoleRank` ordering. They run everywhere with no setup.
- **Integration tests** — repositories and services against a **real Postgres**. The showcase is `internal/tasks` `TestCrossTenantIsolation`: two orgs, two users, asserting one tenant's repo calls can't read/update/delete the other's project or task — org scoping holds at the SQL layer.
- **E2E tests** — `internal/http/e2e_test.go` drives the **real router** (`NewServer(...).Routes()`) over an `httptest.Server`: signup → login → org → project → task → list, plus a no-token 401 and a cross-tenant 403, asserting status codes and the `errorEnvelope` JSON shape.

```bash
make test                 # unit tests only; integration/e2e SKIP cleanly
TEST_DATABASE_URL=postgres://beacon:beacon@localhost:5432/beacon_test?sslmode=disable \
  make test-integration   # the full suite, including integration + e2e
make cover                # coverage profile + summary
```

**How the gating works.** Integration and e2e tests are **ENV-GATED**, not build-tagged: they live in the normal build (so `go build`/`go vet`/`go test` always compile them) and `t.Skip` when `TEST_DATABASE_URL` is unset. A shared helper, `internal/testsupport.NewTestPool`, `CREATE DATABASE`s a private, randomly-named database off the one named in `TEST_DATABASE_URL`, runs the embedded migrations against it, and drops it in cleanup — each call gets its own database rather than a shared one truncated between tests.

That used to be a shared database, truncated clean before and after each call — and it broke the moment enough integration packages existed to overlap: `go test ./...` runs different *packages* as separate OS processes **in parallel**, all pointed at the same `TEST_DATABASE_URL`. Package A's cleanup truncating `memberships` while package B was mid-insert produced exactly the foreign-key violations and deadlocks a real CI run hit — not flakiness, a guaranteed collision once the packages overlapped in time. A private database per call has no shared state across processes to collide on, so parallel `go test` stays parallel and correctness stops depending on scheduling luck.

**Deviation — no testcontainers.** The course stands Postgres up with [testcontainers](https://golang.testcontainers.org/). We don't: its dependency tree wants a newer Go toolchain and fights this project's `go 1.23` / go1.23.4 pin. The env-gated approach gives the same guarantee — real Postgres, real SQL, real org scoping — with zero extra dependencies. **CI** (`.github/workflows/ci.yml`) runs the unit tests on every push/PR (integration auto-skips), then runs the *same* `go test ./...` a second time against a `postgres:16` service container with `TEST_DATABASE_URL` set, so the integration and e2e tests run for real there. A third job `docker build`s the image to prove the Dockerfile.

## Phase-3 endpoints

```
GET    /v1/orgs/{orgID}/events                                   # SSE stream (Ch 25)
GET    /v1/orgs/{orgID}/webhooks                                 # owner/admin (Ch 24)
POST   /v1/orgs/{orgID}/webhooks
DELETE /v1/orgs/{orgID}/webhooks/{webhookID}
GET    /v1/orgs/{orgID}/projects/{p}/tasks/{t}/attachments       # list   (Ch 22)
POST   /v1/orgs/{orgID}/projects/{p}/tasks/{t}/attachments       # → presigned upload URL
GET    /v1/orgs/{orgID}/projects/{p}/tasks/{t}/attachments/{id}  # → presigned download URL
```

The attachment endpoints return `501 storage_disabled` until `S3_BUCKET` is set. Email is logged (not sent) until `SMTP_HOST` is set. Both let `make run` boot with only Postgres.

## Phase-5 endpoints

```
GET    /v1/me/preferences                        # locale + timezone, and what they render (Ch 33)
PATCH  /v1/me/preferences                        # store an IANA zone and a BCP-47 tag
GET    /v1/orgs/{orgID}/search?q=                # tasks, projects and comments (Ch 29–30)
```

On the **internal** port (`:9090` by default — never point a load balancer at it):

```
GET /metrics                                     # Prometheus (Ch 35)
GET /debug/pprof/*                               # loopback or X-Admin-Token (Ch 50)
GET /healthz
```

Any mutating request may carry an `Idempotency-Key` header (Ch 14):

```bash
curl -X POST localhost:8080/v1/orgs/$ORG/projects \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: whatever-you-like-16-chars-min' \
  -d '{"name":"Only Once"}'
```

Send it twice and the second response is the first one replayed, with
`Idempotent-Replayed: true` and exactly one row in the database.

## Operating it

| Task | Command |
|---|---|
| Prove the backups restore | `RESTORE_TARGET_URL=... ./scripts/restore_drill.sh` |
| Smoke-test a stack from outside | `./scripts/smoke.sh https://beacon-api-green.fly.dev` |
| Deploy blue-green | `./deploy/bluegreen/deploy_bluegreen.sh <image-sha>` |
| Roll back (seconds, no build) | `./deploy/bluegreen/rollback.sh` |
| Run a canary | `go run ./cmd/canary-controller --color green` |
| Check the canary gate without changing anything | `go run ./cmd/canary-controller --color green --dry-run` |
| Load test | `k6 run scripts/load/list_tasks.js` (see `scripts/load/README.md`) |
| Profile under load | `go tool pprof http://localhost:9090/debug/pprof/profile?seconds=30` |

When something is on fire, start at [`docs/runbooks/`](docs/runbooks/). When it
is out, start at [`docs/postmortems/`](docs/postmortems/).

> **What is and is not proven here.** Everything in Phases 1–5 that can run
> locally has been run: the idempotency race against real concurrent requests,
> the cache across a process restart, both search engines and the fallback
> between them, the flag cascade and the experiment split, the locale cascade,
> a real trace crossing the job queue, a real `/metrics` scrape, a real
> `pg_dump` restored into a throwaway database with the smoke queries passing —
> and failing, when pointed at an empty one. The canary gate was run against a
> real Prometheus and made to both pass and fail.
>
> The blue-green and canary **routers** (`deploy/bluegreen/worker.js`,
> `deploy/canary/worker.js`) and the deploy scripts have not been run: they need
> a Fly.io account and a Cloudflare Worker with a KV namespace. They are written
> and wired, and the controller that drives them is tested — but nobody has
> watched a real cutover here, and this README will not pretend otherwise.
