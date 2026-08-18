# Deploying the web app to Vercel

Vercel hosts **the web app only**. The Go API cannot run there, and the reason
is architectural rather than a missing flag — see the last section.

## Why the split

`internal/realtime` is an in-memory pub/sub hub: a mutex and a map of channels
inside one process. That works because `beacon-api` is one long-lived server,
so the handler that creates a task and the handler holding your SSE stream are
in the same process and share the same map.

Every Vercel Function invocation is an isolated instance. A task created in
instance A publishes to A's map; the SSE stream in instance B is subscribed to
B's. The event is never delivered — not delayed, never — and the board silently
stops updating for everyone but the person who made the change.

Two more, less fundamental:

- `internal/http/events.go` holds the connection in a `for { select {…} }`
  until the client disconnects. Vercel Functions have a maximum duration, so
  every stream would be cut and reconnected on a timer.
- `cmd/beacon-worker` is a separate always-on binary with a ticker loop.
  Vercel has no always-on process; Cron is the substitute and its floor is one
  minute.

Put the API on something that runs a container — Fly.io, Railway, Render, or
the VPS the `deploy-vps*` workflows already target. All of them run the
existing `api/Dockerfile` unchanged, and SSE and the worker keep working. It is
still one repository and one push; only the destinations differ.

## Setting it up

1. **New Project → import the repo.**
2. **Root Directory: `web`.** The whole repo is still checked out, which
   matters: `package.json` depends on the SDK as `file:../sdk`, so the install
   needs `sdk/` to exist alongside.
3. **Environment variable:** `VITE_API_BASE` = your API's origin, e.g.
   `https://beacon-api.fly.dev`.

   The container deployment resolves the API base at *runtime* from
   `config.js`, so one image runs in any environment (see `src/api/config.ts`).
   On Vercel that indirection is unnecessary — Vercel rebuilds per environment
   anyway — so the build-time variable is the simpler correct choice. Both
   paths are already supported; nothing needs changing to use either.

4. **Edit the CSP in `vercel.json`.** `connect-src` contains the placeholder
   `https://API-ORIGIN-GOES-HERE`. Replace it with the same origin as
   `VITE_API_BASE`.

   This is the one thing that cannot be automated: `vercel.json` is read before
   the build runs, so it cannot interpolate an environment variable. Leaving
   the placeholder means the browser blocks every API call, which fails loudly
   and immediately rather than subtly — deliberately better than omitting
   `connect-src`, which would fail open and let the page talk to anywhere.

5. **CORS on the API:** set `CORS_ORIGINS` to the Vercel domain (and any
   preview domains you want to work). Without it the browser rejects the
   responses.

## What the config does

It mirrors `deploy/nginx.conf`, so the two deployments behave the same:

| | why |
|---|---|
| SPA rewrite to `/index.html` | client-side routes must not 404 on a hard refresh |
| `/assets/*` immutable, 1 year | filenames are content-hashed, so they can never go stale |
| `index.html`, `config.js` `no-store` | `index.html` names the current asset hashes and `config.js` carries the API base; a cached copy of either points a fresh deploy at the wrong thing |
| CSP, nosniff, Referrer-Policy, Permissions-Policy | same set nginx sends |

`script-src` keeps `'unsafe-inline'` for the two inline scripts `index.html`
genuinely needs — the pre-paint theme resolver and the `config.js` loader.
Removing it means a nonce, which means templating the HTML per request. The
nginx config records the same trade for the same reason.

## Preview deployments

Preview builds get the same `VITE_API_BASE` unless you set a different value
per environment. Pointing previews at production data is usually not what you
want; give the Preview environment its own API origin, and add that origin to
the CSP too.
