// Package search is Chapters 29 and 30, in that order and for that reason.
//
//	search.go          the service: Postgres FTS, Meili, and the fallback
//	reindex_worker.go  the write path — Postgres first, Meili after
//	request.go         engine-neutral shapes, so the service imports no SDK
//	meili/             the Meilisearch client
//
// Chapter 29: Postgres already has a search engine. A tsvector column, a GIN
// index and the @@ operator give you stemming, stopwords, ranking and
// tenant-scoped results with no new service to run, no second datastore to
// keep in sync, and no 3am page from a machine you forgot you owned.
//
// Chapter 30: you graduate to a real engine when a customer names a case
// Postgres cannot do — typo tolerance, per-document language, faceted filters
// — and not one day earlier. When you do, the rule that keeps you sane is
// that Postgres stays authoritative. Data flows Postgres -> Meili, one
// direction, and if Meili is down the fallback is the Chapter 29 path, so
// users see different ranking rather than zero results.
//
// # The graduation is not free, and here is the measurement
//
// Both engines were run against the same data. They are not ordered; they
// trade.
//
//	query "verifying", document "Verify the whole thing"
//	  Postgres  1 hit   — it stems, so verifying and verify are one word
//	  Meili     0 hits  — it does not stem; it tolerates typos instead
//
//	query "authentcation", document "Authentication Rewrite"
//	  Postgres  0 hits  — a misspelling is simply a different lexeme
//	  Meili     1 hit   — one typo at 8+ characters is within tolerance
//
// So switching Meili on does not strictly improve search; it changes which
// queries work. The fallback hides that — the same user typing the same word
// gets a different answer depending on whether Meili is up. That is why the
// response carries the engine that served it and the client shows it: a
// silent fallback is a silent incident.
//
// [verbatim ch29 + ch30 for the service shape, Search, searchMeili, IndexOne
// and the fallback; the constructors and the pgx plumbing are the glue the
// chapters imply.]
package search
