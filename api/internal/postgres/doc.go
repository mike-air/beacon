// Package postgres owns the connection to the database and the schema that
// runs on boot.
//
//	db.go       the pgxpool.Pool the whole application shares
//	migrate.go  the migrator, running the SQL embedded from migrations/
//
// The rest of the app is handed a *pgxpool.Pool — a set of reusable
// connections shared across requests, instead of one connection opened per
// request. That is Chapter 1's Friday-night story: opening a connection costs
// a TCP handshake, a TLS handshake and a Postgres startup exchange, and doing
// it per request means the database spends its afternoon accepting
// connections rather than answering queries.
//
// Migrations run in-process at startup, from files embedded in the binary, so
// deploying the binary deploys the schema it expects and there is no window
// where new code is talking to an old schema.
//
// Course mapping: Chapter 5 — Postgres with pgx and a connection pool;
// Chapter 6 — migrations.
package postgres
