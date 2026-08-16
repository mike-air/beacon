// Package http owns the web layer: the router, the middleware chain, and one
// file per group of routes. Domain packages (orgs, tasks, …) never import this
// package — the dependency only points one way.
//
// Course mapping: Chapter 4 — the HTTP server; Chapter 20 — CORS; and the
// middleware chain that grows through Part II.
package http

import (
	"context"
	"fmt"
	"github.com/danielgtaylor/huma/v2"
	"log/slog"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"

	"beacon/internal/attachments"
	"beacon/internal/cache"
	"beacon/internal/config"
	"beacon/internal/email"
	"beacon/internal/experiments"
	"beacon/internal/flags"
	"beacon/internal/jobs"
	"beacon/internal/orgs"
	"beacon/internal/projects"
	"beacon/internal/realtime"
	"beacon/internal/search"
	"beacon/internal/search/meili"
	"beacon/internal/storage"
	"beacon/internal/tasks"
	"beacon/internal/users"
	"beacon/internal/webhooks"
)

// Server bundles the dependencies every handler needs. Handlers are methods on
// *Server, so they reach the pool, logger, and the domain services without
// globals. The services are constructed once in NewServer (the wiring step).
type Server struct {
	// api holds the huma registry so `beacon-api spec` can emit the OpenAPI
	// document without starting a listener.
	api huma.API

	cfg    *config.Config
	logger *slog.Logger
	pool   *pgxpool.Pool

	users    *users.Service
	orgs     *orgs.Service
	projects *projects.Service
	tasks    *tasks.Service
	search   *search.Service

	// Ch 31–32 — the switches. flags decides reachability, experiments splits
	// the users the flag let through.
	flags       *flags.Service
	experiments *experiments.Service

	// Phase 3 — the boundaries.
	attachments *attachments.Service
	webhooks    *webhooks.Service
	realtime    *realtime.Hub
	storage     storage.Storage // nil when unconfigured → attachments answer 501
	redis       *cache.Redis    // Ch 28; nil when REDIS_URL is unset
	email       email.Sender
	jobs        jobs.Queuer

	// Worker is the in-process background worker, returned so main can run it on
	// a goroutine and drain it on shutdown. cmd/beacon-worker builds its own.
	Worker *jobs.Worker
}

// NewServer wires the repositories and services over the pool and returns the
// server. One construction step at startup; no package-level state. It builds
// the Phase-3 boundaries too (storage, email, hub, job queue + worker), each
// degrading cleanly when unconfigured.
func NewServer(cfg *config.Config, logger *slog.Logger, pool *pgxpool.Pool) (*Server, error) {
	usersRepo := users.NewRepo(pool)
	orgsRepo := orgs.NewRepo(pool)
	projectsRepo := projects.NewRepo(pool)
	tasksRepo := tasks.NewRepo(pool)
	attachmentsRepo := attachments.NewRepo(pool)
	webhooksRepo := webhooks.NewRepo(pool)

	store, err := storage.New(storage.Config{
		Bucket:         cfg.S3Bucket,
		Region:         cfg.S3Region,
		Endpoint:       cfg.S3Endpoint,
		AccessKey:      cfg.S3AccessKey,
		SecretKey:      cfg.S3SecretKey,
		PresignTTL:     cfg.S3PresignTTL,
		ForcePathStyle: cfg.S3ForcePathStyle,
	})
	if err != nil {
		return nil, fmt.Errorf("NewServer: storage: %w", err)
	}
	if store == nil {
		logger.Info("storage disabled (no S3_BUCKET); attachment endpoints return 501")
	}

	sender, err := email.New(email.SMTPConfig{
		Host: cfg.SMTPHost,
		Port: cfg.SMTPPort,
		User: cfg.SMTPUser,
		Pass: cfg.SMTPPass,
		From: cfg.EmailFrom,
	}, logger)
	if err != nil {
		return nil, fmt.Errorf("NewServer: email: %w", err)
	}
	if !cfg.SMTPEnabled() {
		logger.Info("email using log sender (no SMTP_HOST); messages are logged, not sent")
	}

	// Ch 28 — the cache stack. Redis is optional; without it the read-through
	// cache still works, just per-instance and per-process.
	rdb, err := cache.NewRedis(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("NewServer: redis: %w", err)
	}
	if !cfg.CacheEnabled() {
		logger.Info("cache: no REDIS_URL; using the in-process layer only")
	}
	membershipCache, err := cache.NewCachedReader[orgs.Membership](rdb, 4096, cfg.CacheTTL)
	if err != nil {
		return nil, fmt.Errorf("NewServer: membership cache: %w", err)
	}

	// Ch 29–30 — search. Postgres FTS always; Meilisearch in front of it when
	// MEILI_URL is set. EnsureIndex runs once here so the first query after a
	// boot meets a configured index rather than a surprise.
	searchSvc := search.NewService(pool, logger)
	if cfg.MeiliEnabled() {
		mc := meili.New(cfg.MeiliURL, cfg.MeiliKey, cfg.MeiliIndex)
		if err := mc.EnsureIndex(context.Background()); err != nil {
			// Not fatal. A Meili that will not configure itself is exactly the
			// case the Chapter 30 fallback exists for: stay on Postgres.
			logger.Warn("meili unavailable; search stays on postgres", "err", err)
		} else {
			searchSvc = searchSvc.WithMeili(mc)
			logger.Info("search: meilisearch enabled", "index", cfg.MeiliIndex)
		}
	} else {
		logger.Info("search: no MEILI_URL; using postgres full-text search")
	}

	flagSvc := flags.NewService(pool, logger)
	expSvc := experiments.NewService(pool, logger)

	jobClient := jobs.NewClient(pool)
	worker := jobs.NewWorker(pool, logger, cfg.WorkerPollInterval, cfg.WorkerConcurrency)
	worker.Register(jobs.KindSendEmail, jobs.EmailHandler(sender))
	worker.Register(jobs.KindDeliverWebhook, jobs.WebhookHandler(webhooksRepo, nil))
	if cfg.BackupsEnabled() {
		// Ch 45 — the off-provider dump. Registered only when configured, so a
		// dev machine does not schedule a backup it cannot take.
		opts := s3.Options{
			Region:       cfg.BackupRegion,
			Credentials:  credentials.NewStaticCredentialsProvider(cfg.BackupAccessKey, cfg.BackupSecretKey, ""),
			UsePathStyle: cfg.BackupEndpoint != "",
		}
		if cfg.BackupEndpoint != "" {
			opts.BaseEndpoint = aws.String(cfg.BackupEndpoint)
		}
		worker.Register(jobs.KindBackup, jobs.BackupHandler(jobs.BackupConfig{
			SourceURL:    cfg.BackupSourceURL,
			Bucket:       cfg.BackupBucket,
			AgeRecipient: cfg.BackupAgeRecipient,
		}, s3.New(opts), logger))
		logger.Info("backups enabled", "bucket", cfg.BackupBucket)
	} else {
		logger.Info("backups disabled (no BACKUP_BUCKET/BACKUP_AGE_RECIPIENT)")
	}
	worker.Register(search.KindReindex, search.ReindexHandler(searchSvc))
	worker.Register(search.KindReindexAll, search.ReindexAllHandler(searchSvc))

	return &Server{
		cfg:         cfg,
		logger:      logger,
		pool:        pool,
		users:       users.NewService(usersRepo, cfg.JWTSecret, cfg.JWTTTL),
		orgs:        orgs.NewService(orgsRepo, pool).WithCache(membershipCache),
		projects:    projects.NewService(projectsRepo),
		tasks:       tasks.NewService(tasksRepo),
		search:      searchSvc,
		flags:       flagSvc,
		experiments: expSvc,
		attachments: attachments.NewService(attachmentsRepo),
		webhooks:    webhooks.NewService(webhooksRepo),
		realtime:    realtime.NewHub(),
		storage:     store,
		redis:       rdb,
		email:       sender,
		jobs:        jobClient,
		Worker:      worker,
	}, nil
}

// Routes builds the full middleware chain and route table, and returns the
// handler to hang on an http.Server.
func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	// Ch 36 — otelhttp goes FIRST. It starts the root span, extracts an
	// incoming traceparent header, and everything below it becomes a child.
	r.Use(otelhttp.NewMiddleware("beacon-api"))
	// ...and the span gets its real name here, AFTER chi has matched the route.
	//
	// The chapter's WithSpanNameFormatter does not work on its own with chi,
	// and the reason is worth understanding: the formatter runs when the span
	// STARTS, which is before any routing has happened, so the route pattern is
	// still empty and every span falls back to the raw URL. One span name per
	// task id is the exact cardinality explosion Chapter 35 warns about,
	// arriving through Chapter 36's door. Renaming after the match fixes it.
	r.Use(renameSpanToRoute)

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(s.requestLogger)
	r.Use(metricsMiddleware)    // Ch 35
	r.Use(middleware.Recoverer) // turn a panic into a 500 instead of a dead server
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: s.cfg.CORSOrigins,
		AllowedMethods: []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		// Every header a browser client actually sends. Idempotency-Key and
		// If-None-Match were missing, which made two shipped features
		// unreachable from a browser: the preflight for a request carrying
		// either one returns 200 with no Access-Control-Allow-Origin, and the
		// browser reports it as a generic CORS block — so the failure looks
		// like a broken client rather than a header that was never allowed.
		AllowedHeaders: []string{
			"Authorization",
			"Content-Type",
			"Idempotency-Key", // Ch 14 — at-most-once writes
			"If-None-Match",   // Ch 28 — conditional GETs
		},
		// A cross-origin script can only read the CORS-safelisted response
		// headers unless they are named here. Without this, the client cannot
		// see the ETag it is meant to send back, cannot honour Retry-After on
		// a 429, and cannot quote a request id in a bug report.
		ExposedHeaders: []string{
			"ETag",
			"Retry-After",
			"X-Request-Id",
		},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Operational endpoints — outside /v1, never versioned.
	r.Get("/healthz", s.handleHealthz)
	r.Get("/readyz", s.handleReadyz)

	// The versioned API. Auth (signup/login) is public; everything else sits
	// behind requireAuth, and org-scoped routes additionally behind requireOrg.
	r.Route("/v1", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{
				"service": "beacon-api",
				"status":  "ok",
			})
		})

		// Public auth endpoints. Keyed by IP and deliberately tight (Ch 19):
		// nobody legitimately fires hundreds of logins a minute from one address.
		// signup and login are registered through huma (ops_auth.go), with
		// the IP limiter as their gate rather than as chi middleware.

		// Authenticated endpoints.
		r.Group(func(r chi.Router) {
			r.Use(s.requireAuth)
			// Ch 19 — one bucket per org, the unit customers pay for.
			r.Use(tenantRateLimit(s.cfg.TenantRateLimitRPS, s.cfg.TenantRateLimitBurst))
			// Ch 14 — at-most-once for mutating requests carrying an
			// Idempotency-Key. Non-mutating methods pass straight through.
			r.Use(s.idempotency)
			// Ch 33 — resolve the locale once, per request, for every handler.
			r.Use(s.localeMiddleware)

			// /v1/me is registered through huma (see ops_me.go) and so is
			// absent here. Conversion is route by route on purpose: the two
			// paths coexist on the same router, so each one can be moved and
			// verified without a flag day.

			// Org-scoped routes. requireOrg loads the caller's membership and
			// puts their role in context; requireRole gates owner/admin actions.
			r.Route("/orgs/{orgID}", func(r chi.Router) {
				r.Use(s.requireOrg)

				// Real-time SSE stream for the org (Ch 25).
				r.Get("/events", s.handleEvents)

				// Webhooks are registered through huma (ops_webhooks.go).

				// projects, tasks, comments and attachments are all registered
				// through huma (ops_projects.go, ops_tasks.go).
			})
		})
	})

	// ---- the huma layer -------------------------------------------------
	//
	// Built AFTER every chi middleware is mounted, because huma registers its
	// operations on this router and inherits whatever is already on it. The
	// gates below are the ones chi could not give a huma operation: they need
	// path parameters or per-operation selection, and huma ignores the nesting
	// chi would have used to apply them.
	api := newHumaAPI(r)
	s.api = api

	g := s.newGates(api)

	s.registerAuth(api, g)
	s.registerMe(api, g)
	s.registerOrgs(api, g)
	s.registerProjects(api, g)
	s.registerTasks(api, g)
	s.registerWebhooks(api, g)
	documentEventStream(api)

	return r
}

// requestLogger logs one structured line per request: method, path, status,
// duration, and the request id, so every log line can be traced to a request.
//
// Ch 36 adds trace_id. That one field is the cross-tool link the chapter says
// teams actually adopt OpenTelemetry for: a slow span in the trace UI and the
// log lines for that exact request become one search instead of a guess.
func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		attrs := []any{
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", ww.Status()),
			slog.Int("bytes", ww.BytesWritten()),
			slog.Duration("took", time.Since(start)),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		}
		if sc := trace.SpanContextFromContext(r.Context()); sc.HasTraceID() {
			attrs = append(attrs, slog.String("trace_id", sc.TraceID().String()))
		}
		s.logger.Info("request", attrs...)
	})
}
