// Chapter 32 — the service around Assign.
//
// Two things happen when a request asks for a variant, and only one of them is
// on the critical path. The hash answers instantly. The assignment row is
// written in the background, because it is the audit trail, not the answer —
// nobody's page load should wait on a row that exists to answer a question
// somebody will ask next month.
//
// [verbatim ch32] with this repo's ttlcache written out (the chapter names the
// field and leaves the type implied) and the sqlc parameter names.
package experiments

import (
	"context"
	"encoding/json"
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

type Experiment struct {
	Key      string
	Status   string
	Variants []Variant
}

type Service struct {
	queries *db.Queries
	log     *slog.Logger
	cache   *ttlcache[string, Experiment]
}

func NewService(pool *pgxpool.Pool, log *slog.Logger) *Service {
	return &Service{
		queries: db.New(pool),
		log:     log,
		cache:   newTTLCache[string, Experiment](30 * time.Second),
	}
}

// VariantFor returns the variant name this user sees for this experiment.
// Returns "" (empty string) if the experiment isn't running or doesn't
// exist — callers treat that as "fall back to control behaviour."
func (s *Service) VariantFor(ctx context.Context, experimentKey, userID string) string {
	exp, ok := s.lookup(ctx, experimentKey)
	if !ok || exp.Status != "running" {
		return ""
	}

	variant := Assign(userID, experimentKey, exp.Variants)

	// Best-effort: record the assignment for later analysis. We do this
	// async because the assignment must be fast — the hash already
	// told us the answer; the DB insert is for the audit trail only.
	go s.recordOnce(experimentKey, userID, variant)

	return variant
}

// recordOnce inserts the assignment row if one doesn't already exist.
// Runs in a background goroutine; failures log and move on.
func (s *Service) recordOnce(experimentKey, userID, variant string) {
	// Its own context, not the request's: the request may well be finished and
	// its context cancelled before this goroutine gets scheduled.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	uid, err := uuid.Parse(userID)
	if err != nil {
		return
	}
	err = s.queries.InsertAssignmentIfAbsent(ctx, db.InsertAssignmentIfAbsentParams{
		ExperimentKey: experimentKey,
		UserID:        uid,
		Variant:       variant,
	})
	if err != nil {
		s.log.Warn("experiments: record assignment failed",
			"exp", experimentKey, "user", userID, "err", err)
	}
}

func (s *Service) lookup(ctx context.Context, key string) (Experiment, bool) {
	if e, ok := s.cache.get(key); ok {
		return e, true
	}
	row, err := s.queries.GetExperiment(ctx, key)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			s.log.Warn("experiments: lookup failed", "exp", key, "err", err)
		}
		return Experiment{}, false
	}
	var variants []Variant
	if err := json.Unmarshal(row.Variants, &variants); err != nil {
		s.log.Warn("experiments: bad variants json", "exp", key, "err", err)
		return Experiment{}, false
	}
	exp := Experiment{Key: row.Key, Status: row.Status, Variants: variants}
	s.cache.set(key, exp)
	return exp, true
}

// SetStatus starts or stops an experiment and drops the cached copy.
func (s *Service) SetStatus(ctx context.Context, key, status string) error {
	if status != "draft" && status != "running" && status != "stopped" {
		return fmt.Errorf("experiments: unknown status %q", status)
	}
	if err := s.queries.SetExperimentStatus(ctx, db.SetExperimentStatusParams{
		Key: key, Status: status,
	}); err != nil {
		return fmt.Errorf("experiments: set status: %w", err)
	}
	s.cache.del(key)
	return nil
}

// Split reports how many users landed in each variant — the first thing to look
// at before believing any result the experiment produces.
func (s *Service) Split(ctx context.Context, key string) (map[string]int64, error) {
	rows, err := s.queries.CountAssignmentsByVariant(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("experiments: split: %w", err)
	}
	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		out[r.Variant] = r.N
	}
	return out, nil
}

// ttlcache is the small expiring map the chapter's Service field implies.
// [glue.]
type ttlcache[K comparable, V any] struct {
	mu   sync.RWMutex
	m    map[K]ttlEntry[V]
	ttl  time.Duration
	zero V
}

type ttlEntry[V any] struct {
	v   V
	exp time.Time
}

func newTTLCache[K comparable, V any](ttl time.Duration) *ttlcache[K, V] {
	return &ttlcache[K, V]{m: make(map[K]ttlEntry[V]), ttl: ttl}
}

func (c *ttlcache[K, V]) get(k K) (V, bool) {
	c.mu.RLock()
	e, ok := c.m[k]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.exp) {
		return c.zero, false
	}
	return e.v, true
}

func (c *ttlcache[K, V]) set(k K, v V) {
	c.mu.Lock()
	c.m[k] = ttlEntry[V]{v: v, exp: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

func (c *ttlcache[K, V]) del(k K) {
	c.mu.Lock()
	delete(c.m, k)
	c.mu.Unlock()
}
