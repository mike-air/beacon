// Chapter 14 — idempotency. A client that sends `Idempotency-Key: <key>` on a
// mutating request gets a promise: this runs at most once. Retry it after a
// timeout, a dropped connection, a train going into a tunnel — the server
// replays the original response instead of charging the card twice.
//
// The whole design turns on one atomic statement (ClaimIdempotencyKey). See
// internal/db/queries/idempotency.sql for the xmax trick.
//
// The gate itself — the claim, the four cases, the response capture — lives in
// humamw.go as humaIdempotency, not here. It used to be here, as chi
// middleware mounted on a nested r.Group. That was silently wrong the moment
// every mutating route finished moving to huma: huma registers an operation on
// the router the API was built with, so a request to it never enters that
// group, never reaches this file's `next.ServeHTTP`, and Idempotency-Key
// stopped doing anything — a retried write created a second row instead of
// replaying the first response, with no error, no log line, nothing to notice
// short of a duplicate showing up somewhere later. TestE2EIdempotency in
// e2e_test.go demonstrates it and guards against it recurring.
//
// What stays here is the two small, transport-agnostic pieces humaIdempotency
// still uses: the method check and the key-format check are pure functions
// with nothing huma- or chi-specific about them, and responseRecorder taps
// http.ResponseWriter, which is what a huma.Context wraps under the specific
// *http.ResponseWriter/*http.Request pair humachi.Unwrap hands back — huma
// never replaces that pair, it just wraps it, so a plain http.ResponseWriter
// recorder still works.
package http

import (
	"bytes"
	"net/http"
)

// responseRecorder taps the status and the bytes on their way out, so the
// stored response is byte-for-byte what the client got the first time.
type responseRecorder struct {
	http.ResponseWriter
	status int
	buf    *bytes.Buffer
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(p []byte) (int, error) {
	r.buf.Write(p)
	return r.ResponseWriter.Write(p)
}

// Flush keeps a streaming response working under the recorder, in case a
// future mutating route ever needs one. Beacon's one stream, /events, is a
// GET, so mutates() already excludes it and no request reaches this today —
// kept for the same reason a seatbelt is fastened on a car that has not
// crashed yet. [glue, not in the course chapter, which never wraps a
// streaming route.]
func (r *responseRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func mutates(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

func validKey(k string) bool {
	if len(k) < 16 || len(k) > 128 {
		return false
	}
	for i := 0; i < len(k); i++ {
		c := k[i]
		if c < 0x21 || c > 0x7e {
			return false
		}
	}
	return true
}
