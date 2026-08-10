package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"beacon/internal/observability"
)

// metricsMiddleware records every HTTP metric in one place.
//
// The label that matters is `route`: chi exposes the matched route PATTERN, so
// ten thousand requests to /v1/orgs/{orgID}/projects are one series, not ten
// thousand. Labelling with r.URL.Path instead is the single fastest way to
// take down a Prometheus server.
//
// [verbatim ch35]
func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observability.InFlightRequests.Inc()
		defer observability.InFlightRequests.Dec()

		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		// chi exposes the matched route pattern in the routing context.
		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "unmatched"
		}
		status := statusClass(ww.Status()) // "2xx", "4xx", "5xx"
		observability.HTTPRequestsTotal.
			WithLabelValues(r.Method, route, status).Inc()
		observability.HTTPRequestDuration.
			WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
	})
}

func statusClass(code int) string {
	switch {
	case code < 200:
		return "1xx"
	case code < 300:
		return "2xx"
	case code < 400:
		return "3xx"
	case code < 500:
		return "4xx"
	default:
		return "5xx"
	}
}
