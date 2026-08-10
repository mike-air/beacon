// Command beacon-worker runs ONLY the background worker and cron scheduler — no
// HTTP server. Use it to scale async processing independently of the API, or to
// run the worker on a separate machine. The API binary (cmd/beacon-api) already
// runs an in-process worker; this is the standalone deployment shape.
//
// Course mapping: Chapter 26 — the same worker, runnable on its own; Chapter 27
// — cron started alongside it. Both share internal/jobs and internal/cron with
// the API so behaviour can't drift between them.
//
// NOTE on the jobs path: Beacon uses a Postgres-backed queue (a `jobs` table
// polled with FOR UPDATE SKIP LOCKED), not River — see internal/jobs/jobs.go's
// header for the why. The worker handlers (send_email, deliver_webhook) are
// registered identically to the API's.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"beacon/internal/config"
	"beacon/internal/cron"
	"beacon/internal/email"
	"beacon/internal/jobs"
	"beacon/internal/observability"
	"beacon/internal/postgres"
	"beacon/internal/webhooks"
	"beacon/migrations"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("beacon-worker: fatal: %v", err)
	}
}

func run() error {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := observability.NewLogger(cfg.Env)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("connecting to database")
	pool, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// The worker also migrates on boot, so it can run even if it starts before
	// the API the very first time.
	logger.Info("running migrations")
	if err := postgres.Migrate(ctx, pool, migrations.Files); err != nil {
		return err
	}

	sender, err := email.New(email.SMTPConfig{
		Host: cfg.SMTPHost, Port: cfg.SMTPPort,
		User: cfg.SMTPUser, Pass: cfg.SMTPPass, From: cfg.EmailFrom,
	}, logger)
	if err != nil {
		return err
	}
	webhooksRepo := webhooks.NewRepo(pool)

	worker := jobs.NewWorker(pool, logger, cfg.WorkerPollInterval, cfg.WorkerConcurrency)
	worker.Register(jobs.KindSendEmail, jobs.EmailHandler(sender))
	worker.Register(jobs.KindDeliverWebhook, jobs.WebhookHandler(webhooksRepo, &http.Client{}))

	scheduler := cron.New(pool, logger)
	scheduler.Start()
	defer scheduler.Stop()

	logger.Info("worker running; Ctrl-C to stop")
	worker.Run(ctx) // blocks until ctx is cancelled, then drains in-flight jobs
	logger.Info("worker stopped cleanly")
	return nil
}
