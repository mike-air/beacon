// Layer three: a fixed-size TTL-LRU living inside one process. Reading it is a
// map lookup and a pointer dereference — no serialisation, no network — and
// everything in it is gone when the process restarts, which is the correct
// behaviour for a cache and a disaster for anything else.
//
// [verbatim ch28] The layer diagram and the rule for what may live here are in
// doc.go.

package cache

import (
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

// InProc is a small TTL-bounded LRU. Values expire after `ttl` regardless
// of LRU eviction. Used for very hot, very small values (feature flags,
// org-level config, etc.) where staleness up to `ttl` is acceptable.
//
// The pairing is the point: LRU alone bounds memory but lets a cold entry sit
// there wrong forever; a TTL alone bounds staleness but lets the map grow
// without limit. You want both.
type InProc[K comparable, V any] struct {
	mu  sync.Mutex
	c   *lru.Cache[K, entry[V]]
	ttl time.Duration
}

type entry[V any] struct {
	value V
	exp   time.Time
}

func NewInProc[K comparable, V any](size int, ttl time.Duration) (*InProc[K, V], error) {
	c, err := lru.New[K, entry[V]](size)
	if err != nil {
		return nil, err
	}
	return &InProc[K, V]{c: c, ttl: ttl}, nil
}

func (p *InProc[K, V]) Get(k K) (V, bool) {
	p.mu.Lock()
	e, ok := p.c.Get(k)
	p.mu.Unlock()
	var zero V
	if !ok {
		return zero, false
	}
	if time.Now().After(e.exp) {
		return zero, false
	}
	return e.value, true
}

func (p *InProc[K, V]) Set(k K, v V) {
	p.mu.Lock()
	p.c.Add(k, entry[V]{value: v, exp: time.Now().Add(p.ttl)})
	p.mu.Unlock()
}

// Del removes a key. Invalidation reaches this layer too — but note the honest
// limit the chapter names: this only clears the copy in THIS process. On three
// instances, the other two keep their stale copy until the TTL runs out. That
// is why the TTL is short and why Redis, which every instance shares, is where
// cross-machine invalidation actually happens.
func (p *InProc[K, V]) Del(k K) {
	p.mu.Lock()
	p.c.Remove(k)
	p.mu.Unlock()
}
