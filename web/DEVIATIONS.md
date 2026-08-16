# Where this client differs from the course, and why

`production-frontend` builds the Beacon client. This repo is that client, built
against the Beacon that actually exists. Four places the two disagree. Each one
is a decision, not an oversight, and each is recorded here so a reader can
disagree with it.

## 1. There is no token refresh, because there is no refresh endpoint

Chapter 28 designs the refresh race: several requests get a 401 at once, one
refresh is fired, the rest queue behind it, and everybody retries with the new
token. That machinery needs somewhere to refresh *to*.

Beacon issues one JWT with a TTL (`JWT_TTL`, default 1h) and has no refresh
endpoint at all. So what survives from the chapter is the part that still
applies: **single-flight**. Twenty parallel 401s call `expireSession()` twenty
times and it acts once (`src/api/session.ts`), so the user is signed out one
time and sees one message rather than twenty.

Adding refresh tokens is on Beacon's own work-list. When they exist, ch28's
design drops into `client.ts` at the point marked by the 401 branch.

## 2. SSE over `fetch`, not `EventSource`

`EventSource` is the obvious API and it cannot set headers. Beacon's
`requireAuth` reads the bearer token from `Authorization` and nowhere else.

The usual workaround is `?access_token=<jwt>`, which writes a live credential
into every access log, proxy cache and `Referer` between here and the server.
So `src/api/sse.ts` streams the response body with `fetch` and keeps the token
in the header. The cost is that everything `EventSource` gives away has to be
written by hand: reconnect with jittered backoff, a pause while the tab is
hidden, and the wire-format parser.

## 3. Three bugs fixed in Beacon rather than worked around here

Building a browser client found three things wrong with the server. All three
are fixed in `../beacon`, because a workaround in the client would have left
the next client to find them again.

**CORS forbade two of Beacon's own features.** `AllowedHeaders` listed only
`Authorization` and `Content-Type`. Every mutation this client sends carries
`Idempotency-Key`, and conditional GETs carry `If-None-Match` — so the
preflight returned 200 with no `Access-Control-Allow-Origin` and the browser
reported a generic CORS block. Chapter 14's idempotency and chapter 28's ETags
were unreachable from any browser. `ExposedHeaders` was also empty, so a
cross-origin script could not read the `ETag` it is supposed to send back, nor
`Retry-After` on a 429.

**Five of the nine list endpoints broke the API's own list envelope.** Four
answered `{items, limit, offset}` and five answered a bare `{items}` — so a
client needed a special case per URL, and one written against the documented
shape broke on the undocumented one. Every list now goes through one
`writeList` helper, so a new endpoint gets the right shape by default rather
than by remembering.

**No OpenAPI document existed.** `npm run generate:api` needs one, so
`../beacon/openapi.yaml` was written from the handlers. It is now the contract
both sides are checked against.

## 4. The board is one flat list, because the server has no ordering endpoint

Chapter 45 describes drag-and-drop with an optimistic reorder. Beacon's `Task`
carries a float `position` and `PATCH` accepts it, so a move is expressible.
What is missing is a bulk-reorder call: rebalancing a column after many
insertions means one PATCH per card. Until that exists, moving a card writes a
midpoint position for that card alone, which is correct but degrades — the
gaps halve each time until floats run out of room.

The rebalance endpoint is the fix, and it belongs on the server. A knowledge
graph over both repos made the shape of the hole visible: `needsRebalance()`
has an edge to the client's whole task API and it resolves to nothing, because
the call that would repair the condition it detects does not exist. The
detector reaches a toast; the sibling path reaches a `PATCH`.

## 5. Two things the server documents but does not implement

Neither is a client bug, and neither can be fixed from this side.

**Optimistic locking.** Beacon's PRD glossary defines it — "the mechanism that
rejects a save if the record changed since you last read it" — and nothing
implements it. No migration adds a `version` column, `Task` carries no version
field, and no endpoint accepts `If-Match`. Writes are last-write-wins.

That is load-bearing for the board specifically. Two people dragging the same
card overwrite each other silently, and `useMoveTask` has no conflict branch
because the server offers nothing to branch on. The optimistic update is
optimistic about the network, not about concurrency.

**A per-upstream rate limit.** `docs/runbooks/upstream-429.md` used to tell an
operator to set `BEACON_STRIPE_RPS`, which no Go file reads and which gates a
payments integration Beacon does not have. That has been corrected in the
runbook, but the underlying gap is real: there is no way to throttle Beacon's
outbound calls except `WORKER_CONCURRENCY`.
