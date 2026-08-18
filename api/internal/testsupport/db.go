// Package testsupport holds shared helpers for integration and e2e tests: a
// real Postgres pool wired to the embedded migrations, against a private
// database per test.
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
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"beacon/internal/postgres"
	"beacon/migrations"
)

// dataTables are every table holding test data, in an order safe to truncate
// together. CASCADE + RESTART IDENTITY clears them and any dependents in one
// statement, so the listed set only needs to name the roots we own. Kept for
// Truncate below — NewTestPool itself no longer calls it; see that function's
// comment for why.
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

// NewTestPool returns a migrated *pgxpool.Pool against a fresh, private
// database, or skips the test when TEST_DATABASE_URL is unset.
//
// Each call runs CREATE DATABASE for a name unique to this call — it does NOT
// truncate a table set shared with every other test, which is what this
// replaced. That design broke the moment enough integration packages existed
// to overlap in time: `go test ./...` runs different PACKAGES as separate OS
// processes IN PARALLEL (default `-p` = GOMAXPROCS), and every one of those
// processes was pointed at the same TEST_DATABASE_URL. Package A's cleanup
// truncating `memberships` while package B was mid-insert produced exactly
// the foreign-key violations and deadlocks a shared database always risks —
// not flakiness, a guaranteed collision once enough packages ran at once. A
// private database per call has no shared state across processes to collide
// on, so parallel `go test` stays parallel and correctness stops depending on
// scheduling luck.
func NewTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("set TEST_DATABASE_URL to run integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	name := freshDatabaseName(t)
	createDatabase(t, ctx, base, name)

	dsn, err := withDatabase(base, name)
	if err != nil {
		t.Fatalf("testsupport: rewrite dsn for %s: %v", name, err)
	}

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

	t.Cleanup(func() {
		pool.Close()
		dropDatabase(t, base, name)
	})
	return pool
}

// freshDatabaseName derives a Postgres-safe, collision-proof name from the
// calling test. Subtests carry `/` in t.Name(), which Postgres identifiers
// cannot; the random suffix means two tests that happen to share a base name
// — in different packages, or the same test on a re-run — never collide, and
// capping the base keeps the whole name inside the 63-byte identifier limit.
func freshDatabaseName(t *testing.T) string {
	t.Helper()

	base := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		default:
			return '_'
		}
	}, t.Name())
	if len(base) > 40 {
		base = base[:40]
	}

	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatalf("testsupport: random suffix: %v", err)
	}
	return fmt.Sprintf("test_%s_%s", base, hex.EncodeToString(suffix))
}

// createDatabase and dropDatabase each open a short-lived connection to the
// database named in TEST_DATABASE_URL (i.e. base itself) to run the DDL —
// CREATE/DROP DATABASE cannot target the database a connection is already
// using, and cannot run inside a transaction, both of which pgxpool's default
// autocommit Exec already satisfies.
func createDatabase(t *testing.T, ctx context.Context, base, name string) {
	t.Helper()
	admin, err := pgxpool.New(ctx, base)
	if err != nil {
		t.Fatalf("testsupport: admin connect: %v", err)
	}
	defer admin.Close()

	if _, err := admin.Exec(ctx, "CREATE DATABASE "+quoteIdent(name)); err != nil {
		t.Fatalf("testsupport: create database %s: %v", name, err)
	}
}

func dropDatabase(t *testing.T, base, name string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	admin, err := pgxpool.New(ctx, base)
	if err != nil {
		// A database leaked in a throwaway CI container costs nothing; failing
		// the test over cleanup would tell the reader the wrong story about
		// what actually broke.
		t.Logf("testsupport: admin connect for drop %s: %v", name, err)
		return
	}
	defer admin.Close()

	// WITH (FORCE), Postgres 13+: disconnects any session still attached.
	// Belt and braces — pool.Close() ran immediately before this, but a
	// connection mid-drain can still hold the database open for a moment.
	if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+quoteIdent(name)+" WITH (FORCE)"); err != nil {
		t.Logf("testsupport: drop database %s: %v", name, err)
	}
}

// quoteIdent double-quotes a Postgres identifier for use in DDL, where a
// placeholder is not an option — CREATE/DROP DATABASE take a name, not a
// value. freshDatabaseName already restricts its output to [a-z0-9_], so the
// escaping here is defense in depth, not load-bearing.
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// withDatabase returns dsn with its database (path) component replaced.
func withDatabase(dsn, name string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	u.Path = "/" + name
	return u.String(), nil
}

// Truncate empties every data table and restarts identity sequences, leaving
// the schema (and schema_migrations) intact. NewTestPool no longer calls this
// — a fresh database starts empty by construction — but it stays exported for
// the one pattern it is still right for: several sub-tests sharing ONE pool
// within a single package (via TestMain), where truncating between them is
// cheaper than a fresh database per sub-test and just as safe, since intra-
// package tests run in one process and do not race each other unless they
// opt into t.Parallel().
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
