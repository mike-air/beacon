// Unit tests for environment parsing — pure logic, no database.
//
// Course mapping: Chapter 38 — unit tests. These exercise Load's required-field
// handling, the dev-vs-prod JWT fallback, and the duration/int/bool/CSV helpers,
// all without touching Postgres.
package config

import (
	"testing"
	"time"
)

// setEnv sets the named environment variables for one test and restores the
// previous state on cleanup, so tests don't leak into one another.
func setEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	for k, v := range kv {
		t.Setenv(k, v)
	}
}

func TestLoadDevDefaults(t *testing.T) {
	// A minimal valid environment: DATABASE_URL set, nothing else. In
	// development the JWT secret falls back to the known-insecure default.
	clearKnownEnv(t)
	setEnv(t, map[string]string{"DATABASE_URL": "postgres://x"})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Env != "development" {
		t.Errorf("Env = %q, want development", cfg.Env)
	}
	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want 8080", cfg.Port)
	}
	if cfg.JWTSecret != devJWTSecret {
		t.Errorf("JWTSecret = %q, want dev fallback", cfg.JWTSecret)
	}
	if cfg.ShutdownTimeout != 15*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 15s", cfg.ShutdownTimeout)
	}
	if cfg.IsProduction() {
		t.Error("IsProduction() = true in development")
	}
}

func TestLoadMissingDatabaseURL(t *testing.T) {
	clearKnownEnv(t)
	if _, err := Load(); err == nil {
		t.Fatal("Load with no DATABASE_URL: want error, got nil")
	}
}

func TestLoadProductionRequiresJWTSecret(t *testing.T) {
	clearKnownEnv(t)
	setEnv(t, map[string]string{
		"DATABASE_URL": "postgres://x",
		"BEACON_ENV":   "production",
	})
	if _, err := Load(); err == nil {
		t.Fatal("production Load with no JWT_SECRET: want error, got nil")
	}

	// With the secret supplied, production loads and reports itself production.
	setEnv(t, map[string]string{"JWT_SECRET": "a-real-secret"})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.IsProduction() {
		t.Error("IsProduction() = false for BEACON_ENV=production")
	}
	if cfg.JWTSecret != "a-real-secret" {
		t.Errorf("JWTSecret = %q, want the supplied value", cfg.JWTSecret)
	}
}

func TestLoadParsesValues(t *testing.T) {
	clearKnownEnv(t)
	setEnv(t, map[string]string{
		"DATABASE_URL":        "postgres://x",
		"PORT":                "9090",
		"SHUTDOWN_TIMEOUT":    "5s",
		"JWT_TTL":             "30m",
		"CORS_ORIGINS":        "http://a.test, http://b.test",
		"S3_BUCKET":           "beacon-test",
		"S3_FORCE_PATH_STYLE": "false",
		"SMTP_PORT":           "2525",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want 9090", cfg.Port)
	}
	if cfg.ShutdownTimeout != 5*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 5s", cfg.ShutdownTimeout)
	}
	if cfg.JWTTTL != 30*time.Minute {
		t.Errorf("JWTTTL = %v, want 30m", cfg.JWTTTL)
	}
	if len(cfg.CORSOrigins) != 2 || cfg.CORSOrigins[0] != "http://a.test" || cfg.CORSOrigins[1] != "http://b.test" {
		t.Errorf("CORSOrigins = %v, want trimmed two-element slice", cfg.CORSOrigins)
	}
	if !cfg.StorageEnabled() {
		t.Error("StorageEnabled() = false with S3_BUCKET set")
	}
	if cfg.S3ForcePathStyle {
		t.Error("S3ForcePathStyle = true, want false (parsed)")
	}
	if cfg.SMTPPort != 2525 {
		t.Errorf("SMTPPort = %d, want 2525", cfg.SMTPPort)
	}
	if cfg.SMTPEnabled() {
		t.Error("SMTPEnabled() = true with no SMTP_HOST")
	}
}

func TestGetDuration(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		fallback time.Duration
		want     time.Duration
	}{
		{"empty falls back", "", 7 * time.Second, 7 * time.Second},
		{"go duration", "2m", time.Second, 2 * time.Minute},
		{"bare seconds", "45", time.Second, 45 * time.Second},
		{"garbage falls back", "not-a-duration", 3 * time.Second, 3 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const key = "TEST_DUR_KEY"
			if tt.value == "" {
				t.Setenv(key, "") // ensure unset/empty
			} else {
				t.Setenv(key, tt.value)
			}
			if got := getDuration(key, tt.fallback); got != tt.want {
				t.Errorf("getDuration(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestGetBoolAndInt(t *testing.T) {
	t.Setenv("TEST_BOOL_KEY", "true")
	if !getBool("TEST_BOOL_KEY", false) {
		t.Error("getBool(true) = false")
	}
	t.Setenv("TEST_BOOL_KEY", "garbage")
	if !getBool("TEST_BOOL_KEY", true) {
		t.Error("getBool(garbage) should fall back to true")
	}
	t.Setenv("TEST_INT_KEY", "42")
	if getInt("TEST_INT_KEY", 0) != 42 {
		t.Error("getInt(42) wrong")
	}
	t.Setenv("TEST_INT_KEY", "x")
	if getInt("TEST_INT_KEY", 9) != 9 {
		t.Error("getInt(garbage) should fall back")
	}
}

func TestSplitCSV(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{" a , b ,c ", []string{"a", "b", "c"}},
		{",,", nil},
	}
	for _, tt := range tests {
		got := splitCSV(tt.in)
		if len(got) != len(tt.want) {
			t.Errorf("splitCSV(%q) = %v, want %v", tt.in, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitCSV(%q)[%d] = %q, want %q", tt.in, i, got[i], tt.want[i])
			}
		}
	}
}

// clearKnownEnv unsets every variable Load reads so a test starts from a clean
// slate regardless of the host environment. t.Setenv restores them after.
func clearKnownEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"BEACON_ENV", "PORT", "DATABASE_URL", "SHUTDOWN_TIMEOUT", "CORS_ORIGINS",
		"JWT_SECRET", "JWT_TTL",
		"S3_BUCKET", "S3_REGION", "S3_ENDPOINT", "S3_ACCESS_KEY", "S3_SECRET_KEY",
		"S3_PRESIGN_TTL", "S3_FORCE_PATH_STYLE",
		"SMTP_HOST", "SMTP_PORT", "SMTP_USER", "SMTP_PASS", "EMAIL_FROM",
		"WORKER_POLL_INTERVAL", "WORKER_CONCURRENCY",
	} {
		t.Setenv(k, "")
	}
}
