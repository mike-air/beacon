package http

// The gates, as huma middleware.
//
// These are the same checks the chi middleware performed, in the same order,
// with one structural difference: an operation now DECLARES which gates it
// wants instead of inheriting them from how deeply it was nested inside
// chi.Route calls.
//
// That change was forced rather than chosen. huma registers every operation on
// the router the API was built with, so middleware attached to a nested
// chi.Route group never runs for a huma operation — the request is routed
// straight past it. Discovering that cost one thirty-line spike and saved
// rewriting thirty handlers on a false premise.
//
// The result reads better than what it replaced. `Middlewares: orgAdmin` on
// the operation says exactly what protects it. Nesting said the same thing
// only if you scrolled up far enough to find every enclosing Use().
//
// Middleware that needs no path parameter and applies to everything —
// request id, real IP, logging, metrics, tracing, panic recovery, CORS, and
// idempotency, which selects itself on method and header — stays as ordinary
// chi middleware mounted at the root, where it still works untouched.

import (
	"errors"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"beacon/internal/auth"
	"beacon/internal/orgs"
)

// gate is one link in an operation's middleware chain.
type gate = func(huma.Context, func(huma.Context))

// humaRequireAuth verifies the bearer token and puts the user id in context.
// Everything below it may assume there is a caller.
func (s *Server) humaRequireAuth(api huma.API) gate {
	return func(ctx huma.Context, next func(huma.Context)) {
		raw, ok := bearerFromHeader(ctx.Header("Authorization"))
		if !ok {
			_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "missing bearer token")
			return
		}
		userID, err := auth.ParseToken(s.cfg.JWTSecret, raw)
		if err != nil {
			_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "invalid or expired token")
			return
		}
		next(huma.WithValue(ctx, auth.UserIDContextKey(), userID))
	}
}

// humaRequireOrg loads the caller's membership of {orgID} and puts their role
// in context. Must run after humaRequireAuth, which supplies the user id.
func (s *Server) humaRequireOrg(api huma.API) gate {
	return func(ctx huma.Context, next func(huma.Context)) {
		userID, ok := auth.UserIDFrom(ctx.Context())
		if !ok {
			_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "authentication required")
			return
		}
		orgID := ctx.Param("orgID")

		m, err := s.orgs.GetMembership(ctx.Context(), userID, orgID)
		if err != nil {
			if errors.Is(err, orgs.ErrNotMember) {
				// 403, not 404. The caller proved who they are; they simply do
				// not belong here. A 404 would be a small privacy win and a
				// large debugging cost, and membership is not a secret.
				_ = huma.WriteErr(api, ctx, http.StatusForbidden,
					"you are not a member of this organization")
				return
			}
			_ = huma.WriteErr(api, ctx, http.StatusInternalServerError, "could not load membership")
			return
		}
		next(huma.WithValue(ctx, auth.RoleContextKey(), m.Role))
	}
}

// humaRequireRole allows the request only if the caller's role ranks at or
// above min. Owner > admin > member. Runs after humaRequireOrg.
func (s *Server) humaRequireRole(api huma.API, min string) gate {
	return func(ctx huma.Context, next func(huma.Context)) {
		role, ok := auth.RoleFrom(ctx.Context())
		if !ok || orgs.RoleRank(role) < orgs.RoleRank(min) {
			_ = huma.WriteErr(api, ctx, http.StatusForbidden,
				"you don't have permission to perform this action")
			return
		}
		next(ctx)
	}
}

// bearerFromHeader is the header half of the old bearerToken, split out so the
// chi path and the huma path parse a token identically. bearerToken now calls
// it too, which is the only way to be sure they cannot diverge.
func bearerFromHeader(h string) (string, bool) {
	const prefix = "Bearer "
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", false
	}
	return strings.TrimSpace(h[len(prefix):]), true
}
