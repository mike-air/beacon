package http

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"beacon/internal/auth"
	"beacon/internal/orgs"
)

// requireOrg runs under routes mounted at /v1/orgs/{orgID}. It loads the
// caller's membership in that org and stashes their role in the context. Not a
// member → 403. Must run after requireAuth (it reads the user ID).
//
// Course mapping: Chapter 7 — multi-tenancy; Chapter 17 — RBAC.
func (s *Server) requireOrg(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := auth.UserIDFrom(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
			return
		}
		orgID := chi.URLParam(r, "orgID")

		m, err := s.orgs.GetMembership(r.Context(), userID, orgID)
		if err != nil {
			if errors.Is(err, orgs.ErrNotMember) {
				writeError(w, http.StatusForbidden, "not_member", "you are not a member of this organization")
				return
			}
			s.handleError(w, r, err)
			return
		}

		ctx := auth.WithRole(r.Context(), m.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requireRole returns middleware that allows the request only if the caller's
// role (set by requireOrg) is at least min. Owner > admin > member. Used to
// gate owner/admin-only actions.
func (s *Server) requireRole(min string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, ok := auth.RoleFrom(r.Context())
			if !ok || orgs.RoleRank(role) < orgs.RoleRank(min) {
				writeError(w, http.StatusForbidden, "forbidden", "you don't have permission to perform this action")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
