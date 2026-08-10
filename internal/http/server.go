// Package http owns the web layer: the router, the middleware chain, and one
// file per group of routes. Domain packages (orgs, tasks, …) never import this
// package — the dependency only points one way.
//
// Course mapping: Chapter 4 — the HTTP server; Chapter 20 — CORS; and the
// middleware chain that grows through Part II.
package http

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"beacon/internal/attachments"
	"beacon/internal/config"
	"beacon/internal/email"
	"beacon/internal/jobs"
	"beacon/internal/orgs"
	"beacon/internal/projects"
	"beacon/internal/realtime"
	"beacon/internal/storage"
	"beacon/internal/tasks"
	"beacon/internal/users"
	"beacon/internal/webhooks"
)

// Server bundles the dependencies every handler needs. Handlers are methods on
// *Server, so they reach the pool, logger, and the domain services without
// globals. The services are constructed once in NewServer (the wiring step).
type Server struct {
	cfg    *config.Config
	logger *slog.Logger
	pool   *pgxpool.Pool

	users    *users.Service
	orgs     *orgs.Service
	projects *projects.Service
	tasks    *tasks.Service

	// Phase 3 — the boundaries.
	attachments *attachments.Service
	webhooks    *webhooks.Service
	realtime    *realtime.Hub
	storage     storage.Storage // nil when unconfigured → attachments answer 501
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

	jobClient := jobs.NewClient(pool)
	worker := jobs.NewWorker(pool, logger, cfg.WorkerPollInterval, cfg.WorkerConcurrency)
	worker.Register(jobs.KindSendEmail, jobs.EmailHandler(sender))
	worker.Register(jobs.KindDeliverWebhook, jobs.WebhookHandler(webhooksRepo, nil))

	return &Server{
		cfg:         cfg,
		logger:      logger,
		pool:        pool,
		users:       users.NewService(usersRepo, cfg.JWTSecret, cfg.JWTTTL),
		orgs:        orgs.NewService(orgsRepo, pool),
		projects:    projects.NewService(projectsRepo),
		tasks:       tasks.NewService(tasksRepo),
		attachments: attachments.NewService(attachmentsRepo),
		webhooks:    webhooks.NewService(webhooksRepo),
		realtime:    realtime.NewHub(),
		storage:     store,
		email:       sender,
		jobs:        jobClient,
		Worker:      worker,
	}, nil
}

// Routes builds the full middleware chain and route table, and returns the
// handler to hang on an http.Server.
func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(s.requestLogger)
	r.Use(middleware.Recoverer) // turn a panic into a 500 instead of a dead server
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   s.cfg.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
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

		// Public auth endpoints.
		r.Post("/auth/signup", s.handleSignup)
		r.Post("/auth/login", s.handleLogin)

		// Authenticated endpoints.
		r.Group(func(r chi.Router) {
			r.Use(s.requireAuth)

			r.Get("/me", s.handleMe)

			r.Post("/orgs", s.handleCreateOrg)
			r.Get("/orgs", s.handleListOrgs)

			// Org-scoped routes. requireOrg loads the caller's membership and
			// puts their role in context; requireRole gates owner/admin actions.
			r.Route("/orgs/{orgID}", func(r chi.Router) {
				r.Use(s.requireOrg)

				r.Get("/members", s.handleListMembers)
				r.With(s.requireRole(orgs.RoleAdmin)).Post("/members", s.handleAddMember)

				// Real-time SSE stream for the org (Ch 25).
				r.Get("/events", s.handleEvents)

				// Outgoing webhooks (Ch 24) — owner/admin only.
				r.Route("/webhooks", func(r chi.Router) {
					r.Use(s.requireRole(orgs.RoleAdmin))
					r.Get("/", s.handleListWebhooks)
					r.Post("/", s.handleRegisterWebhook)
					r.Delete("/{webhookID}", s.handleDeleteWebhook)
				})

				r.Route("/projects", func(r chi.Router) {
					r.Get("/", s.handleListProjects)
					r.Post("/", s.handleCreateProject)

					r.Route("/{projectID}", func(r chi.Router) {
						r.Get("/", s.handleGetProject)
						r.Patch("/", s.handleUpdateProject)
						r.Delete("/", s.handleDeleteProject)

						r.Route("/tasks", func(r chi.Router) {
							r.Get("/", s.handleListTasks)
							r.Post("/", s.handleCreateTask)

							r.Route("/{taskID}", func(r chi.Router) {
								r.Get("/", s.handleGetTask)
								r.Patch("/", s.handleUpdateTask)
								r.Delete("/", s.handleDeleteTask)

								r.Get("/comments", s.handleListComments)
								r.Post("/comments", s.handleCreateComment)

								// Attachments (Ch 22). Presigned S3 upload/download;
								// 501 when storage is unconfigured.
								r.Get("/attachments", s.handleListAttachments)
								r.Post("/attachments", s.handleCreateAttachment)
								r.Get("/attachments/{attachmentID}", s.handleGetAttachment)
							})
						})
					})
				})
			})
		})
	})

	return r
}

// requestLogger logs one structured line per request: method, path, status,
// duration, and the request id, so every log line can be traced to a request.
func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		s.logger.Info("request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", ww.Status()),
			slog.Int("bytes", ww.BytesWritten()),
			slog.Duration("took", time.Since(start)),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)
	})
}
