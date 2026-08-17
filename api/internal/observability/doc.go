// Package observability holds the three ways this service explains itself.
// They answer different questions and none of them substitutes for another.
//
//	logger.go   logs     what happened to ONE request        Chapter 34
//	metrics.go  metrics  what is happening to all of them    Chapter 35
//	trace.go    traces   where the time went, across process Chapter 36
//
// # The rule that keeps metrics affordable
//
// A metric's cost scales with the number of distinct label combinations — its
// series count — so labels must be bounded sets: the method, the route
// PATTERN (never the URL), the status class. Never user_id, never org_id,
// never a raw path. When you want to know about one customer you count, you
// do not label. This is the failure that takes a monitoring bill from tens of
// dollars to thousands without a single alert firing.
//
// # What tracing is actually for
//
// Three nouns and that is the model: a span is one unit of work with a start,
// an end and attributes; a trace is a tree of spans sharing a trace ID; the
// context.Context carries the currently-active span.
//
// The payoff teams adopt OTel for is not the flame graph. It is the trace_id
// on every log line, so a slow request on a dashboard becomes the exact log
// lines for that request in one click. The flame graph is a bonus.
//
// The span name is the route pattern, not the URL — same reason as the metric
// label, and it is set where the pattern is known rather than where the
// chapter puts it. internal/http/server.go says why, and the answer is about
// when middleware runs relative to routing.
package observability
