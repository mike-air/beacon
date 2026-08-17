// Package http is Beacon's web layer: the router, the middleware chain, the
// operation declarations, and one file per group of routes. The dependency
// only points one way — domain packages (orgs, tasks, …) never import this
// one.
//
// # Two layers, and why there are two
//
// chi is the router. huma is the operation registry that sits on top of it.
// Neither replaces the other, and the split is the thing to understand first.
//
//	chi     matches a URL to a handler, and runs the process-wide middleware:
//	        request ID, recovery, CORS, real IP, timeouts, metrics, tracing.
//	huma    knows an operation's INPUT and OUTPUT as Go structs, validates a
//	        request against them, and — the reason it is here — emits
//	        openapi.json from them.
//
// The contract chain has no hand-written link:
//
//	Go struct -> api/openapi.json -> sdk/src/generated -> the web app compiles
//
// See humaapi.go for the two adjustments that keep huma from changing
// Beacon's observable behaviour (the error envelope, and the "$schema"
// property), and cmd/beacon-spec for how the document gets emitted.
//
// # The trap that shaped this package
//
// huma registers an operation on the router the API was BUILT with. So chi
// middleware installed inside a nested r.Route or r.Group never runs for a
// huma operation — the operation is not mounted inside that subtree, it only
// shares a path prefix with it. A route can therefore look guarded and be
// wide open, with nothing failing to compile.
//
// That is why per-request protection lives in huma.Middlewares instead:
// humamw.go holds the gates, gates.go composes them into named sets, and each
// operation names the set it wants. `Middlewares: g.orgAdmin` on the
// operation is readable at the point of use, which the chi version was not.
//
// # Where things are
//
//	server.go        the chi router, the process-wide middleware, boot wiring
//	humaapi.go       the huma API: error envelope, document metadata
//	humamw.go        the per-request gates (auth, org, role, rate limit, locale)
//	gates.go         the composed sets: public, authed, orgScoped, orgAdmin
//	ops_*.go         operation declarations — one file per resource
//	auth.go orgs.go projects.go tasks.go webhooks.go search.go prefs.go
//	                 the handler bodies the operations call
//	errors.go        classify(): one domain error -> one status, for both paths
//	response.go      the error envelope every client reads
//	notify.go        after a write: fan out to SSE, enqueue the webhook job
//	idempotency.go   at-most-once for mutating requests (Ch 14)
//	cache.go         Cache-Control, ETag, Vary (Ch 28)
//	admin.go         the second listener: /metrics and /debug/pprof (Ch 35, 50)
//	events.go        the SSE stream itself — not a huma operation, and says why
//
// # One request, in order
//
//	chi middleware      request ID, recovery, real IP, CORS, timeout, metrics,
//	                    span
//	huma middleware     the gate set this operation named: auth -> rate limit
//	                    -> locale -> org -> role
//	huma               decode and validate the input struct against the spec
//	ops_*.go           the operation body: authorize, call the service
//	internal/<domain>  the rules
//	internal/db        sqlc-generated SQL, org-scoped
//	notify.go          SSE publish + webhook job enqueue
//
// Course mapping: Chapter 4 — the HTTP server; Chapter 20 — CORS; and the
// middleware chain that grows through Part II.
package http
