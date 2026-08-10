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

func getInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
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
