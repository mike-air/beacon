package http

import (
	"net/http"

	"beacon/internal/auth"
)

// requireAuth reads `Authorization: Bearer <jwt>`, verifies it, and puts the
// user ID in the request context. Missing or invalid → 401. Every protected
// route hangs off this.
//
// Course mapping: Chapter 16 — the auth middleware.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := bearerToken(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "missing bearer token")
			return
		}
		userID, err := auth.ParseToken(s.cfg.JWTSecret, raw)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "invalid or expired token")
			return
		}
		ctx := auth.WithUserID(r.Context(), userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func bearerToken(r *http.Request) (string, bool) {
	// Delegates to the shared parser so the chi path and the huma path can
	// never disagree about what counts as a valid Authorization header.
	return bearerFromHeader(r.Header.Get("Authorization"))
}
