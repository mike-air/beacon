// Chapter 28's first two layers, the ones you get for free by saying the right
// thing in a header.
//
//   - Cache-Control tells the browser (and any CDN in front of us) whether it
//     may keep a copy, who may see it, and for how long.
//   - ETag names the version of a resource. The client sends it back as
//     If-None-Match; if it still matches we answer 304 with no body, and the
//     saving is the entire payload.
//   - Vary is the one nobody remembers until the incident: without
//     `Vary: Authorization`, a shared cache is free to hand user A's response
//     to user B. Set it on anything private.
//
// [verbatim ch28 for setCacheHeaders; withETag is described in the chapter as a
// middleware over a setETag(ctx, ...) handler contract, and this is that
// contract written out.]
package http

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"
)

type cacheKind int

const (
	cachePrivate cacheKind = iota // one user may cache it; shared caches may not
	cachePublic                   // a CDN may hold one copy for everybody
	cacheNoStore                  // nobody writes this down anywhere
)

func setCacheHeaders(w http.ResponseWriter, kind cacheKind, maxAge time.Duration) {
	switch kind {
	case cachePrivate:
		w.Header().Set("Cache-Control", fmt.Sprintf("private, max-age=%d", int(maxAge.Seconds())))
	case cachePublic:
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", int(maxAge.Seconds())))
	case cacheNoStore:
		w.Header().Set("Cache-Control", "no-store")
	}
	w.Header().Set("Vary", "Accept-Encoding, Authorization")
}

type etagCtxKey struct{}

// etagHolder is stashed in the context so the handler, which is the only code
// that knows the resource's version, can hand the tag back out to the
// middleware, which is the only code positioned to answer 304.
type etagHolder struct{ tag string }

// setETag is called by a handler once it knows the version of what it is about
// to write. Safe to call when withETag is not in the chain — it just does
// nothing.
func setETag(ctx context.Context, tag string) {
	if h, ok := ctx.Value(etagCtxKey{}).(*etagHolder); ok {
		h.tag = tag
	}
}

// etagFor builds a weak version token from any set of strings — typically a row
// id and its updated_at. Cheap, opaque, and stable, which is all an ETag has to
// be.
func etagFor(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return `"` + hex.EncodeToString(h.Sum(nil)[:12]) + `"`
}

// withETag handles If-None-Match for an endpoint. The handler sets the ETag
// value with setETag(ctx, ...) before writing the response; this middleware
// compares it against the request header and short-circuits with a 304 if they
// match.
//
// The buffering matters: we cannot know whether to send a 304 until the handler
// has told us the tag, and by then it is about to write the body. So we hold
// the body, ask the question, and either flush it or drop it.
func withETag(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		holder := &etagHolder{}
		ctx := context.WithValue(r.Context(), etagCtxKey{}, holder)

		buf := &bufferingWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(buf, r.WithContext(ctx))

		if holder.tag != "" {
			w.Header().Set("ETag", holder.tag)
			if r.Header.Get("If-None-Match") == holder.tag && buf.status == http.StatusOK {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
		w.WriteHeader(buf.status)
		_, _ = w.Write(buf.body)
	})
}

type bufferingWriter struct {
	http.ResponseWriter
	status  int
	body    []byte
	written bool
}

func (b *bufferingWriter) WriteHeader(status int) {
	if !b.written {
		b.status = status
		b.written = true
	}
}

func (b *bufferingWriter) Write(p []byte) (int, error) {
	b.body = append(b.body, p...)
	return len(p), nil
}
