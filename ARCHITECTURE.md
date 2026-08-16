# Beacon: how the parts fit

Beacon is a multi-tenant task board. This repository holds the whole system —
the Go service that owns the data, the React client people actually use, and
the generated contract between them.

Read this file first. It is the map. `api/READING.md` and `web/READING.md`
are the guided tours of each half.

## The layout

```
beacon/
  api/     the Go service. Owns the database, the rules, and the contract.
  sdk/     the TypeScript client. Generated. Nobody edits it by hand.
  web/     the React app. Consumes the SDK; knows no URLs of its own.
  Makefile one entry point for everything.
```

## The one idea worth understanding

**The contract is derived, never written.**

A Go handler declares its input and output as ordinary Go structs. Those same
structs are what `huma` reads to emit `api/openapi.json`, and that document is
what generates `sdk/`. So the chain is:

```
  Go struct  ->  openapi.json  ->  sdk/src/  ->  the web app compiles or does not
     ^                                                    |
     |____________________ one edit ______________________|
```

Change a field in Go, run `make sdk`, and any part of the web app that
disagreed now fails to compile. Nothing in that chain is maintained by hand,
which is the entire point.

### Why this replaced what was here before

The first version of this project hand-wrote `openapi.yaml` by reading the
handlers. That is a document describing code rather than a document produced
by it, and the two drift the moment somebody is in a hurry. The
`production-frontend` course names the hazard exactly:

> **Spec drift** — the gap between what the spec in your repository says and
> what the running server actually sends. It produces no compile error,
> because the compiler only ever saw the spec.

Worse, the two halves lived in separate repositories, so a breaking change
could not be committed atomically and no CI could check both sides of it.

The monorepo fixes the second problem: one commit changes the handler, the
spec and the client together, or it changes none of them. `make contract`
fixes the first: it regenerates everything and fails if the result differs
from what is committed, so a handler change without a regenerated SDK is a
red build rather than a runtime surprise.

### What this deliberately is not

**Not a hand-written SDK.** An SDK is the output of a contract, not a
replacement for one. A hand-maintained client can lie about the server exactly
as a hand-maintained spec can, and it lies more convincingly, because it looks
authoritative.

**Not gRPC or Connect.** Protobuf would give a stronger guarantee and a
smaller contract. It would also replace chi's routing, turn middleware into
interceptors, and abandon the REST shape that `go-production-api` spends fifty
chapters teaching. The cost is real and the benefit here is not.

**Not a framework rewrite.** `huma` sits on top of chi. The router is still
chi, all fourteen middleware still run, and `internal/` below the HTTP layer
never learns that anything changed.

## Request path, end to end

Following one request all the way through is the fastest way to learn this
codebase. Moving a card is a good one:

```
  web/features/board/use-move-task.ts   optimistic cache update, then
  sdk/                                  typed call, generated
  api  chi middleware chain             requestID, CORS, auth, rate limit,
                                        idempotency, locale, metrics, span
       huma operation                   input struct validated from the spec
       internal/http/tasks.go           handler: authorize, call the service
       internal/tasks                   domain rules
       internal/db                      sqlc-generated SQL, org-scoped
       Postgres
       internal/http/notify.go          fan out: SSE hub + webhook job queue
  web  org-gate.tsx                     SSE event -> invalidate -> refetch
```

Every hop is a real file. `api/READING.md` walks it with line numbers.

## Boundaries that matter

**Tenancy.** `org_id` in the `WHERE` clause is the boundary, and it is
enforced twice: `requireOrg` middleware puts the caller's role in context, and
every generated query filters on the org. Comments and attachments have no
`org_id` column of their own and are scoped through their parent task in Go —
a deliberate choice, and the one place the boundary depends on application
code rather than the schema.

**The queue.** Anything slow or third-party happens in a job, never inline.
Email and webhook delivery both cross that line. A W3C trace context is
carried on the job row so one request's trace survives the boundary.

**Search.** Meilisearch when it answers, Postgres full-text when it does not.
The response says which engine served it, and the client shows that, because
a silent fallback is a silent incident.

## Where to go next

| You want to | Read |
|---|---|
| Understand the server | `api/READING.md` |
| Understand the client | `web/READING.md` |
| Know what is deliberately missing | `web/DEVIATIONS.md` |
| Run something | `make help` |
| Know why a decision was made | the comment above it — they are load-bearing here |
