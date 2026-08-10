// Package flags is Chapter 31 — a switch you can flip without a deploy.
//
// A flag is a boolean with a cascade behind it: the user's own override wins,
// then their org's, then the flag's default. Two passes over the overrides,
// user first, so an org override can never shadow a user one.
//
// The rule that matters most here is what happens when something goes wrong:
// FAIL CLOSED. An unknown flag, a dead database, a malformed row — all return
// false. A flag that failed open would dump a half-tested feature on every
// customer the moment Postgres hiccuped, which is the exact opposite of what a
// flag is for.
//
// [verbatim ch31] with the auth lookup adapted: the chapter reads
// auth.UserFrom(ctx).OrgID; this repo's context carries the user ID (Chapter
// 16) and the org arrives per-request, so Enabled takes an explicit
// (userID, orgID) subject and the HTTP layer fills it in.
package flags

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"beacon/internal/db"
)

// Subject is who we are asking about. Either field may be empty — an empty
// subject just means only the default is consulted.
type Subject struct {
	UserID string
	OrgID  string
}

// Service evaluates feature flags. One instance lives in main for the
// life of the process, behind a small in-memory cache.
type Service struct {
	queries *db.Queries
	log     *slog.Logger

	// An RWMutex here and a plain Mutex in the rate limiter, for a reason:
	// reads of a flag vastly outnumber refreshes, so the readers really do run
	// concurrently. In the limiter every read also writes, so there is nothing
	// to gain.
	mu    sync.RWMutex
	cache map[string]cacheEntry
	ttl   time.Duration
}

type cacheEntry struct {
	def       db.FeatureFlag
	overrides []db.FeatureFlagOverride
	expiresAt time.Time
}

func NewService(pool *pgxpool.Pool, log *slog.Logger) *Service {
	return &Service{
		queries: db.New(pool),
		log:     log,
		cache:   make(map[string]cacheEntry),
		// Thirty seconds: short enough that an admin flipping a flag sees it
		// take effect while they are still watching, long enough that a hot
		// path doesn't hammer Postgres for a boolean.
		ttl: 30 * time.Second,
	}
}

// Enabled returns true if the named flag is on for the given subject.
//
// Evaluation order:
//  1. user-specific override
//  2. org-specific override
//  3. flag default
//  4. safe fallback (unknown flag) — return false, log a warning
func (s *Service) Enabled(ctx context.Context, name string, subj Subject) bool {
	entry, err := s.loadCached(ctx, name)
	if err != nil {
		s.log.Warn("flags: lookup failed", "flag", name, "err", err)
		return false
	}
	if entry == nil {
		s.log.Warn("flags: unknown flag", "flag", name)
		return false
	}

	if subj.UserID != "" {
		for _, o := range entry.overrides {
			if o.UserID != nil && o.UserID.String() == subj.UserID {
				return o.Value
			}
		}
	}
	if subj.OrgID != "" {
		for _, o := range entry.overrides {
			if o.OrgID != nil && o.OrgID.String() == subj.OrgID {
				return o.Value
			}
		}
	}

	return entry.def.DefaultValue
}

func (s *Service) loadCached(ctx context.Context, name string) (*cacheEntry, error) {
	s.mu.RLock()
	entry, ok := s.cache[name]
	s.mu.RUnlock()
	if ok && time.Now().Before(entry.expiresAt) {
		return &entry, nil
	}

	def, err := s.queries.GetFeatureFlag(ctx, name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Deliberately NOT cached. Negative caching here would mean a flag
			// you just inserted stays invisible for a full TTL, and the first
			// thing anyone does after adding a flag is test it.
			return nil, nil
		}
		return nil, fmt.Errorf("flags: get definition: %w", err)
	}
	overrides, err := s.queries.ListFeatureFlagOverrides(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("flags: list overrides: %w", err)
	}

	entry = cacheEntry{
		def:       db.FeatureFlag(def),
		overrides: make([]db.FeatureFlagOverride, 0, len(overrides)),
		expiresAt: time.Now().Add(s.ttl),
	}
	for _, o := range overrides {
		entry.overrides = append(entry.overrides, db.FeatureFlagOverride(o))
	}
	s.mu.Lock()
	s.cache[name] = entry
	s.mu.Unlock()
	return &entry, nil
}

// SetOrgOverride and SetUserOverride are the admin surface. They drop the
// cached entry immediately so an operator flipping a flag doesn't spend thirty
// seconds wondering whether it worked. [glue, implied by ch31's "an admin flip
// shows up fast".]
func (s *Service) SetOrgOverride(ctx context.Context, flag, orgID string, value bool) error {
	id, err := uuid.Parse(orgID)
	if err != nil {
		return fmt.Errorf("flags: parse org id: %w", err)
	}
	if err := s.queries.UpsertOrgFlagOverride(ctx, db.UpsertOrgFlagOverrideParams{
		FlagName: flag, OrgID: &id, Value: value,
	}); err != nil {
		return fmt.Errorf("flags: set org override: %w", err)
	}
	s.invalidate(flag)
	return nil
}

func (s *Service) SetUserOverride(ctx context.Context, flag, userID string, value bool) error {
	id, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("flags: parse user id: %w", err)
	}
	if err := s.queries.UpsertUserFlagOverride(ctx, db.UpsertUserFlagOverrideParams{
		FlagName: flag, UserID: &id, Value: value,
	}); err != nil {
		return fmt.Errorf("flags: set user override: %w", err)
	}
	s.invalidate(flag)
	return nil
}

func (s *Service) SetDefault(ctx context.Context, flag string, value bool) error {
	if err := s.queries.SetFeatureFlagDefault(ctx, db.SetFeatureFlagDefaultParams{
		Name: flag, DefaultValue: value,
	}); err != nil {
		return fmt.Errorf("flags: set default: %w", err)
	}
	s.invalidate(flag)
	return nil
}

func (s *Service) invalidate(flag string) {
	s.mu.Lock()
	delete(s.cache, flag)
	s.mu.Unlock()
}
