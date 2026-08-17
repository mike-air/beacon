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
// request id, real IP, logging, metrics, tracing, panic recovery, and CORS —
// stays as ordinary chi middleware mounted at the root, where it still works
// untouched. Idempotency does not: it used to live there too, on the theory
// that selecting itself on method and header made it transport-agnostic
// enough not to need converting. It was mounted inside a nested r.Group,
// which a huma-routed request never enters, so it silently did nothing for
// every mutating operation once the conversion finished. humaIdempotency
// below is the fix — see idempotency.go's header for the full story.

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
//
// Errors here go through asHumaError + writeHumaError rather than
// huma.WriteErr, so a gate's rejection carries the same `code` a handler's
// would. See writeHumaError's comment for why that distinction is load-
// bearing and not cosmetic.
func (s *Server) humaRequireAuth(api huma.API) gate {
	return func(ctx huma.Context, next func(huma.Context)) {
		raw, ok := bearerFromHeader(ctx.Header("Authorization"))
		if !ok {
			writeHumaError(ctx, s.asHumaError(ctx.Context(), auth.ErrInvalidToken))
			return
		}
		userID, err := auth.ParseToken(s.cfg.JWTSecret, raw)
		if err != nil {
			writeHumaError(ctx, s.asHumaError(ctx.Context(), auth.ErrInvalidToken))
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
			writeHumaError(ctx, s.asHumaError(ctx.Context(), auth.ErrInvalidToken))
			return
		}
		orgID := ctx.Param("orgID")

		m, err := s.orgs.GetMembership(ctx.Context(), userID, orgID)
		if err != nil {
			// classify() turns orgs.ErrNotMember into 403/not_member and
			// anything else into the 500/internal_error fallback (logged,
			// since classify reports it as unknown) — the same two outcomes
			// the hand-written version below produced, but through the one
			// place that mapping is allowed to live. 403, not 404: the caller
			// proved who they are; they simply do not belong here. A 404
			// would be a small privacy win and a large debugging cost, and
			// membership is not a secret.
			writeHumaError(ctx, s.asHumaError(ctx.Context(), err))
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
			writeHumaError(ctx, s.asHumaError(ctx.Context(), orgs.ErrForbidden))
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

// humaIdempotency is Chapter 14, ported. A client that sends
// Idempotency-Key on a mutating request gets a promise: this runs at most
// once. Retry it after a timeout, a dropped connection, a train going into a
// tunnel — the server replays the original response instead of performing
// the write twice. The claim, the four cases and the SQL (the xmax trick —
// see internal/db/queries/idempotency.sql) are identical to the chi version
// this replaced; see idempotency.go's header for why it had to move.
//
// The one real difference is mechanical. A chi http.Handler owns its
// http.ResponseWriter/*http.Request outright, so buffering the request body
// and swapping in a response-capturing writer are a field assignment and a
// wrapped ServeHTTP call. A gate only has a huma.Context, which is an
// interface over whatever adapter huma was built with — here, humachi. Two
// of its functions are the bridge: Unwrap gets the raw pair back out so the
// body can be replaced the same way, and NewContext builds a fresh
// huma.Context around a substitute ResponseWriter so `next` — huma's own
// request decoding AND the operation handler, both of which still run
// inside it — writes through the recorder without either one knowing it is
// there.
func (s *Server) humaIdempotency() gate {
	return func(ctx huma.Context, next func(huma.Context)) {
		if !mutates(ctx.Method()) {
			next(ctx)
			return
		}

		key := ctx.Header("Idempotency-Key")
		if key == "" {
			// Header is optional. Without it, the request runs normally and
			// the client takes on the duplicate risk themselves.
			next(ctx)
			return
		}
		if !validKey(key) {
			writeHumaError(ctx, &humaError{status: http.StatusBadRequest, Body: errorBody{
				Code:    "invalid_idempotency_key",
				Message: "Idempotency-Key must be 16-128 printable ASCII characters",
			}})
			return
		}

		userID, ok := auth.UserIDFrom(ctx.Context())
		if !ok {
			// Unauthenticated requests don't get idempotency. humaRequireAuth
			// will have rejected them already; this is a defensive check.
			next(ctx)
			return
		}
		uid, err := uuid.Parse(userID)
		if err != nil {
			next(ctx)
			return
		}

		r, w := humachi.Unwrap(ctx)

		// Read the body once so it can be hashed AND read again by huma's own
		// decoding and the handler. The size cap from Chapter 11 already
		// applies before this gate ever sees the request.
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeHumaError(ctx, &humaError{status: http.StatusBadRequest, Body: errorBody{
				Code: "invalid_request", Message: "could not read request body",
			}})
			return
		}
		hash := sha256.Sum256(body)

		// Try to claim the key. The INSERT ... ON CONFLICT decides which of
		// the four cases this is.
		stored, err := db.New(s.pool).ClaimIdempotencyKey(ctx.Context(), db.ClaimIdempotencyKeyParams{
			UserID:      uid,
			Key:         key,
			RequestHash: hash[:],
			Method:      r.Method,
			Path:        r.URL.Path,
		})
		if err != nil {
			writeHumaError(ctx, s.asHumaError(ctx.Context(), err))
			return
		}

		if !stored.Claimed {
			// Existing row. Three sub-cases.
			if !bytes.Equal(stored.RequestHash, hash[:]) || stored.Method != r.Method || stored.Path != r.URL.Path {
				writeHumaError(ctx, &humaError{status: http.StatusUnprocessableEntity, Body: errorBody{
					Code: "idempotency_key_reused", Message: "Idempotency-Key reused with a different request",
				}})
				return
			}
			if stored.CompletedAt == nil {
				ctx.SetHeader("Retry-After", "1")
				writeHumaError(ctx, &humaError{status: http.StatusConflict, Body: errorBody{
					Code:    "idempotency_in_flight",
					Message: "a previous request with this Idempotency-Key is still processing",
				}})
				return
			}
			// Replay the stored response, byte for byte.
			ctx.SetHeader("Idempotent-Replayed", "true")
			ctx.SetHeader("Content-Type", "application/json; charset=utf-8")
			ctx.SetStatus(int(*stored.ResponseStatus))
			_, _ = ctx.BodyWriter().Write(stored.ResponseBody)
			return
		}

		// We claimed the key. Put the buffered body back — huma's request
		// decoding reads r.Body next, inside next(ctx), and has to see the
		// same bytes that were just hashed — then run the rest of the chain
		// through a context built around a recorder, so whatever huma or the
		// handler writes is captured on its way out.
		r.Body = io.NopCloser(bytes.NewReader(body))
		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK, buf: &bytes.Buffer{}}
		next(humachi.NewContext(ctx.Operation(), r, rec))

		status := int32(rec.status)
		if err := db.New(s.pool).CompleteIdempotencyKey(ctx.Context(), db.CompleteIdempotencyKeyParams{
			UserID:         uid,
			Key:            key,
			ResponseStatus: &status,
			ResponseBody:   rec.buf.Bytes(),
		}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			// Logged but not surfaced — the client already got their
			// response. The next retry will see "still processing" or a
			// stale row depending on timing, and recover.
			s.logger.Error("idempotency complete failed", "err", err, "key", key)
		}
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
