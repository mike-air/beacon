package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// SSE endpoint, GET /v1/orgs/{orgID}/events. One long-lived HTTP connection per
// browser tab; the handler subscribes to the org's events on the hub and writes
// each as an SSE frame, with periodic keep-alive comments, until the client's
// request context is cancelled.
//
// Course mapping: Chapter 25 — real-time updates with SSE. The hub
// (internal/realtime) is transport-free; this handler owns the SSE wire format,
// the flush-per-event, and the heartbeat.

const sseKeepAlive = 25 * time.Second

// handleEvents opens the SSE stream for the caller's org.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming_unsupported", "server cannot stream responses")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable proxy buffering
	w.WriteHeader(http.StatusOK)

	// Tell the client our heartbeat interval and flush headers immediately.
	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	sub := s.realtime.Subscribe(orgID)
	defer sub.Unsubscribe()

	keepAlive := time.NewTicker(sseKeepAlive)
	defer keepAlive.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			// Client disconnected (tab closed) or request cancelled.
			return
		case ev := <-sub.C():
			data, err := json.Marshal(ev.Data)
			if err != nil {
				continue
			}
			// One SSE frame: an event name and a data line, terminated by a
			// blank line. Flush so the byte hits the wire now, not at buffer
			// fill.
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, data)
			flusher.Flush()
		case <-keepAlive.C:
			// A comment line keeps the connection (and any intermediary idle
			// timeout) alive without emitting an event.
			fmt.Fprintf(w, ": keep-alive\n\n")
			flusher.Flush()
		}
	}
}
