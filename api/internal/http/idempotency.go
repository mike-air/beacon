// Chapter 14 — idempotency. A client that sends `Idempotency-Key: <key>` on a
// mutating request gets a promise: this runs at most once. Retry it after a
// timeout, a dropped connection, a train going into a tunnel — the server
// replays the original response instead of charging the card twice.
//
// The whole design turns on one atomic statement (ClaimIdempotencyKey). See
// internal/db/queries/idempotency.sql for the xmax trick.
//
// [verbatim ch14] with the adaptations this repo's shape forces:
//   - the chapter's `Idempotency(pool *postgres.Pool)` becomes a method on
//     *Server, because that is how every other middleware here is written and
//     it is where the *pgxpool.Pool and the logger already live;
//   - error responses go through writeError (this repo's one envelope from
//     Chapter 12) rather than the chapter's writeError(w, r, err) signature.
//
// The four cases, the SHA-256 body hash, the responseRecorder, and the SQL are
// exactly the chapter's.
package http

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"beacon/internal/auth"
	"beacon/internal/db"
)

// idempotency wraps mutating handlers. Routes that don't mutate
// (anything not POST/PUT/PATCH/DELETE) are passed through unchanged.
func (s *Server) idempotency(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !mutates(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			// Header is optional. Without it, the request runs normally
			// and the client takes on the duplicate risk themselves.
			next.ServeHTTP(w, r)
			return
		}
		if !validKey(key) {
			writeError(w, http.StatusBadRequest, "invalid_idempotency_key",
				"Idempotency-Key must be 16-128 printable ASCII characters")
			return
		}

		userID, ok := auth.UserIDFrom(r.Context())
		if !ok {
			// Unauthenticated requests don't get idempotency. The
			// auth middleware will have rejected them already; this
			// is a defensive check.
			next.ServeHTTP(w, r)
			return
		}
		uid, err := uuid.Parse(userID)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		// Read the body once into memory so we can hash it AND let the
		// handler read it. The size cap from Chapter 11 already applies.
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "could not read request body")
			return
		}
		hash := sha256.Sum256(body)

		// Try to claim the key. The INSERT ... ON CONFLICT decides which
		// of the four cases we're in.
		stored, err := db.New(s.pool).ClaimIdempotencyKey(r.Context(), db.ClaimIdempotencyKeyParams{
			UserID:      uid,
			Key:         key,
			RequestHash: hash[:],
			Method:      r.Method,
			Path:        r.URL.Path,
		})
		if err != nil {
			s.handleError(w, r, err)
			return
		}

		if !stored.Claimed {
			// Existing row. Three sub-cases.
			if !bytes.Equal(stored.RequestHash, hash[:]) || stored.Method != r.Method || stored.Path != r.URL.Path {
				writeError(w, http.StatusUnprocessableEntity, "idempotency_key_reused",
					"Idempotency-Key reused with a different request")
				return
			}
			if stored.CompletedAt == nil {
				w.Header().Set("Retry-After", "1")
				writeError(w, http.StatusConflict, "idempotency_in_flight",
					"a previous request with this Idempotency-Key is still processing")
				return
			}
			// Replay the stored response.
			w.Header().Set("Idempotent-Replayed", "true")
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(int(*stored.ResponseStatus))
			_, _ = w.Write(stored.ResponseBody)
			return
		}

		// We claimed the key — run the handler with the buffered body,
		// capture its response, then store it.
		r.Body = io.NopCloser(bytes.NewReader(body))
		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK, buf: &bytes.Buffer{}}

		next.ServeHTTP(rec, r)

		status := int32(rec.status)
		if err := db.New(s.pool).CompleteIdempotencyKey(r.Context(), db.CompleteIdempotencyKeyParams{
			UserID:         uid,
			Key:            key,
			ResponseStatus: &status,
			ResponseBody:   rec.buf.Bytes(),
		}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			// Logged but not surfaced — the client already got their
			// response. The next retry will see "still processing" or
			// a stale row depending on timing, and recover.
			s.logger.Error("idempotency complete failed", "err", err, "key", key)
		}
	})
}

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

// Flush keeps the SSE stream (Chapter 25) working under the recorder. Not in
// the chapter — it does not have to be, because the chapter never wraps a
// streaming route. [glue, forced by this repo: /events is a flushing handler.]
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
