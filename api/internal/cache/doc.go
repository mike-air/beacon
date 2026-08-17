// Package cache is Chapter 28's read path: five layers, fastest first, each
// one quicker but holding less. A read walks left to right and stops at the
// first hit; a write has to reach back and invalidate every layer it passed.
//
//	browser -> CDN -> in-process LRU -> Redis -> Postgres
//
// The first two layers are headers, and live in internal/http/cache.go. The
// three here are:
//
//	inproc.go     a fixed-size map inside one process — nanoseconds, nothing
//	              to serialise, gone on restart
//	redis.go      shared by every instance, so one process's write is another
//	              process's hit, and one process's delete is a delete for all
//	withcache.go  the read-through helper that ties them together
//
// # The line worth slowing down for
//
// withcache.go's singleflight. A hot key expires; two hundred requests miss
// at the same instant; without it, all two hundred run the loader — a
// stampede that lands on Postgres at exactly the moment the cache stopped
// protecting it. singleflight lets the first goroutine run the loader while
// the other 199 wait for its result. One query, not two hundred.
//
// # What belongs in here and what does not
//
// The in-process layer suits values that are tiny, very hot, and tolerant of
// being a few seconds stale — feature flags, org config. Never put anything a
// user would notice going backwards in it: two API instances hold two
// independent copies, so a user refreshing a page can be served the new value
// and then the old one.
package cache
