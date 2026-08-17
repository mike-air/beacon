// The internal port. Chapters 35 and 50 both end at the same place: some
// endpoints must exist and must not be on the public internet.
//
//   - /metrics (Ch 35) is expensive to serve and leaks error counts, queue
//     depths and every route name you have.
//   - /debug/pprof (Ch 50) leaks goroutine stacks, and /debug/pprof/profile
//     will happily burn 30 seconds of CPU for anyone who asks.
//
// So they go on a second listener, bound separately, that a load balancer never
// points at. The chapter's phrase for it is worth keeping: "a separate internal
// port". Defence in depth on top of that: pprof additionally requires a token
// unless the caller is on loopback.

package http

import (
	"crypto/subtle"
	"net"
	"net/http"
	"net/http/pprof"

	"github.com/go-chi/chi/v5"

	"beacon/internal/observability"
)

// AdminRoutes builds the internal-only handler: metrics, pprof, and a liveness
// check so the port itself can be probed.
func (s *Server) AdminRoutes() http.Handler {
	r := chi.NewRouter()

	r.Get("/healthz", s.handleHealthz)

	// Ch 35 — the scrape target.
	r.Handle("/metrics", observability.MetricsHandler())

	// Ch 50 — profiling.
	s.registerDebugRoutes(r)

	return r
}

// registerDebugRoutes mounts the pprof endpoints under /debug/pprof.
// Never expose pprof to the open internet: it leaks goroutine state and stacks,
// and /profile costs you a CPU for as long as the caller asks for.
//
// [verbatim ch50's RegisterDebugRoutes, with the chapter's named-but-unwritten
// requireInternalIPOrAdminToken filled in below.]
func (s *Server) registerDebugRoutes(r chi.Router) {
	r.Route("/debug/pprof", func(r chi.Router) {
		r.Use(s.requireInternalIPOrAdminToken)
		r.Get("/", pprof.Index)
		r.Get("/cmdline", pprof.Cmdline)
		r.Get("/profile", pprof.Profile)
		r.Get("/symbol", pprof.Symbol)
		r.Get("/trace", pprof.Trace)
		r.Get("/{name}", func(w http.ResponseWriter, req *http.Request) {
			pprof.Handler(chi.URLParam(req, "name")).ServeHTTP(w, req)
		})
	})
}

// requireInternalIPOrAdminToken allows loopback callers (you, on the box, with
// an SSH tunnel) and anyone presenting the admin token. Everyone else gets 404
// — not 403, because a 403 confirms the endpoint exists.
//
// subtle.ConstantTimeCompare, not ==, so the comparison does not leak the
// token one byte at a time through its timing (Chapter 15's habit, applied
// somewhere it is easy to forget).
func (s *Server) requireInternalIPOrAdminToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isLoopback(r.RemoteAddr) {
			next.ServeHTTP(w, r)
			return
		}
		token := s.cfg.AdminToken
		presented := r.Header.Get("X-Admin-Token")
		if token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(presented)) == 1 {
			next.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})
}

func isLoopback(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
