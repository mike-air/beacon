// The queue itself: Enqueue, the claim query, backoff and retry, and the worker
// loop that polls. The package overview — including why this is a hand-rolled
// Postgres queue and not River — is in doc.go.
//
// Course mapping: Chapter 26 — background jobs.

package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"beacon/internal/observability"
)

// Job kinds. Handlers are registered against these strings.
const (
	KindSendEmail      = "send_email"
	KindDeliverWebhook = "deliver_webhook"
)

// Queuer is the narrow enqueue surface domain/orchestration code depends on, so
// callers that only need to drop work on the queue don't pull in the worker.
type Queuer interface {
	Enqueue(ctx context.Context, kind string, payload any) error
}

// dbExecer is satisfied by both *pgxpool.Pool and pgx.Tx, enabling atomic
// enqueue (write the user row and the job in one transaction — Chapter 23's
// point) as well as plain enqueue on the pool.
type dbExecer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Client enqueues jobs onto the pool. For atomic enqueue inside an existing
// transaction, use EnqueueTx with a pgx.Tx.
type Client struct {
	pool *pgxpool.Pool
}

// NewClient returns an enqueue client over the pool.
func NewClient(pool *pgxpool.Pool) *Client { return &Client{pool: pool} }

const insertSQL = `
	INSERT INTO jobs (kind, payload, max_attempts, trace_context)
	VALUES ($1, $2, $3, $4)`

const defaultMaxAttempts = 5

// Enqueue writes a pending job to run as soon as a worker picks it up.
func (c *Client) Enqueue(ctx context.Context, kind string, payload any) error {
	return enqueue(ctx, c.pool, kind, payload)
}

// EnqueueTx enqueues inside an existing transaction so the job and the
// user-visible write commit together (atomic enqueue).
func (c *Client) EnqueueTx(ctx context.Context, tx pgx.Tx, kind string, payload any) error {
	return enqueue(ctx, tx, kind, payload)
}

func enqueue(ctx context.Context, db dbExecer, kind string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("jobs.Enqueue: marshal payload: %w", err)
	}

	// Ch 36 — carry the trace across the queue boundary. A context does not
	// survive a database row, so the propagator writes the trace into a plain
	// map of strings, the row stores it, and the worker reads it back. Without
	// this the worker's work shows up as an unrelated orphan trace, and the
	// question "why was this user's signup slow" stops at the enqueue.
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	traceCtx, err := json.Marshal(carrier)
	if err != nil {
		return fmt.Errorf("jobs.Enqueue: marshal trace context: %w", err)
	}

	if _, err := db.Exec(ctx, insertSQL, kind, raw, defaultMaxAttempts, traceCtx); err != nil {
		return fmt.Errorf("jobs.Enqueue: %w", err)
	}
	return nil
}

// Handler runs one job of a given kind. A returned error triggers a retry (with
// backoff) until max_attempts is exhausted, after which the job is parked dead.
type Handler func(ctx context.Context, payload json.RawMessage) error

// Worker polls the jobs table and runs registered handlers. It is started from
// main (in-process) and from cmd/beacon-worker (standalone). Both share this
// code so behaviour can't drift between them.
type Worker struct {
	pool        *pgxpool.Pool
	logger      *slog.Logger
	handlers    map[string]Handler
	poll        time.Duration
	concurrency int
}

// NewWorker builds a worker. poll is how often to look for due jobs; concurrency
// caps simultaneously-running jobs.
func NewWorker(pool *pgxpool.Pool, logger *slog.Logger, poll time.Duration, concurrency int) *Worker {
	if poll <= 0 {
		poll = time.Second
	}
	if concurrency <= 0 {
		concurrency = 5
	}
	return &Worker{
		pool:        pool,
		logger:      logger,
		handlers:    make(map[string]Handler),
		poll:        poll,
		concurrency: concurrency,
	}
}

// Register binds a handler to a job kind. Call before Run.
func (w *Worker) Register(kind string, h Handler) { w.handlers[kind] = h }

// Run polls until ctx is cancelled. It claims one due job at a time (a slot per
// concurrency token) and runs it on a goroutine. Returns when ctx ends and all
// in-flight jobs have drained.
func (w *Worker) Run(ctx context.Context) {
	w.logger.Info("worker started", slog.Duration("poll", w.poll), slog.Int("concurrency", w.concurrency))
	sem := make(chan struct{}, w.concurrency)
	var wg sync.WaitGroup
	ticker := time.NewTicker(w.poll)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			w.logger.Info("worker stopped")
			return
		case <-ticker.C:
			// Drain as many due jobs as we have free slots for, this tick.
			for {
				select {
				case sem <- struct{}{}:
				default:
					// All slots busy; wait for the next tick.
					goto next
				}
				claimed, err := w.claimAndRun(ctx, &wg, sem)
				if err != nil {
					w.logger.Error("worker claim failed", slog.Any("err", err))
				}
				if !claimed {
					<-sem // release the slot we took but didn't use
					goto next
				}
			}
		next:
		}
	}
}

// claimAndRun atomically claims one due pending job (FOR UPDATE SKIP LOCKED),
// marks it running, and dispatches it on a goroutine. Returns whether a job was
// claimed. The caller holds a semaphore slot which the goroutine releases.
func (w *Worker) claimAndRun(ctx context.Context, wg *sync.WaitGroup, sem chan struct{}) (bool, error) {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("jobs.claim: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	const claimSQL = `
		SELECT id::text, kind, payload, attempts, max_attempts, trace_context
		FROM jobs
		WHERE status = 'pending' AND run_at <= now()
		ORDER BY run_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT 1`
	var (
		id          string
		kind        string
		payload     json.RawMessage
		attempts    int
		maxAttempts int
		traceCtx    json.RawMessage
	)
	err = tx.QueryRow(ctx, claimSQL).Scan(&id, &kind, &payload, &attempts, &maxAttempts, &traceCtx)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("jobs.claim: select: %w", err)
	}

	if _, err := tx.Exec(ctx, `UPDATE jobs SET status = 'running' WHERE id = $1`, id); err != nil {
		return false, fmt.Errorf("jobs.claim: mark running: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("jobs.claim: commit: %w", err)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() { <-sem }()
		w.execute(ctx, id, kind, payload, attempts, maxAttempts, traceCtx)
	}()
	return true, nil
}

// execute runs the handler and records the outcome: success → done; failure with
// retries left → pending with backoff; failure with none left → dead.
func (w *Worker) execute(ctx context.Context, id, kind string, payload json.RawMessage, attempts, maxAttempts int, traceCtx json.RawMessage) {
	handler, ok := w.handlers[kind]
	if !ok {
		w.fail(ctx, id, attempts, maxAttempts, fmt.Errorf("no handler registered for kind %q", kind))
		return
	}

	// Give each job its own timeout so one stuck handler can't pin a slot.
	runCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()

	// Ch 36 — rejoin the trace the enqueuer was in, then open a consumer span
	// under it. SpanKindConsumer is what tells a trace UI to draw the worker's
	// work hanging off the request that caused it, across the gap in time.
	runCtx = extractTraceContext(runCtx, traceCtx)
	runCtx, span := otel.Tracer("beacon/jobs").Start(runCtx, "job "+kind,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(attribute.String("job.kind", kind)),
	)
	defer span.End()

	err := handler(runCtx, payload)

	// Ch 35 — one counter, two labels, both bounded sets. Job kinds are a fixed
	// list and outcome is success/failure, so this is four series, not four
	// million.
	outcome := "success"
	if err != nil {
		outcome = "failure"
	}
	observability.JobsCompletedTotal.WithLabelValues(kind, outcome).Inc()

	if err != nil {
		span.RecordError(err)
		w.logger.Warn("job failed", slog.String("id", id), slog.String("kind", kind), slog.Any("err", err))
		w.fail(ctx, id, attempts, maxAttempts, err)
		return
	}
	if _, err := w.pool.Exec(ctx,
		`UPDATE jobs SET status = 'done', attempts = attempts + 1, last_error = '' WHERE id = $1`, id,
	); err != nil {
		w.logger.Error("job mark-done failed", slog.String("id", id), slog.Any("err", err))
	}
}

// extractTraceContext rebuilds the enqueuer's trace context from the stored
// map. A missing or malformed value is not an error — the job simply starts its
// own trace, which is what happened before Chapter 36 for every job in the
// table.
func extractTraceContext(ctx context.Context, raw json.RawMessage) context.Context {
	if len(raw) == 0 {
		return ctx
	}
	carrier := propagation.MapCarrier{}
	if err := json.Unmarshal(raw, &carrier); err != nil {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

// fail records a failed attempt: reschedule with backoff if tries remain, else
// park the job as dead (the queue's dead-letter state).
func (w *Worker) fail(ctx context.Context, id string, attempts, maxAttempts int, cause error) {
	next := attempts + 1
	if next >= maxAttempts {
		if _, err := w.pool.Exec(ctx,
			`UPDATE jobs SET status = 'dead', attempts = $2, last_error = $3 WHERE id = $1`,
			id, next, cause.Error(),
		); err != nil {
			w.logger.Error("job mark-dead failed", slog.String("id", id), slog.Any("err", err))
		}
		return
	}
	delay := Backoff(next)
	if _, err := w.pool.Exec(ctx,
		`UPDATE jobs SET status = 'pending', attempts = $2, last_error = $3, run_at = now() + $4 WHERE id = $1`,
		id, next, cause.Error(), delay,
	); err != nil {
		w.logger.Error("job reschedule failed", slog.String("id", id), slog.Any("err", err))
	}
}

// Backoff is exponential with a real 5-minute ceiling: 2^attempt seconds,
// capped at 5 minutes. Attempt 1 → 2s, 2 → 4s, … 8 → 256s, 9 and beyond → 300s.
// The shift is clamped first so a large attempt count can't overflow the shift.
func Backoff(attempt int) time.Duration {
	const ceiling = 5 * time.Minute
	if attempt > 9 {
		attempt = 9 // 2^9 = 512s already exceeds the ceiling; clamp to avoid overflow
	}
	d := time.Duration(1<<attempt) * time.Second
	if d > ceiling {
		d = ceiling
	}
	return d
}
