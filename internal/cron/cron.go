// Package cron runs scheduled work inside the same binary as everything else,
// on its own goroutine, with no outside daemon. Each entry does one small thing
// — here, a periodic sweep of stuck rows and a heartbeat log.
//
// Course mapping: Chapter 27 — cron jobs in the same binary, with
// robfig/cron/v3. The chapter's entries each enqueue a River job so the work
// runs in the shared worker pool; ours sweep directly (a small, real DB query)
// plus a heartbeat, which keeps the example self-contained against our
// Postgres-backed queue.
//
// NOTE: in-process cron fires once per running instance. Scaled horizontally
// the sweep would run N times every interval; the queries are idempotent so
// that's harmless here. A leader-election guard is the production fix (out of
// scope for this phase, noted as the course does).
package cron

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robfig/cron/v3"
)

// Scheduler wraps a robfig cron with Beacon's entries.
type Scheduler struct {
	c      *cron.Cron
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// New builds a scheduler and registers Beacon's periodic entries.
func New(pool *pgxpool.Pool, logger *slog.Logger) *Scheduler {
	s := &Scheduler{
		c:      cron.New(),
		pool:   pool,
		logger: logger,
	}
	// Every hour on the hour: sweep jobs stuck in 'running' (e.g. a worker died
	// mid-job) back to 'pending' so they get retried, and log a heartbeat.
	_, _ = s.c.AddFunc("@hourly", func() { s.sweep(context.Background()) })
	return s
}

// Start begins the cron goroutine. Non-blocking.
func (s *Scheduler) Start() {
	s.logger.Info("cron started")
	s.c.Start()
}

// Stop halts the scheduler and waits for any running entry to finish.
func (s *Scheduler) Stop() {
	ctx := s.c.Stop()
	<-ctx.Done()
	s.logger.Info("cron stopped")
}

// sweep requeues jobs orphaned in 'running' and logs a heartbeat with the
// current queue depth. Kept small but real.
func (s *Scheduler) sweep(ctx context.Context) {
	tag, err := s.pool.Exec(ctx,
		`UPDATE jobs SET status = 'pending' WHERE status = 'running' AND created_at < now() - interval '10 minutes'`)
	if err != nil {
		s.logger.Error("cron sweep failed", slog.Any("err", err))
		return
	}

	var pending int
	if err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM jobs WHERE status = 'pending'`).Scan(&pending); err != nil {
		s.logger.Error("cron heartbeat count failed", slog.Any("err", err))
		return
	}
	s.logger.Info("cron heartbeat",
		slog.Int64("requeued_stuck_jobs", tag.RowsAffected()),
		slog.Int("pending_jobs", pending),
	)
}
