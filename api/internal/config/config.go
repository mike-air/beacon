// Package config reads environment variables into a typed struct once, at
// startup. The rest of the program reads cfg.Field instead of touching the
// environment directly. Missing-but-important values fail loudly here, on
// boot, rather than as a surprise nil somewhere deep in a request.
//
// Course mapping: Chapter 3 — Configuration.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Env             string        // "development" | "production"
	Port            string        // HTTP listen port, e.g. "8080"
	DatabaseURL     string        // postgres connection string (required)
	ShutdownTimeout time.Duration // how long to wait for in-flight requests on shutdown
	CORSOrigins     []string      // allowed browser origins

	// Auth (Ch 15–17). JWTSecret signs access tokens; it is required in
	// production but falls back to a known dev value so `make run` works out
	// of the box. JWTTTL is the access-token lifetime.
	JWTSecret string
	JWTTTL    time.Duration

	// Storage (Ch 22). S3-compatible object storage for attachments. All
	// optional: if S3Bucket is empty the storage layer is "disabled" and the
	// attachment endpoints return 501 cleanly. Endpoint is set for MinIO/R2 in
	// dev; left empty for real AWS S3.
	S3Bucket         string
	S3Region         string
	S3Endpoint       string // e.g. http://localhost:9000 for MinIO; empty = AWS
	S3AccessKey      string
	S3SecretKey      string
	S3PresignTTL     time.Duration
	S3ForcePathStyle bool // true for MinIO; AWS uses virtual-hosted style

	// Email (Ch 23). When SMTPHost is empty the sender is a LogSender that just
	// logs the message (so `make run` boots with only Postgres). EmailFrom is
	// the envelope/From address.
	SMTPHost  string
	SMTPPort  int
	SMTPUser  string
	SMTPPass  string
	EmailFrom string

	// Worker (Ch 26). PollInterval is how often the in-process worker polls the
	// jobs table for due work; WorkerConcurrency caps in-flight jobs.
	WorkerPollInterval time.Duration
	WorkerConcurrency  int

	// Cache (Ch 28). RedisURL is the shared layer; leave it empty and the cache
	// degrades to the in-process LRU only. CacheTTL bounds staleness at every
	// layer — the chapter's rule is that no entry is ever written without one.
	RedisURL string
	CacheTTL time.Duration

	// Search (Ch 29–30). Postgres FTS is always on. MeiliURL is what turns the
	// Chapter 30 engine on; leave it empty and search stays on Postgres, which
	// is a complete product, not a degraded one.
	MeiliURL   string
	MeiliKey   string
	MeiliIndex string

	// Observability (Ch 35, 36, 50). MetricsPort is the SEPARATE internal
	// listener that carries /metrics and /debug/pprof — never the public port.
	// AdminToken gates pprof for non-loopback callers. OtelEndpoint is the OTLP
	// collector; empty turns tracing off entirely. TraceSampleRatio is head
	// sampling — 0.1 keeps one trace in ten.
	MetricsPort      string
	AdminToken       string
	OtelEndpoint     string
	TraceSampleRatio float64
	Version          string

	// Backups (Ch 45). The bucket must be with a DIFFERENT provider from the
	// database — a backup in the same account as the thing it protects covers
	// exactly one failure mode and not the interesting one. Empty disables the
	// backup job. BackupAgeRecipient is an age PUBLIC key; the private half
	// never goes near the server.
	BackupBucket       string
	BackupRegion       string
	BackupEndpoint     string
	BackupAccessKey    string
	BackupSecretKey    string
	BackupAgeRecipient string
	BackupSourceURL    string // defaults to DatabaseURL

	// Rate limiting (Ch 19). Two buckets: authenticated traffic keyed by org,
	// unauthenticated auth endpoints keyed by IP. RPS is the sustained refill
	// rate; Burst is how big a one-time spike is allowed through.
	TenantRateLimitRPS   float64
	TenantRateLimitBurst int
	AuthRateLimitRPS     float64 // default 5/min, expressed as 5.0/60.0
	AuthRateLimitBurst   int
}

// devJWTSecret is the fallback signing secret used outside production so the
// service boots without configuration. It is intentionally obvious; production
// fails loudly if JWT_SECRET is unset (see Load).
const devJWTSecret = "dev-insecure-jwt-secret-change-me"

// Load reads the environment into a Config. It returns an error listing every
// required variable that is missing, so you fix them all at once.
func Load() (*Config, error) {
	cfg := &Config{
		Env:             getEnv("BEACON_ENV", "development"),
		Port:            getEnv("PORT", "8080"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		ShutdownTimeout: getDuration("SHUTDOWN_TIMEOUT", 15*time.Second),
		CORSOrigins:     splitCSV(getEnv("CORS_ORIGINS", "http://localhost:3000")),
		JWTSecret:       os.Getenv("JWT_SECRET"),
		JWTTTL:          getDuration("JWT_TTL", time.Hour),

		S3Bucket:         os.Getenv("S3_BUCKET"),
		S3Region:         getEnv("S3_REGION", "us-east-1"),
		S3Endpoint:       os.Getenv("S3_ENDPOINT"),
		S3AccessKey:      os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey:      os.Getenv("S3_SECRET_KEY"),
		S3PresignTTL:     getDuration("S3_PRESIGN_TTL", 10*time.Minute),
		S3ForcePathStyle: getBool("S3_FORCE_PATH_STYLE", true),

		SMTPHost:  os.Getenv("SMTP_HOST"),
		SMTPPort:  getInt("SMTP_PORT", 1025),
		SMTPUser:  os.Getenv("SMTP_USER"),
		SMTPPass:  os.Getenv("SMTP_PASS"),
		EmailFrom: getEnv("EMAIL_FROM", "beacon@localhost"),

		WorkerPollInterval: getDuration("WORKER_POLL_INTERVAL", time.Second),
		WorkerConcurrency:  getInt("WORKER_CONCURRENCY", 5),

		MeiliURL:   os.Getenv("MEILI_URL"),
		MeiliKey:   os.Getenv("MEILI_KEY"),
		MeiliIndex: getEnv("MEILI_INDEX", "beacon"),

		MetricsPort:      getEnv("METRICS_PORT", "9090"),
		AdminToken:       os.Getenv("ADMIN_TOKEN"),
		OtelEndpoint:     os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		TraceSampleRatio: getFloat("OTEL_TRACE_SAMPLE_RATIO", 0.1),
		Version:          getEnv("BEACON_VERSION", "dev"),

		BackupBucket:       os.Getenv("BACKUP_BUCKET"),
		BackupRegion:       getEnv("BACKUP_REGION", "us-east-1"),
		BackupEndpoint:     os.Getenv("BACKUP_ENDPOINT"),
		BackupAccessKey:    os.Getenv("BACKUP_ACCESS_KEY"),
		BackupSecretKey:    os.Getenv("BACKUP_SECRET_KEY"),
		BackupAgeRecipient: os.Getenv("BACKUP_AGE_RECIPIENT"),
		BackupSourceURL:    os.Getenv("BACKUP_SOURCE_URL"),

		RedisURL: os.Getenv("REDIS_URL"),
		CacheTTL: getDuration("CACHE_TTL", 60*time.Second),

		TenantRateLimitRPS:   getFloat("TENANT_RATE_LIMIT_RPS", 10),
		TenantRateLimitBurst: getInt("TENANT_RATE_LIMIT_BURST", 60),
		AuthRateLimitRPS:     getFloat("AUTH_RATE_LIMIT_RPS", 5.0/60.0), // 5 per minute
		AuthRateLimitBurst:   getInt("AUTH_RATE_LIMIT_BURST", 10),
	}

	var missing []string
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	// JWTSecret is mandatory in production; in development we fall back to a
	// known-insecure default so the service boots without extra setup.
	if cfg.JWTSecret == "" {
		if cfg.Env == "production" {
			missing = append(missing, "JWT_SECRET")
		} else {
			cfg.JWTSecret = devJWTSecret
		}
	}
	if cfg.BackupSourceURL == "" {
		cfg.BackupSourceURL = cfg.DatabaseURL
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %v", missing)
	}
	return cfg, nil
}

func (c *Config) IsProduction() bool { return c.Env == "production" }

// StorageEnabled reports whether object storage is configured. When false the
// attachment endpoints return 501 instead of touching a half-configured S3.
func (c *Config) StorageEnabled() bool { return c.S3Bucket != "" }

// SMTPEnabled reports whether a real SMTP server is configured. When false the
// email sender logs instead of dialing out.
func (c *Config) SMTPEnabled() bool { return c.SMTPHost != "" }

// CacheEnabled reports whether the shared Redis layer is configured. When false
// the read-through cache still works, just per-instance (Ch 28).
func (c *Config) CacheEnabled() bool { return c.RedisURL != "" }

// MeiliEnabled reports whether the Chapter 30 search engine is configured. When
// false, search runs entirely on the Chapter 29 Postgres path.
func (c *Config) MeiliEnabled() bool { return c.MeiliURL != "" }

// BackupsEnabled reports whether the Chapter 45 off-provider dump is
// configured. When false the job is not registered and the nightly cron entry
// does not fire.
func (c *Config) BackupsEnabled() bool {
	return c.BackupBucket != "" && c.BackupAgeRecipient != ""
}

func getInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func getBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	// Allow a plain number of seconds, e.g. "15".
	if n, err := strconv.Atoi(v); err == nil {
		return time.Duration(n) * time.Second
	}
	return fallback
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			part := trimSpace(s[start:i])
			if part != "" {
				out = append(out, part)
			}
			start = i + 1
		}
	}
	return out
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
