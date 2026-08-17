// Chapter 19 — rate limiting. Not a security feature first, a fairness feature
// first: one customer's runaway script must not starve everyone else. Two
// buckets, two keys.
//
//   - authenticated traffic is keyed by org, because the org is the unit
//     customers pay for;
//   - login/signup are keyed by IP, tighter, because there is no legitimate
//     reason for one address to fire hundreds of logins a minute.
//
// [verbatim ch19] with two adaptations:
//   - the chapter puts this in a separate internal/http/middleware package;
//     this repo keeps middleware in internal/http as methods/functions on the
//     server (middleware_auth.go, middleware_org.go), so it lives here;
//   - the chapter reads the org from auth.UserFrom(ctx).OrgID. This repo's JWT
//     carries only the user ID (Chapter 16) and the org arrives as a URL
//     parameter resolved by requireOrg, so tenantRateLimit keys on the {orgID}
//     route parameter and falls back to the user ID. Same fairness unit.

package http

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/time/rate"

	"beacon/internal/auth"
)

// limiterStore holds one limiter per key, plus a "last seen" timestamp
// for garbage collection. The struct is concurrency-safe via the mutex;
// every lookup, create, and prune goes through it.
//
// A plain sync.Mutex, not an RWMutex: every get() writes lastUsed, so there
// are no pure readers to optimise for.
type limiterStore struct {
	mu       sync.Mutex
	limiters map[string]*limiterEntry
	rate     rate.Limit
	burst    int
}

type limiterEntry struct {
	lim      *rate.Limiter
	lastUsed time.Time
}

func newLimiterStore(r rate.Limit, burst int) *limiterStore {
	s := &limiterStore{
		limiters: make(map[string]*limiterEntry),
		rate:     r,
		burst:    burst,
	}
	go s.gcLoop()
	return s
}

func (s *limiterStore) get(key string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.limiters[key]
	if !ok {
		e = &limiterEntry{lim: rate.NewLimiter(s.rate, s.burst)}
		s.limiters[key] = e
	}
	e.lastUsed = time.Now()
	return e.lim
}

// gcLoop wakes every 10 minutes and drops limiter entries that haven't
// been touched in an hour. Without this, every IP we ever see leaks
// memory forever.
func (s *limiterStore) gcLoop() {
	t := time.NewTicker(10 * time.Minute)
	defer t.Stop()
	for range t.C {
		cutoff := time.Now().Add(-1 * time.Hour)
		s.mu.Lock()
		for k, e := range s.limiters {
			if e.lastUsed.Before(cutoff) {
				delete(s.limiters, k)
			}
		}
		s.mu.Unlock()
	}
}

// tenantRateLimit is for authenticated endpoints. Each org gets its
// own bucket. Returns 429 with Retry-After when empty.
func tenantRateLimit(rps float64, burst int) func(http.Handler) http.Handler {
	store := newLimiterStore(rate.Limit(rps), burst)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := tenantKey(r)
			if key == "" {
				next.ServeHTTP(w, r) // not authenticated yet — let later middleware handle it
				return
			}

			lim := store.get(key)
			reservation := lim.Reserve()
			if !reservation.OK() {
				// Reservation says even waiting won't help — burst is too small.
				writeRateLimited(w, 1)
				return
			}
			wait := reservation.Delay()
			if wait > 0 {
				// Reject, don't queue. A queued request holds a goroutine and
				// a database connection open while it waits, which is how a
				// rate limiter turns into the outage it was meant to prevent.
				reservation.Cancel()
				writeRateLimited(w, int(wait.Seconds())+1)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// tenantKey picks the fairness unit: the org when the route has one, otherwise
// the user. Empty means "not authenticated", and the request passes through to
// the auth middleware that will reject it properly.
func tenantKey(r *http.Request) string {
	if orgID := chi.URLParam(r, "orgID"); orgID != "" {
		return "org:" + orgID
	}
	if userID, ok := auth.UserIDFrom(r.Context()); ok {
		return "user:" + userID
	}
	return ""
}

// ipRateLimit is for unauthenticated endpoints (login, signup, etc.).
// Tighter limits — there's no legitimate reason for one IP to fire
// hundreds of logins per minute.
func ipRateLimit(rps float64, burst int) func(http.Handler) http.Handler {
	store := newLimiterStore(rate.Limit(rps), burst)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			lim := store.get("ip:" + ip)

			reservation := lim.Reserve()
			if !reservation.OK() {
				writeRateLimited(w, 60)
				return
			}
			wait := reservation.Delay()
			if wait > 0 {
				reservation.Cancel()
				writeRateLimited(w, int(wait.Seconds())+1)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeRateLimited(w http.ResponseWriter, retryAfterSec int) {
	w.Header().Set("Retry-After", strconv.Itoa(retryAfterSec))
	writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
}

func clientIP(r *http.Request) string {
	// The RealIP middleware (Chapter 4) already normalised this. r.RemoteAddr
	// is "host:port"; we want just the host.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
