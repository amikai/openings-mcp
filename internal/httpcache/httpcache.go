// Package httpcache provides a shared in-memory HTTP response cache. Wrap
// an http.RoundTripper with (*Cache).Wrap and identical GET/POST requests
// are answered from memory for the cache's TTL instead of hitting the
// network. All Transports created from one Cache share its byte budget.
package httpcache

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"slices"
	"time"

	"github.com/maypok86/otter/v2"
)

// entry is one stored upstream response.
type entry struct {
	status int
	header http.Header
	body   []byte
}

// Cache is a byte-bounded, TTL-expiring response store. Create one per
// process and Wrap every outbound transport with it; entries evict by
// LFU/LRU policy once the byte budget is exceeded and expire ttl after
// they were written.
type Cache struct {
	store  *otter.Cache[string, *entry]
	logger *slog.Logger
}

// New returns a Cache holding at most maxBytes of responses, each entry
// expiring ttl after it was stored. A nil logger disables hit/miss logging.
func New(maxBytes uint64, ttl time.Duration, logger *slog.Logger) *Cache {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Cache{
		store: otter.Must(&otter.Options[string, *entry]{
			MaximumWeight:    maxBytes,
			Weigher:          weigh,
			ExpiryCalculator: otter.ExpiryWriting[string, *entry](ttl),
		}),
		logger: logger,
	}
}

// Close stops the background maintenance goroutine that Otter runs to
// enforce the expiry policy. Otter otherwise only stops it once the Cache
// becomes unreachable and the garbage collector runs, which keeps the
// goroutine alive for the rest of a test binary. A closed Cache must not
// be used again.
func (c *Cache) Close() {
	c.store.StopAllGoroutines()
}

// weigh charges an entry for its body, key, and header bytes so the cache's
// MaximumWeight tracks real memory use.
func weigh(key string, e *entry) uint32 {
	w := len(key) + len(e.body)
	for k, vals := range e.header {
		w += len(k)
		for _, v := range vals {
			w += len(v)
		}
	}
	if w > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(w)
}

// Wrap returns a RoundTripper that serves cacheable requests from c and
// delegates everything else — and every miss — to next.
func (c *Cache) Wrap(next http.RoundTripper) http.RoundTripper {
	return &transport{cache: c, next: next}
}

type transport struct {
	cache *Cache
	next  http.RoundTripper
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet && req.Method != http.MethodPost {
		return t.next.RoundTrip(req)
	}
	key, err := cacheKey(req)
	if err != nil {
		return nil, err
	}
	if e, ok := t.cache.store.GetIfPresent(key); ok {
		t.cache.logger.Debug("httpcache hit", "key", key, "size", len(e.body))
		return e.response(req), nil
	}
	resp, err := t.next.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return resp, nil
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("httpcache: read upstream body: %w", err)
	}
	e := &entry{status: resp.StatusCode, header: strippedHeader(resp.Header), body: body}
	t.cache.store.Set(key, e)
	t.cache.logger.Debug("httpcache miss", "key", key, "size", len(body))
	return e.response(req), nil
}

// cacheKey identifies a request by method, URL, request headers, and body
// content. Headers belong in the key because a provider may select the
// representation with a header rather than the URL: Indeed sends the
// search country in indeed-co against one fixed GraphQL endpoint, so two
// countries' searches are otherwise indistinguishable and one would be
// served the other's postings. A consumed request body is restored so the
// delegated round trip still sends it.
func cacheKey(req *http.Request) (string, error) {
	sum := sha256.New()
	writeCanonicalHeader(sum, req.Header)
	if req.Body != nil {
		b, err := io.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			return "", fmt.Errorf("httpcache: read request body: %w", err)
		}
		sum.Write(b)
		req.Body = io.NopCloser(bytes.NewReader(b))
	}
	return req.Method + " " + req.URL.String() + " " + hex.EncodeToString(sum.Sum(nil)), nil
}

// unkeyedHeaders are request headers deliberately left out of the key.
// They carry per-process session state rather than selecting a
// representation, so keying on them would fragment the cache without
// preventing a wrong hit. Every other header is keyed: one that varies
// without changing the response only costs hit rate, whereas one that
// changes the response and is missing from the key serves wrong data.
var unkeyedHeaders = map[string]bool{
	"Cookie": true,
}

// writeCanonicalHeader folds h into sum by ascending header name, so two
// requests carrying the same headers hash alike whatever order the headers
// were set in. Fields are NUL-separated because a header name or value
// cannot contain NUL, which keeps "ab" + "c" from hashing like "a" + "bc".
func writeCanonicalHeader(sum io.Writer, h http.Header) {
	names := make([]string, 0, len(h))
	for name := range h {
		if !unkeyedHeaders[http.CanonicalHeaderKey(name)] {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	for _, name := range names {
		_, _ = io.WriteString(sum, name+"\x00")
		for _, v := range h[name] {
			_, _ = io.WriteString(sum, v+"\x00")
		}
	}
}

// hopByHop are connection-scoped response headers that must not be
// replayed to a different connection (RFC 9110 §7.6.1).
var hopByHop = []string{
	"Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization",
	"Te", "Trailer", "Transfer-Encoding", "Upgrade",
}

func strippedHeader(h http.Header) http.Header {
	out := h.Clone()
	for _, k := range hopByHop {
		out.Del(k)
	}
	return out
}

// response materializes a stored entry as a fresh *http.Response for req.
// The header is cloned so callers can mutate it without corrupting the
// cached copy.
func (e *entry) response(req *http.Request) *http.Response {
	return &http.Response{
		Status:        fmt.Sprintf("%d %s", e.status, http.StatusText(e.status)),
		StatusCode:    e.status,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        e.header.Clone(),
		Body:          io.NopCloser(bytes.NewReader(e.body)),
		ContentLength: int64(len(e.body)),
		Request:       req,
	}
}
