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

**`/v1/orgs` broke the API's own list envelope.** Every other list answers
`{items, limit, offset}`; that one answered a bare `{items}`. It now uses
`listResponse` like the rest, paged in the handler.

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

The rebalance endpoint is the fix, and it belongs on the server.
