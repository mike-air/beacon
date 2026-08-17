# Beacon

A multi-tenant task board. A Go API, a React client, and a generated contract
between them, in one repository.

This is a study codebase: it is built to be **read**, not just run. Almost
every non-obvious line has a comment saying why it is that way, and the four
real bugs it has shipped are written up rather than quietly patched.

---

## Start here

Read these three, in this order. They are the whole map.

| # | Read | Time | What you get |
|---|---|---|---|
| 1 | [ARCHITECTURE.md](ARCHITECTURE.md) | 5 min | Why the repo is shaped this way. One page. |
| 2 | [api/READING.md](api/READING.md) | 30 min | Guided tour of the server, with traces to run yourself. |
| 3 | [web/READING.md](web/READING.md) | 20 min | Same, for the client. |

Then pick any package and run `go doc ./internal/<name>` — every one opens
with what it is for, the decision behind it, and the trap it exists to
prevent.

**If you only read one section**, make it *"Four bugs this codebase actually
shipped"* in [api/READING.md](api/READING.md). Each was live, each was found
by using the running system rather than reading it, and each teaches a
different way a correct-looking change goes wrong.

## Run it

Needs Docker, Go 1.25 and Node 24. Two terminals:

```bash
make up && make api     # Postgres, Redis, Meilisearch, MinIO — then :8080
```

```bash
make web                # :5180
```

Then `make help` shows everything else.

## The one idea worth understanding

**The contract is derived, never written.**

```
Go struct  ->  api/openapi.json  ->  sdk/src/generated  ->  the web app compiles or does not
```

A handler declares its input and output as ordinary Go structs. Those emit
the OpenAPI document, which generates the TypeScript client. Nothing in that
chain is maintained by hand, so nothing in it can drift.

Try it — this is the fastest way to feel what the repo is for:

```bash
# rename a field on CreateTaskInput in api/internal/http/ops_tasks.go, then:
make sdk && cd web && npm run typecheck
```

The web app fails to compile, in one file, naming the exact field. Put it
back and `make contract` goes green again.

## Layout

```
api/     Go service. Owns the database, the rules, and the contract.
sdk/     TypeScript client. src/generated/ is generated; the rest is behaviour.
web/     React app. Consumes the SDK; knows no URLs of its own.
```

## Checks

```bash
make ci                 # everything, in the order CI runs it
make test-integration   # the one that actually proves anything — needs Docker
make visual             # screenshot diff, in a container so pixels match CI
```

One warning worth internalising: plain `make test` **skips** every
integration and e2e test unless `TEST_DATABASE_URL` is set, and still prints
`ok`. Three of this project's four shipped bugs hid behind exactly that. The
target now says so out loud, but prefer `make test-integration`.

## Status

Everything described in the docs is built and verified: 13/13 browser e2e
against a real server, the full Go suite including integration tests, a
200KB gzipped entry-bundle budget enforced at build time, contrast checked on
46 colour pairs across both themes, and container images for both halves.

Not done, and listed as your work-list in
[api/READING.md](api/READING.md#what-is-deliberately-missing--your-work-list):
soft deletes and an audit trail, API keys, CSRF, cursor pagination,
refresh-token rotation, and the real deploy. Each is a genuine change to an
inherited codebase, which is the point.
