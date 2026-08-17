// Chapter 28, layer four: Redis. Every instance of the API shares it, so a value
// cached by one is a hit for all of them, and a delete by one is a delete for
// all of them. Slower than the in-process map — a network round trip instead of
// a pointer dereference — but still far cheaper than the query it replaces.
//
// [verbatim ch28] plus a New() that tolerates an empty URL, because this repo's
// services all degrade cleanly when their dependency is unconfigured (the same
// pattern storage and email already use).

package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis is a thin typed wrapper around go-redis. It serialises values as
// JSON; callers operate in terms of their domain types.
type Redis struct {
	c *redis.Client
}

// NewRedis parses the URL and returns a client. An empty URL returns (nil, nil)
// — the caller treats a nil *Redis as "no shared layer", and the stack falls
// through to the in-process cache and then Postgres. [glue, this repo's
// degrade-cleanly convention.]
func NewRedis(url string) (*Redis, error) {
	if url == "" {
		return nil, nil
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	c := redis.NewClient(opts)
	return &Redis{c: c}, nil
}

// Get retrieves value at key. Returns false if the key is missing or
// expired. Returns an error only for I/O failures, not for "miss."
//
// That distinction is the whole reason this wrapper exists: redis.Nil is not a
// failure, it is the ordinary answer "not here", and a caller that treats it as
// an error will log noise on every cold read.
func (r *Redis) Get(ctx context.Context, key string, dst any) (bool, error) {
	if r == nil {
		return false, nil
	}
	raw, err := r.c.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal(raw, dst)
}

// Set stores value under key. Note that ttl is not optional in this API — a
// cache entry without an expiry is a bug waiting for a quiet afternoon.
func (r *Redis) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	if r == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return r.c.Set(ctx, key, raw, ttl).Err()
}

func (r *Redis) Del(ctx context.Context, keys ...string) error {
	if r == nil {
		return nil
	}
	return r.c.Del(ctx, keys...).Err()
}

// Ping is used by /readyz so a dead cache shows up in the health check rather
// than as mysterious latency. [glue, wired to this repo's Chapter 18 readiness
// probe.]
func (r *Redis) Ping(ctx context.Context) error {
	if r == nil {
		return nil
	}
	return r.c.Ping(ctx).Err()
}

func (r *Redis) Close() error {
	if r == nil {
		return nil
	}
	return r.c.Close()
}
