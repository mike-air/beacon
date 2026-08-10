package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/trace"
)

// renameSpanToRoute sets the active span's name to the matched chi route
// pattern, e.g. "GET /v1/orgs/{orgID}/projects".
//
// It has to run after the route is matched, so it wraps next.ServeHTTP and does
// its work on the way out. Reading the pattern before the handler runs would
// give the same empty string that makes otelhttp's own formatter useless here.
//
// [glue, forced by chi: Chapter 36 shows this as a WithSpanNameFormatter option
// on otelhttp. See the note at the router in server.go for why that is not
// enough.]
func renameSpanToRoute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)

		rc := chi.RouteContext(r.Context())
		if rc == nil || rc.RoutePattern() == "" {
			return
		}
		trace.SpanFromContext(r.Context()).SetName(r.Method + " " + rc.RoutePattern())
	})
}
