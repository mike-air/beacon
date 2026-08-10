// Command beacon-api is the Beacon web service.
//
// This is the file you read top-to-bottom to learn what a Beacon binary does.
// It grows one concern at a time across the series: today it loads config,
// connects to Postgres, migrates the schema, starts the HTTP server, runs the
// in-process background worker and cron scheduler, and shuts all of them down
// cleanly when asked — always here, always in boot order.
//
// Course mapping: Chapter 26 — the worker runs in-process on a goroutine (a
// separate cmd/beacon-worker runs it alone); Chapter 27 — the cron scheduler
// starts here too.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"beacon/internal/config"
	"beacon/internal/cron"
	beaconhttp "beacon/internal/http"
	"beacon/internal/observability"
	"beacon/internal/postgres"
	"beacon/migrations"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("beacon-api: fatal: %v", err)
	}
}

func run() error {
	// .env is a developer convenience; in production the platform injects real
	// environment variables and there is no file to load. Missing file is fine.
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := observability.NewLogger(cfg.Env)

	// One context cancelled on Ctrl-C or SIGTERM. Everything boot-related hangs
	// off it, so a shutdown signal during startup also unwinds cleanly.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Ch 36 — tracing is set up before anything else that might want to emit a
	// span. With OTEL_EXPORTER_OTLP_ENDPOINT unset this is a no-op, so the
	// service runs identically with no collector in sight.
	shutdownTracing, err := observability.SetupTracing(ctx, observability.TracingConfig{
		Endpoint:    cfg.OtelEndpoint,
		ServiceName: "beacon-api",
		Version:     cfg.Version,
		Env:         cfg.Env,
		SampleRatio: cfg.TraceSampleRatio,
	})
	if err != nil {
		return err
	}
	defer func() {
		// Its own context: the shutdown context may already be cancelled, and
		// spans that were buffered when the signal arrived still need flushing.
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTracing(flushCtx); err != nil {
			logger.Error("otel shutdown", "err", err)
		}
	}()
	if cfg.OtelEndpoint != "" {
		logger.Info("tracing enabled", "endpoint", cfg.OtelEndpoint, "sample_ratio", cfg.TraceSampleRatio)
	}

	logger.Info("connecting to database")
	pool, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	logger.Info("running migrations")
	if err := postgres.Migrate(ctx, pool, migrations.Files); err != nil {
		return err
	}

	srv, err := beaconhttp.NewServer(cfg, logger, pool)
	if err != nil {
		return err
	}
	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		// SSE streams (Ch 25) are long-lived: WriteTimeout would cut them off,
		// so it is left unset (0 = no write deadline). Per-request slowness is
		// bounded by ReadHeaderTimeout/ReadTimeout and handler context.
		IdleTimeout: 60 * time.Second,
	}

	// Ch 35/50 — the internal listener. /metrics and /debug/pprof live here, on
	// a port a load balancer is never pointed at, because both leak more than
	// you want a stranger to read.
	adminServer := &http.Server{
		Addr:              ":" + cfg.MetricsPort,
		Handler:           srv.AdminRoutes(),
		ReadHeaderTimeout: 5 * time.Second,
		// No WriteTimeout: /debug/pprof/profile deliberately takes 30 seconds.
	}

	// Background worker (Ch 26) and cron (Ch 27) run in-process on their own
	// goroutines, both hung off a context we cancel on shutdown so they drain.
	workerCtx, stopWorker := context.WithCancel(context.Background())
	var workerWG sync.WaitGroup
	workerWG.Add(1)
	go func() {
		defer workerWG.Done()
		srv.Worker.Run(workerCtx)
	}()

	scheduler := cron.New(pool, logger)
	if cfg.BackupsEnabled() {
		scheduler = scheduler.WithBackups() // Ch 45
	}
	scheduler.Start()

	// Serve in the background so main can wait on either a server error or a
	// shutdown signal.
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", httpServer.Addr, "env", cfg.Env)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()
	go func() {
		logger.Info("internal listener", "addr", adminServer.Addr, "paths", "/metrics /debug/pprof")
		if err := adminServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		stopWorker()
		scheduler.Stop()
		workerWG.Wait()
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received; draining in-flight requests")
	}

	// Graceful shutdown: stop accepting new connections, let in-flight requests
	// finish (up to the timeout), then stop the worker and cron.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	_ = adminServer.Shutdown(shutdownCtx)
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		stopWorker()
		scheduler.Stop()
		workerWG.Wait()
		return err
	}

	scheduler.Stop()
	stopWorker()
	workerWG.Wait()
	logger.Info("stopped cleanly")
	return nil
}
