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
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"golang.org/x/time/rate"

	"beacon/internal/auth"
	"beacon/internal/db"
	"beacon/internal/i18n"
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

// humaTenantRateLimit is the per-organisation bucket — the unit customers pay
// for. It shares its limiterStore with nothing: one store per registration, so
// the buckets survive as long as the server does.
//
// It rejects rather than queues. A queued request holds a goroutine and a
// database connection open while it waits, which is how a rate limiter turns
// into the outage it was meant to prevent.
func (s *Server) humaTenantRateLimit(api huma.API, rps float64, burst int) gate {
	store := newLimiterStore(rate.Limit(rps), burst)
	return func(ctx huma.Context, next func(huma.Context)) {
		key := humaTenantKey(ctx)
		if key == "" {
			next(ctx) // not authenticated yet; a later gate will say so
			return
		}
		if retryAfter, ok := allow(store, key); !ok {
			humaRateLimited(api, ctx, retryAfter)
			return
		}
		next(ctx)
	}
}

// humaIPRateLimit guards the unauthenticated endpoints. Tighter, because
// nobody legitimately fires hundreds of logins a minute from one address.
func (s *Server) humaIPRateLimit(api huma.API, rps float64, burst int) gate {
	store := newLimiterStore(rate.Limit(rps), burst)
	return func(ctx huma.Context, next func(huma.Context)) {
		if retryAfter, ok := allow(store, "ip:"+humaClientIP(ctx)); !ok {
			humaRateLimited(api, ctx, retryAfter)
			return
		}
		next(ctx)
	}
}

// humaLocale resolves the locale cascade once per request: the user's stored
// preference, then their org's default, then Accept-Language. Every handler
// below reads the answer from context rather than re-deciding it.
func (s *Server) humaLocale() gate {
	return func(ctx huma.Context, next func(huma.Context)) {
		c := ctx.Context()
		if userID, ok := auth.UserIDFrom(c); ok {
			if uid, err := uuid.Parse(userID); err == nil {
				// The org is only known on org-scoped routes. Elsewhere the
				// LEFT JOIN finds nothing and the cascade skips a step.
				var orgID uuid.UUID
				if raw := ctx.Param("orgID"); raw != "" {
					if oid, err := uuid.Parse(raw); err == nil {
						orgID = oid
					}
				}
				if row, err := db.New(s.pool).GetUserPreferences(c, db.GetUserPreferencesParams{
					ID: uid, ID_2: orgID,
				}); err == nil {
					c = i18n.WithPrefs(c, i18n.Prefs{
						UserLocale: row.Locale,
						OrgLocale:  row.OrgDefaultLocale,
						Timezone:   row.Timezone,
					})
				}
			}
		}
		tag := i18n.LocaleFromHeader(c, ctx.Header("Accept-Language"))
		c = i18n.WithLocale(c, tag)
		// Content-Language tells caches and clients which language they got.
		// Without it the CDN layer would happily serve a German response to an
		// English reader.
		ctx.SetHeader("Content-Language", tag.String())
		next(huma.WithContext(ctx, c))
	}
}

// allow asks one bucket whether this request may proceed, and returns the
// Retry-After it should be told if not.
func allow(store *limiterStore, key string) (retryAfterSec int, ok bool) {
	lim := store.get(key)
	reservation := lim.Reserve()
	if !reservation.OK() {
		// Even waiting would not help — the burst is smaller than one request.
		return 1, false
	}
	if wait := reservation.Delay(); wait > 0 {
		reservation.Cancel()
		return int(wait.Seconds()) + 1, false
	}
	return 0, true
}

func humaRateLimited(api huma.API, ctx huma.Context, retryAfterSec int) {
	// Retry-After is the whole point of a 429. Without it a client guesses,
	// and a guessing client is how a throttle becomes a ban.
	ctx.SetHeader("Retry-After", strconv.Itoa(retryAfterSec))
	_ = huma.WriteErr(api, ctx, http.StatusTooManyRequests, "too many requests")
}

// humaTenantKey mirrors tenantKey: the org when the route has one, otherwise
// the user. Limiting by user on non-org routes stops one account exhausting
// the service before it has told us which tenant it is.
func humaTenantKey(ctx huma.Context) string {
	if orgID := ctx.Param("orgID"); orgID != "" {
		return "org:" + orgID
	}
	if userID, ok := auth.UserIDFrom(ctx.Context()); ok {
		return "user:" + userID
	}
	return ""
}

func humaClientIP(ctx huma.Context) string {
	// chi's RealIP middleware has already normalised RemoteAddr upstream;
	// this only has to drop the port.
	addr := ctx.RemoteAddr()
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}
