package config_test

import (
	"errors"
	"flag"
	"reflect"
	"strings"
	"testing"

	"beacon/internal/config"
)

const testDB = "postgres://u:p@localhost:5432/db?sslmode=disable"

// The rule the whole file exists for: a flag beats the environment, and the
// environment is still honoured for every flag NOT given. The second half is
// the one that breaks silently — using flag.VisitAll instead of flag.Visit
// makes every unset flag stamp its own default over the environment, and the
// symptom looks like "my .env is being ignored".
func TestFlagBeatsEnvAndEnvSurvivesUnsetFlags(t *testing.T) {
	t.Setenv("DATABASE_URL", testDB)
	t.Setenv("PORT", "7777")
	t.Setenv("METRICS_PORT", "9777")
	t.Setenv("REDIS_URL", "redis://from-env:6379")

	cfg, err := config.LoadAPIFlags([]string{"-port", "8123"})
	if err != nil {
		t.Fatalf("LoadAPIFlags: %v", err)
	}

	if cfg.Port != "8123" {
		t.Errorf("Port = %q, want the flag value 8123", cfg.Port)
	}
	if cfg.MetricsPort != "9777" {
		t.Errorf("MetricsPort = %q, want the env value 9777 — an unset flag overwrote it", cfg.MetricsPort)
	}
	if cfg.RedisURL != "redis://from-env:6379" {
		t.Errorf("RedisURL = %q, want the env value — an unset flag overwrote it", cfg.RedisURL)
	}
}

// No flags at all must behave exactly like Load(). If these two ever diverge,
// every binary is configured differently from every test.
func TestNoFlagsMatchesLoad(t *testing.T) {
	t.Setenv("DATABASE_URL", testDB)
	t.Setenv("PORT", "7777")

	viaLoad, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	viaFlags, err := config.LoadAPIFlags(nil)
	if err != nil {
		t.Fatalf("LoadAPIFlags: %v", err)
	}

	// DeepEqual, not ==: Config holds CORSOrigins []string, so it is not a
	// comparable type.
	if !reflect.DeepEqual(viaLoad, viaFlags) {
		t.Errorf("Load() and LoadAPIFlags(nil) disagree:\n load  = %+v\n flags = %+v", *viaLoad, *viaFlags)
	}
}

// A required value supplied ONLY by flag must satisfy validation. This is why
// finish() runs after the flags rather than inside fromEnv().
func TestRequiredValueCanComeFromAFlagAlone(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	if _, err := config.Load(); err == nil {
		t.Fatal("Load with no DATABASE_URL should fail, or this test proves nothing")
	}

	cfg, err := config.LoadAPIFlags([]string{"-database-url", testDB})
	if err != nil {
		t.Fatalf("LoadAPIFlags with -database-url: %v", err)
	}
	if cfg.DatabaseURL != testDB {
		t.Errorf("DatabaseURL = %q, want the flag value", cfg.DatabaseURL)
	}
}

// -env production is the local "run it like production" switch, and it must
// pull the production rules with it — JWT_SECRET stops being optional.
func TestEnvFlagAppliesProductionRules(t *testing.T) {
	t.Setenv("DATABASE_URL", testDB)
	t.Setenv("BEACON_ENV", "development")
	t.Setenv("JWT_SECRET", "")

	if _, err := config.LoadAPIFlags([]string{"-env", "production"}); err == nil {
		t.Error("-env production with no JWT_SECRET should fail; the flag did not reach the validation")
	}

	// The same run in development falls back to the dev secret, as before.
	cfg, err := config.LoadAPIFlags(nil)
	if err != nil {
		t.Fatalf("development: %v", err)
	}
	if cfg.JWTSecret == "" {
		t.Error("development should fall back to the dev signing secret")
	}
}

func TestHelpIsNotAFailure(t *testing.T) {
	t.Setenv("DATABASE_URL", testDB)

	_, err := config.LoadAPIFlags([]string{"-help"})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("err = %v, want flag.ErrHelp so the binaries can exit 0", err)
	}
}

func TestStrayArgumentIsRejected(t *testing.T) {
	t.Setenv("DATABASE_URL", testDB)

	_, err := config.LoadAPIFlags([]string{"serve"})
	if err == nil {
		t.Fatal("a stray argument should be rejected, not ignored")
	}
	if !strings.Contains(err.Error(), "serve") {
		t.Errorf("error = %q, should name the argument it did not understand", err)
	}
}

// The worker's flag set is deliberately smaller. Offering it -port would parse
// and then do nothing, since the worker never reads Config.Port.
func TestWorkerFlags(t *testing.T) {
	t.Setenv("DATABASE_URL", testDB)
	t.Setenv("WORKER_CONCURRENCY", "5")

	cfg, err := config.LoadWorkerFlags([]string{"-concurrency", "1"})
	if err != nil {
		t.Fatalf("LoadWorkerFlags: %v", err)
	}
	if cfg.WorkerConcurrency != 1 {
		t.Errorf("WorkerConcurrency = %d, want the flag value 1", cfg.WorkerConcurrency)
	}

	if _, err := config.LoadWorkerFlags([]string{"-port", "8081"}); err == nil {
		t.Error("-port should be rejected by the worker rather than silently accepted")
	}
}
