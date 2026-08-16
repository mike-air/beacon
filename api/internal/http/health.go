package http

import (
	"context"
	"net/http"
	"time"
)

// handleHealthz is a liveness check: "is the process up?" It must not touch the
// database — a liveness probe that depends on Postgres will get the process
// killed during a brief DB blip. Always 200 if we can answer at all.
//
// Course mapping: Chapter 37 — Health checks.
func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleReadyz is a readiness check: "can this instance serve traffic right
// now?" It pings the database. If the DB is down we report 503 so the load
// balancer routes around us until we recover.
func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := s.pool.Ping(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "database unreachable")
		return
	}
	// Ch 28 — a dead shared cache should show up here as "not ready", not as
	// mysterious latency on every request that misses.
	if err := s.redis.Ping(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "cache unreachable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
