// Chapter 35 — metrics. Logs tell you what happened to ONE request; metrics
// tell you what is happening to all of them. Two ingredients: a library that
// counts things in-process, and an endpoint that serialises those counts into
// text a Prometheus server scrapes.
//
// The trap the chapter spends the most time on is CARDINALITY. A metric's cost
// scales with the number of distinct label combinations — its series count. So
// labels must be bounded sets: the method, the route PATTERN (not the URL), the
// status class. Never user_id, never org_id, never a raw path. When you want to
// know about one customer, you count; you do not label.
//
// [verbatim ch35]
package observability

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsHandler returns the HTTP handler that exposes the standard
// /metrics endpoint. Prometheus scrapes this URL on its scrape interval.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// Counters, gauges, and histograms — defined once at package init.
var (
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests, by method, route, status class.",
		},
		[]string{"method", "route", "status"},
	)

	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "http_request_duration_seconds",
			Help: "End-to-end latency of HTTP requests in seconds.",
			// A histogram, not a summary: buckets add cleanly across replicas,
			// so a p95 computed over four instances means something. A
			// summary's percentiles cannot be merged and quietly lie.
			Buckets: prometheus.ExponentialBuckets(0.005, 2, 12),
			//       5ms, 10, 20, 40, 80, 160, 320, 640ms, 1.28, 2.56, 5.12, 10.24s
		},
		[]string{"method", "route"},
	)

	InFlightRequests = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "http_in_flight_requests",
		Help: "Number of HTTP requests currently being served.",
	})

	JobsCompletedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "beacon_jobs_completed_total",
			Help: "Total background jobs completed, by kind and outcome.",
		},
		[]string{"kind", "outcome"},
	)
)

func init() {
	// MustRegister panics on a duplicate metric name. That is the behaviour you
	// want: a name collision is a bug, and finding it at startup beats finding
	// it in a dashboard that has been wrong for a week.
	prometheus.MustRegister(
		HTTPRequestsTotal,
		HTTPRequestDuration,
		InFlightRequests,
		JobsCompletedTotal,
	)
}
