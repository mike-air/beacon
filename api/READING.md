# How to read the Go service

You did not write this code. That is the point. This half of the repository is
your practice ground for the one skill no course teaches directly: entering a
system somebody else built, orienting in it, and changing it safely.

Read `../ARCHITECTURE.md` first — it is one page and it explains why the
repository is shaped the way it is. This file is the guided tour of the server.
`../web/READING.md` is the same tour for the client.

The structure follows the **Reading Code You Didn't Write** course
(`/docs/reading-code` on Stencil): four questions to orient, then a trace you
do yourself, because doing it is the exercise. Two more have been added — one
that leaves the process, and one that leaves the language.

---

## Question 1 — What does this thing do?

Beacon is a team task board API. Organizations own projects, projects own
tasks, tasks own comments and attachments. Users exist *outside* organizations
and reach in through a membership that carries a role (owner / admin /
member). Almost every interesting bug in multi-tenant systems lives in that
separation.

Four binaries ship from one module:

| Binary | What it is |
|---|---|
| `beacon-api` | the HTTP server, which also runs an in-process worker and cron in dev |
| `beacon-worker` | the same worker and cron alone, so the queue can scale separately |
| `beacon-spec` | emits `openapi.json` and exits — no listener, no database |
| `canary-controller` | the progressive-rollout gate (chapter 47) |

The third one is the newest and the least obvious. It exists because the
OpenAPI document is **generated from the handlers**, and `make contract` runs
it to check that the committed document still matches the code.

## Question 2 — How is it arranged?

Behaviour lives in two places: `internal/` (all of it) and `migrations/` (the
schema, embedded into the binary). Everything else is packaging.

**Start with `go doc`, not with the files.** Every package's overview lives in
its `doc.go` — what the package is for, the decision behind it, and the trap
it exists to prevent. In a single-file package the overview sits above the
`package` clause instead, which is the Go convention. Either way:

```bash
go doc ./internal/http        # the web layer, and why huma sits on chi
go doc ./internal/jobs        # the queue, and why it is not River
go doc ./internal/search      # the two engines, and the measurement
```

| Directory | What it is | Course chapters |
|---|---|---|
| `internal/config` | env vars → one typed Config; nothing else reads env | 3 |
| `internal/observability` | slog, Prometheus, OpenTelemetry | 34–36 |
| `internal/postgres` | pgx pool + migrator | 5–6 |
| `internal/db` | sqlc-generated queries — **never hand-edited** | 8 |
| `internal/http` | router, gates, operations, handlers, error envelope | 4, 11–14, 19–20, 28 |
| `internal/auth` | argon2id hashing, JWT, the typed context keys | 15–17 |
| `internal/audit` | who deleted what, written in the delete's own transaction | 10 |
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

### The web layer is the part that changed most

`internal/http` is 36 files. It has a shape, and it is worth learning before
you open any of them:

```
server.go      the chi router and the process-wide middleware
humaapi.go     the huma API on top of it: error envelope, document metadata
humamw.go      the gates: auth, org, role, rate limit, locale
gates.go       the composed sets: public, authed, orgScoped, orgAdmin
ops_*.go       operation declarations — one file per resource
<resource>.go  the handler bodies those operations call
errors.go      classify(): one domain error, one status, for both paths
notify.go      after a write: publish to SSE, enqueue the webhook job
```

chi still routes. huma sits on top and knows each operation's input and
output as Go structs — which is what lets it emit `openapi.json`. Read
`go doc ./internal/http` for the trap that shaped this layout; it is a real
one, and it does not fail to compile.

## Question 3 — Where does execution start?

`cmd/beacon-api/main.go`. Read it top to bottom once, without following
anything (chapter 4 of reading-code explains why). It is the table of
contents: config → logger → pool → migrations → services → router → serve.

Run it from the repository root:

```bash
make up      # Postgres, Redis, Meilisearch, MinIO, Prometheus
make api     # migrates on boot, serves on :8080
curl localhost:8080/healthz
```

Tests without Docker: `make test-api` (integration tests skip themselves
unless `TEST_DATABASE_URL` is set — `internal/testsupport/db.go` says why).

## Question 4 — Trace one path. This one is yours.

Do not read about it. Do it, with MAP.md open, per the course:

1. Register a user, create an org, create a project, create a task — four
   curl calls. The paths are in the `ops_*.go` files, or in `openapi.json`.
2. Pick **create task** and trace it. It starts in
   [`ops_tasks.go`](internal/http/ops_tasks.go), at the `huma.Register` call
   whose `OperationID` is `create-task` — every operation in this service is
   findable by its ID, in the code and in `openapi.json` both. The hops are:
   chi middleware → the gate set the operation names → huma decodes and validates
   `CreateTaskInput` → the operation body → `internal/tasks` →
   `internal/db` → Postgres → `notify.go`. Write every hop down.
3. Then answer these three from your own trace, not from this file:
   - Where does `org_id` get checked?
   - Who turns `pgx.ErrNoRows` into a 404?
   - Which middleware would have rejected you before the operation ran, and
     how would you know from reading the registration alone?

If you can answer those, you know this codebase better than most people know
the one they work in.

## Question 5 — Trace a path that leaves the process

The Question 4 trace stays inside one process. This one does not, and the
difference is most of what makes production systems hard to reason about.

**Trace a signup's welcome email.** It starts in an HTTP handler and finishes
in a worker, minutes later, possibly on a different machine.

1. Start the stack with tracing on: `docker compose -f deploy/docker-compose.yml up -d`, then `make api`.
2. Sign up a user.
3. Find the request in the log — it carries a `trace_id`.
4. `docker logs beacon-otel | grep <that trace id>`.

You should see the HTTP span, the database spans under it, and — arriving
seconds later — a `job send_email` span with **the same trace ID** and the
HTTP span as its parent. Then answer:

- A `context.Context` cannot be written to a database row. So how did the
  trace get from the handler to the worker? (`internal/jobs/jobs.go`, and the
  `trace_context` column added in migration `0008`.)
- What happens to that span if the worker restarts between the enqueue and
  the run? Read `extractTraceContext` and say what the trace looks like then.
- The route pattern is the span name, not the URL. Find where that is set,
  and work out why it is not set where the chapter puts it. The answer is a
  comment in `internal/http/server.go` and it is about *when* middleware runs.

## Question 6 — Trace a path that leaves the language

This one is specific to this repository, and it is the reason the two halves
live together.

**Change a field and watch TypeScript break.** Open
`internal/http/ops_tasks.go`, find `CreateTaskInput`, and rename a body field
— `title` to `name`, say. Then:

```bash
make sdk           # regenerate openapi.json and the TypeScript client
cd ../web && npm run typecheck
```

The web app now fails to compile. You will get exactly one error, in
`web/src/api/endpoints.ts` — not because the rename was small, but because
that file is the only place in the client that names a request field. That is
the boundary doing its job: the contract change lands in one file rather than
scattering through the components. Put the field back, run `make contract`,
and confirm it passes again.

Now answer the interesting part: **what would have happened before?** The
first version of this project hand-wrote `openapi.yaml`. Rename the field
there and nothing breaks — not the build, not the tests. The bug ships and a
user finds it. `openapi.README.md` records the real instance of that,
`PATCH /v1/me/preferences`, which claimed to return a body it did not return.

---

## What is deliberately missing — your work-list

These course chapters are **not built yet**. Each is a real change to an
inherited codebase — the exact practice the reading-code course ends with:

| Missing | Chapter | Size |
|---|---|---|
| API keys as a second credential type | 18 | small — **start here** |
| CSRF protection for cookie-authenticated clients | 20 | small |
| Optimistic locking (a `version` column) on tasks | — | small |
| Cursor pagination to replace offset | 13 | medium |
| Refresh-token rotation to replace the single access token | 16 | medium |
| Production docker-compose and the deploy itself | 42, 44 | medium |
| Undelete, and a purge job for rows past their retention window | 10 | medium |

Chapter 10 — soft deletes and an audit trail — used to head this list and is
now built; see the house rules below and `internal/audit`. It left two
follow-ons of its own, both listed above: nothing can be *un*-deleted, and
nothing ever actually removes a soft-deleted row, so the tables only grow.

Rule from the course: write the failing test first, keep the first change in
one file, match the conventions you find (chapter 13 of reading-code — the
last twenty merged changes are the style guide; here, the existing operations
are). One addition in this repository: if your change touches a handler's
input or output, `make contract` is part of "done".

## Four bugs this codebase actually shipped, and what each one teaches

These are not hypotheticals. Each was live, each was found by using the
running system rather than by reading it, and each is fixed now — but the
shape of the mistake is the part worth keeping. All four came from the same
project-wide event: converting the HTTP layer from chi handlers to huma
operations. That is the normal way real bugs arrive — not one careless line,
but one architectural move whose consequences were not chased all the way
down.

**1. Every single-resource GET and PATCH returned an empty body.**
`GET /projects/{id}`, `GET /tasks/{id}`, both PATCHes, `get-attachment` — all
answered `200 OK` with `Content-Length: 0`, while the write itself succeeded.
The board reported "the server did not accept the move" for moves the server
had accepted and saved.

The cause is a huma API design that punishes a Go habit. If an output struct
declares a `Status int` field, huma uses that field's RUNTIME VALUE as the
response status — unconditionally. `DefaultStatus` on the registration is
consulted only when the struct has no `Status` field at all. So a handler
returning `&TaskOutput{Body: t}` gets Go's zero value, `0`, and huma silently
declines to write a body for a status it cannot make sense of. No panic, no
log, no failing type check. The `create-*` operations all set `Status`
explicitly and were fine; the read and update operations did not.

*The lesson: a zero value that is also a valid-looking input is a trap. Go
gives you `0` for free, and `0` is not a status code.*

**2. A 403 lost its specific error code.** Reading another org's project
answered `{"code":"forbidden"}` instead of `{"code":"not_member"}`, which the
client branches on. The huma gates wrote errors with `huma.WriteErr`, which
can only derive a generic per-status code — it has no way to carry what
`classify()` already knew. The codebase's own rule is that exactly one place
maps a domain error to a status; the gates had quietly become a second.

*The lesson: when you port a layer, port its error path too. It is the half
nobody demos.*

**3. `Idempotency-Key` did nothing at all.** It was documented, in the
OpenAPI spec, faithfully sent by the generated SDK on every retry — and
ignored, because the check was still chi middleware inside a nested
`r.Group` that huma-routed requests never enter. A retried create made a
second row. This is the *same root cause* the team had already found and
fixed for auth, org and role; idempotency was simply the one nobody
re-checked.

*The lesson: when you learn a trap, grep for everything else standing in it.
Finding it once is luck; finding all of it is method.*

**4. A wrong password was unreportable.** The SDK treats any `401` as "your
session expired" and does a full document load to `/sign-in`. On the login
endpoint a 401 means "wrong password" — so the page reloaded, wiping the form
and the error before it could be read. The fix is one clause: a 401 can only
mean expiry if the request actually carried a token. Sign-in sends none.

*The lesson: the same status code means different things at different
endpoints. "Handle 401 globally" is a rule with an exception in it.*

### Why all four survived a green test suite

This is the most useful part. `internal/http`'s integration and e2e tests
skip themselves unless `TEST_DATABASE_URL` is set — and `go test ./...`
without it still prints `ok` for every package. A skip that looks like a pass
is worse than a failure, because it is quieter. Three of these four bugs sat
behind exactly that.

`make test` now says out loud what it skipped. `make test-integration` is the
one that actually proves anything. Run that one.

## Six things in here worth reading closely

Not because they are clever. Because each one is a decision with a reason,
and the reason is the part that transfers.

1. **`internal/http/gates.go`** — the comment about `append()` on a shared
   slice. Two derived middleware chains can write into the same backing array
   and silently overwrite each other's last element, which in this package
   means a route quietly losing its role check. Work out why
   `append(append(huma.Middlewares{}, authed...), …)` is not redundant.
2. **`internal/http/idempotency.go` + `internal/db/queries/idempotency.sql`** —
   one SQL statement does claim-or-find atomically, using `xmax = 0` to tell
   which happened. The unique index is the synchronisation primitive; the Go
   code just reads a boolean. Ask yourself what the two-step version (SELECT
   then INSERT) would do under two simultaneous retries.
3. **`internal/cache/withcache.go`** — the `singleflight` call. Delete it in
   your head and work out what 200 concurrent requests do to Postgres the
   instant a hot key expires.
4. **`internal/search/search.go`** — `Search` tries Meilisearch and falls back
   to Postgres on *any* error. Find every place that decision is protected
   (hint: the write path, the delete path, and what is authoritative) and work
   out what would break if a handler wrote to Meili directly.
5. **`internal/experiments/assignment.go`** — twenty lines, no I/O, no clock,
   no randomness. The null byte between the two inputs is not decoration;
   `assignment_test.go` says what it prevents.
6. **`cmd/canary-controller/main.go`** — `or vector(0)` in the error-rate
   query, and the minimum-traffic guard on the latency check. Both exist
   because the obvious version of the gate fails on healthy deploys. That is
   the usual reason odd-looking code exists.

## House rules this codebase already follows

- Only `internal/config` touches `os.Getenv`. Everything else takes the typed
  Config.
- Every error crossing the HTTP boundary becomes the one envelope:
  `{ "error": { code, message, fields, details } }`. There is exactly one
  place that maps a domain error to a status — `classify()` in
  `internal/http/errors.go` — and both the chi path and the huma path call it.
- Domain packages export sentinels (`ErrNotFound`, `ErrConflict`); driver
  errors are translated at the repository edge (`internal/pgerr`).
- `internal/db` is generated: edit `migrations/` or `internal/db/queries/*.sql`,
  run `make sqlc`, commit both. It drifts if you don't — the `jobs.trace_context`
  column added in migration `0008` was missing from the generated model for
  exactly that reason until `make sqlc` was added.
- `openapi.json` and `sdk/` are generated: change a handler, run `make sdk`,
  commit all three. `make contract` fails the build if you don't.
- A package's overview lives in `doc.go` (or above the `package` clause in a
  one-file package). One package comment per package — several is not an
  error, it just makes `go doc` print them in filename order, which buries the
  overview.
- `org_id` scoping on every tenant-owned table, in the `WHERE` clause of every
  query.
- Soft deletes on everything a user can delete — projects, tasks, webhooks.
  `deleted_at IS NULL` in every read; the delete is an `UPDATE`. Deleting a
  parent cascades explicitly, in Go, because a soft-deleted row is still
  there and the `ON DELETE CASCADE` foreign key sees nothing to cascade from.
- Every delete writes an audit entry in the SAME transaction as the delete.
  Not best-effort: an audit trail that can silently not exist is not evidence.

## What has actually been run

Read this before trusting anything above. Everything in Phases 1–5 that can
run on a laptop has been run against real infrastructure — real Postgres,
real Redis, real Meilisearch, a real Prometheus, a real OpenTelemetry
collector — and watched doing the thing the chapter says it does, including
the failure cases.

Two things have **not** been run: the blue-green and canary routers
(`deploy/bluegreen/worker.js`, `deploy/canary/worker.js`) and the deploy
scripts. They need a Fly.io account and a Cloudflare Worker with a KV
namespace. The controller that drives them was tested against a real
Prometheus and made to both pass and fail its gate, but no cutover has
happened here. Treat those two files as read-and-understand, not as verified.
