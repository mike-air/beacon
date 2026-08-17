// Chapter 28's read-through helper: the thing that ties the layers together so
// callers write one line instead of five.
//
// The singleflight is the part worth slowing down for. A hot key expires. Two
// hundred requests miss at the same instant and all two hundred run the loader
// — that is a stampede, and it lands on Postgres at exactly the moment the
// cache stopped protecting it. singleflight.Group.Do makes the first goroutine
// run the loader while the other 199 wait for its result. One query, not two
// hundred.
//
// [verbatim ch28] with the constructor and the Del filled in — the chapter
// shows the struct and Get and leaves the wiring implied.

package cache

import (
	"context"
	"time"

	"golang.org/x/sync/singleflight"
)

type CachedReader[T any] struct {
	redis  *Redis
	inproc *InProc[string, T]
	group  singleflight.Group
	ttl    time.Duration
}

// NewCachedReader builds the two-layer read-through cache. redis may be nil
// (unconfigured), in which case only the in-process layer is used.
// [glue, implied by ch28.]
func NewCachedReader[T any](r *Redis, size int, ttl time.Duration) (*CachedReader[T], error) {
	inproc, err := NewInProc[string, T](size, ttl)
	if err != nil {
		return nil, err
	}
	return &CachedReader[T]{redis: r, inproc: inproc, ttl: ttl}, nil
}

// Get tries inproc first, then Redis, then runs `loader` to fetch from
// the database. The singleflight ensures only one loader runs per key
// even under heavy concurrent miss.
func (c *CachedReader[T]) Get(
	ctx context.Context,
	key string,
	loader func(ctx context.Context) (T, error),
) (T, error) {
	if v, ok := c.inproc.Get(key); ok {
		return v, nil
	}

	var zero T
	var dst T
	found, err := c.redis.Get(ctx, key, &dst)
	if err == nil && found {
		c.inproc.Set(key, dst)
		return dst, nil
	}

	// Miss. Use singleflight to coalesce concurrent loaders.
	v, err, _ := c.group.Do(key, func() (any, error) {
		out, err := loader(ctx)
		if err != nil {
			return zero, err
		}
		_ = c.redis.Set(ctx, key, out, c.ttl)
		c.inproc.Set(key, out)
		return out, nil
	})
	if err != nil {
		return zero, err
	}
	return v.(T), nil
}

// Del drops the key from both layers. Call it AFTER the write commits, never
// before: invalidate first and a concurrent read can re-populate the cache from
// the old row microseconds before your transaction lands, leaving a stale entry
// with a full TTL ahead of it. [glue, the chapter states the rule; this is the
// method that follows it.]
func (c *CachedReader[T]) Del(ctx context.Context, key string) error {
	c.inproc.Del(key)
	return c.redis.Del(ctx, key)
}
