# HTTP Response Cache Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A shared 1 GiB, 60-minute-TTL HTTP response cache (otter v2) wrapped around every provider's `http.Client`, so repeated identical upstream requests are served from memory.

**Architecture:** New package `internal/httpcache` exposes `New(maxBytes, ttl, logger) *Cache` and `(*Cache).Wrap(next http.RoundTripper) http.RoundTripper`. One `*Cache` instance is created in `cmd/openings-mcp/main.go` and wrapped around all three `http.Client` transports; all providers share the single byte budget. Spec: `docs/superpowers/specs/2026-07-12-httpcache-design.md`.

**Tech Stack:** Go 1.26, `github.com/maypok86/otter/v2` v2.3.0 (verified API: `otter.Must(&otter.Options[K,V]{MaximumWeight uint64, Weigher func(K,V) uint32, ExpiryCalculator: otter.ExpiryWriting[K,V](ttl)})`, `GetIfPresent`, `Set`, `CleanUp`), stdlib `net/http`, `crypto/sha256`, `log/slog`, testify for assertions (already a dependency).

## Global Constraints

- Cache budget: exactly `1 << 30` bytes (1 GiB), shared across ALL providers via one `*Cache`.
- TTL: 60 minutes, `ExpiryWriting` (expiry after write), global — no per-endpoint TTL.
- Cache key: `method + " " + full URL + " " + hex(sha256(request body))`. Cookie and Authorization headers MUST NOT enter the key.
- Only GET and POST requests are cache-eligible; only 2xx responses are stored. Transport errors and non-2xx pass through uncached.
- No ETag revalidation, no singleflight, no disk persistence, no Vary handling (explicitly out of scope per spec).
- **Commits: do NOT commit automatically.** The user commits on explicit request only (user preference overrides the skill's commit steps). Where this plan says "checkpoint", run the verification commands and stop; never run `git commit`.
- Comments follow caller-perspective style (what the caller gets, not internal mechanics), per repo convention.

---

### Task 1: `internal/httpcache` core — hit/miss round-tripper

**Files:**
- Create: `internal/httpcache/httpcache.go`
- Test: `internal/httpcache/httpcache_test.go`
- Modify: `go.mod` / `go.sum` (via `go get`)

**Interfaces:**
- Consumes: nothing from this repo.
- Produces: `httpcache.New(maxBytes uint64, ttl time.Duration, logger *slog.Logger) *Cache` and `(*Cache).Wrap(next http.RoundTripper) http.RoundTripper`. Task 3 wires these into main.go. Tests in Task 2 extend `httpcache_test.go`.

- [x] **Step 1: Add the dependency**

```bash
go get github.com/maypok86/otter/v2@v2.3.0
```

- [x] **Step 2: Write the failing tests**

`internal/httpcache/httpcache_test.go` (white-box, package `httpcache`, so later tasks can reach `c.store`):

```go
package httpcache

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newUpstream returns a stub server counting how many requests actually
// reached it, and a client whose transport is wrapped by c.
func newUpstream(t *testing.T, c *Cache, status int, body string) (*httptest.Server, *atomic.Int64, *http.Client) {
	t.Helper()
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits, &http.Client{Transport: c.Wrap(http.DefaultTransport)}
}

func get(t *testing.T, hc *http.Client, url string) (int, string) {
	t.Helper()
	resp, err := hc.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(b)
}

func TestGetServedFromCache(t *testing.T) {
	c := New(1<<30, time.Hour, nil)
	srv, hits, hc := newUpstream(t, c, http.StatusOK, `{"jobs":[]}`)

	status, body := get(t, hc, srv.URL+"/board")
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, `{"jobs":[]}`, body)

	status, body = get(t, hc, srv.URL+"/board")
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, `{"jobs":[]}`, body)

	assert.Equal(t, int64(1), hits.Load(), "second GET must be served from cache")
}

func TestCachedResponseKeepsHeaders(t *testing.T) {
	c := New(1<<30, time.Hour, nil)
	srv, _, hc := newUpstream(t, c, http.StatusOK, "x")

	resp1, err := hc.Get(srv.URL)
	require.NoError(t, err)
	resp1.Body.Close()

	resp2, err := hc.Get(srv.URL)
	require.NoError(t, err)
	resp2.Body.Close()
	assert.Equal(t, "application/json", resp2.Header.Get("Content-Type"))
}

func TestNon2xxNotCached(t *testing.T) {
	c := New(1<<30, time.Hour, nil)
	srv, hits, hc := newUpstream(t, c, http.StatusNotFound, "nope")

	get(t, hc, srv.URL)
	get(t, hc, srv.URL)
	assert.Equal(t, int64(2), hits.Load(), "404 must not be cached")
}

func TestDistinctURLsDistinctEntries(t *testing.T) {
	c := New(1<<30, time.Hour, nil)
	srv, hits, hc := newUpstream(t, c, http.StatusOK, "x")

	get(t, hc, srv.URL+"/a")
	get(t, hc, srv.URL+"/b")
	assert.Equal(t, int64(2), hits.Load())
}
```

- [x] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/httpcache/ -v`
Expected: FAIL — `undefined: New`, `undefined: Cache`.

- [x] **Step 4: Write the implementation**

`internal/httpcache/httpcache.go`:

```go
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

// cacheKey identifies a request by method, URL, and body content. Headers
// stay out of the key on purpose: cookies (LinkedIn's per-process guest
// session) would only fragment the cache. A consumed request body is
// restored so the delegated round trip still sends it.
func cacheKey(req *http.Request) (string, error) {
	sum := sha256.New()
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
```

- [x] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/httpcache/ -v`
Expected: all four tests PASS.

- [x] **Step 6: Checkpoint (no commit)**

Run: `go build ./... && go vet ./...`
Expected: clean. Stop here; do not commit.

---

### Task 2: key semantics — POST bodies, cookie exclusion, passthrough, TTL, budget

**Files:**
- Modify: `internal/httpcache/httpcache_test.go` (append tests)

**Interfaces:**
- Consumes: `New`, `(*Cache).Wrap`, and (white-box) `(*Cache).store` from Task 1.
- Produces: nothing new — this task pins behavior Task 1 already implements; expect implementation fixes only if a test exposes a gap.

- [x] **Step 1: Write the tests**

Append to `internal/httpcache/httpcache_test.go`:

```go
import "strings" // add to the import block

func TestPostBodiesKeyedSeparately(t *testing.T) {
	c := New(1<<30, time.Hour, nil)
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		b, _ := io.ReadAll(r.Body)
		_, _ = w.Write(b) // echo, so we can assert the right entry is served
	}))
	t.Cleanup(srv.Close)
	hc := &http.Client{Transport: c.Wrap(http.DefaultTransport)}

	post := func(body string) string {
		resp, err := hc.Post(srv.URL, "application/json", strings.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		return string(b)
	}

	assert.Equal(t, `{"page":1}`, post(`{"page":1}`))
	assert.Equal(t, `{"page":2}`, post(`{"page":2}`), "different body must not collide")
	assert.Equal(t, `{"page":1}`, post(`{"page":1}`), "repeat body must replay the matching entry")
	assert.Equal(t, int64(2), hits.Load())
}

func TestCookieHeaderExcludedFromKey(t *testing.T) {
	c := New(1<<30, time.Hour, nil)
	srv, hits, hc := newUpstream(t, c, http.StatusOK, "x")

	req1, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req1.Header.Set("Cookie", "session=a")
	resp, err := hc.Do(req1)
	require.NoError(t, err)
	resp.Body.Close()

	req2, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req2.Header.Set("Cookie", "session=b")
	resp, err = hc.Do(req2)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, int64(1), hits.Load(), "cookie must not fragment the cache")
}

func TestNonGetPostPassesThrough(t *testing.T) {
	c := New(1<<30, time.Hour, nil)
	srv, hits, hc := newUpstream(t, c, http.StatusOK, "x")

	for range 2 {
		req, _ := http.NewRequest(http.MethodHead, srv.URL, nil)
		resp, err := hc.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
	}
	assert.Equal(t, int64(2), hits.Load(), "HEAD must bypass the cache")
}

func TestTTLExpiry(t *testing.T) {
	c := New(1<<30, 50*time.Millisecond, nil)
	srv, hits, hc := newUpstream(t, c, http.StatusOK, "x")

	get(t, hc, srv.URL)
	time.Sleep(120 * time.Millisecond)
	get(t, hc, srv.URL)
	assert.Equal(t, int64(2), hits.Load(), "expired entry must be refetched")
}

func TestWeightBudgetEvicts(t *testing.T) {
	// Budget far smaller than one entry: the entry must not be servable
	// on the second request.
	c := New(16, time.Hour, nil)
	srv, hits, hc := newUpstream(t, c, http.StatusOK, strings.Repeat("x", 1024))

	get(t, hc, srv.URL)
	c.store.CleanUp() // force pending maintenance so eviction is deterministic
	get(t, hc, srv.URL)
	assert.Equal(t, int64(2), hits.Load(), "over-budget entry must be evicted")
}
```

- [x] **Step 2: Run the new tests**

Run: `go test ./internal/httpcache/ -v -run 'TestPost|TestCookie|TestNonGet|TestTTL|TestWeight'`
Expected: PASS. If `TestWeightBudgetEvicts` is flaky even with `CleanUp()`, poll with `assert.Eventually` (up to 1 s) around the second `get` instead — otter evicts asynchronously.

- [x] **Step 3: Run the whole package suite with the race detector**

Run: `go test ./internal/httpcache/ -race -count=1`
Expected: PASS.

- [x] **Step 4: Checkpoint (no commit)**

Stop; do not commit.

---

### Task 3: wire the shared cache into the MCP server

**Files:**
- Modify: `cmd/openings-mcp/main.go:118-150` (`runWithTransport`)

**Interfaces:**
- Consumes: `httpcache.New(maxBytes uint64, ttl time.Duration, logger *slog.Logger) *Cache`, `(*Cache).Wrap(next http.RoundTripper) http.RoundTripper` from Task 1.
- Produces: nothing new — final integration.

- [x] **Step 1: Modify `runWithTransport`**

In `cmd/openings-mcp/main.go`, add the import `"github.com/amikai/openings-mcp/internal/httpcache"` and change the client construction at the top of `runWithTransport` to:

```go
func runWithTransport(transport mcp.Transport, logger *slog.Logger) error {
	// One shared response cache: every provider's transport draws on the
	// same 1 GiB budget, so a repeated query anywhere is served from
	// memory for up to an hour instead of re-fetching upstream.
	cache := httpcache.New(1<<30, 60*time.Minute, logger)

	// One connection pool, with a ceiling so a hung upstream fails that call
	// instead of stalling the MCP session.
	hc104 := &http.Client{Timeout: 30 * time.Second, Transport: cache.Wrap(job104.BrowserTransport{})}

	c104, err := job104.NewClient("https://www.104.com.tw", job104.WithClient(hc104))
	if err != nil {
		return err
	}

	hc := &http.Client{Timeout: 30 * time.Second, Transport: cache.Wrap(http.DefaultTransport)}
```

and the LinkedIn client construction to:

```go
	jarLinkedin, _ := cookiejar.New(nil)
	cLinkedin := linkedin.NewClient("https://www.linkedin.com", &http.Client{Timeout: 30 * time.Second, Jar: jarLinkedin, Transport: cache.Wrap(http.DefaultTransport)})
```

Everything else in the function (cake/nvidia/tsmc/google clients, `newATSRegistry(hc)`) stays as is — they all receive the wrapped `hc`.

- [x] **Step 2: Build and run the full test suite**

Run: `go build ./... && go test ./... -count=1`
Expected: everything passes; no existing test touches the network, so wrapping transports must not change any behavior.

- [x] **Step 3: Manual smoke check**

Run the server and exercise one tool twice via any MCP client, or simply verify debug logs appear when running with `--log-level debug`:

```bash
go run ./cmd/openings-mcp --log-level debug --log-file /tmp/openings-mcp.log
```

Expected in the log on a repeated identical query: one `httpcache miss` then `httpcache hit` lines for the same key.

- [x] **Step 4: Checkpoint (no commit)**

Stop. Report results to the user and wait for commit instructions.
