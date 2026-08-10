// Package testsupport holds shared helpers for integration and e2e tests: a
// real Postgres pool wired to the embedded migrations, plus a truncate helper
// to reset state between tests.
//
// Course mapping: Chapter 39 — integration tests against a real database.
//
// DEVIATION (chosen, noted per the build spec): the course spins Postgres up
// with testcontainers. We do NOT use testcontainers — its dependency tree wants
// a newer Go toolchain and fights this project's `go 1.23` / go1.23.4 pin.
// Instead every integration/e2e test is ENV-GATED: it reads TEST_DATABASE_URL
// and SKIPS (never fails) when it is unset. CI supplies a postgres:16 service
// container and sets the variable, so the same tests run for real there. No
// build tags — these files compile in the normal build; they just skip locally.
package testsupport

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"beacon/internal/postgres"
	"beacon/migrations"
)

// dataTables are every table holding test data, in an order safe to truncate
// together. CASCADE + RESTART IDENTITY clears them and any dependents in one
// statement, so the listed set only needs to name the roots we own. We list all
// of them explicitly so a new table added without a truncate is an obvious miss.
var dataTables = []string{
	"webhook_deliveries",
	"webhooks",
	"jobs",
	"attachments",
	"comments",
	"tasks",
	"projects",
	"memberships",
	"organizations",
	"users",
}

// NewTestPool returns a migrated *pgxpool.Pool against TEST_DATABASE_URL, or
// skips the test when that variable is unset. It registers a cleanup that
// truncates every data table and closes the pool, so each test starts from a
// known-empty schema. Connect + migrate happen once per call; tests that want a
// shared pool can call it in TestMain instead.
func NewTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("testsupport: connect: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("testsupport: ping: %v", err)
	}
	if err := postgres.Migrate(ctx, pool, migrations.Files); err != nil {
		pool.Close()
		t.Fatalf("testsupport: migrate: %v", err)
	}

	// Start clean, and clean up after.
	Truncate(t, pool)
	t.Cleanup(func() {
		Truncate(t, pool)
		pool.Close()
	})
	return pool
}

// Truncate empties every data table and restarts identity sequences, leaving the
// schema (and schema_migrations) intact. Call between sub-tests that share a
// pool to isolate them.
func Truncate(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	sql := "TRUNCATE TABLE "
	for i, tbl := range dataTables {
		if i > 0 {
			sql += ", "
		}
		sql += tbl
	}
	sql += " RESTART IDENTITY CASCADE"

	if _, err := pool.Exec(ctx, sql); err != nil {
		t.Fatalf("testsupport: truncate: %v", err)
	}
}
