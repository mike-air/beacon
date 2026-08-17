// Package jobs is Beacon's background-job queue: enqueue slow work now, run it
// later, retry on failure. It is the boundary Chapters 23–24 push email and
// webhook delivery across, and the rule behind it is short — nothing slow and
// nothing third-party happens on the request path.
//
//	jobs.go      the queue: enqueue, claim, retry, the polling worker
//	handlers.go  the work itself, one function per job kind
//	backup.go    the nightly off-provider pg_dump (Chapter 45)
//
// # How the queue works
//
// A `jobs` table, polled inside a transaction with FOR UPDATE SKIP LOCKED.
// That one clause is what makes N workers safe: each transaction takes rows
// nobody else has locked and skips the rest instead of blocking on them, so
// adding a worker adds throughput rather than contention.
//
// Because the queue is a table in the same database as the data, an enqueue
// can share a transaction with the write that caused it. Either both land or
// neither does — no job that references a row that was rolled back, and no
// committed row whose follow-up work was lost.
//
// # The trace crosses the boundary
//
// A context.Context cannot be written to a database row, so a job carries a
// W3C trace context in a column of its own (migration 0008) and the worker
// rebuilds the parent span from it. That is why one signup's trace shows the
// HTTP span and, seconds later, the send_email span underneath it, on a
// different machine.
//
// # Deviation: no River
//
// Course mapping: Chapter 26 — background jobs, which the course builds on
// River. River's recent releases need Go 1.24+, and even the go-1.22 line
// drags in a large dependency tree that fights this project's toolchain pin.
// This is the small Postgres-backed queue the spec offers as the fallback —
// the same FOR UPDATE SKIP LOCKED mechanism the chapter says River uses
// internally. Same shape (Enqueue + a handler registry + a polling worker),
// far fewer dependencies.
package jobs
