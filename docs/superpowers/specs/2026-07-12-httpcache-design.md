# HTTP-layer response cache (`internal/httpcache`)

Date: 2026-07-12
Status: approved

## Problem

Every MCP tool call re-fetches upstream in full. The ATS adapters are the
worst case: Ashby and Greenhouse have no server-side search, so every
`Search`/`Filters` call re-downloads the whole board dump, and Ashby's
`Detail` re-downloads it too (no per-job endpoint). Measured on 2026-07-12:

| Board | Dump size | Fetch time |
|---|---|---|
| OpenAI (Ashby, `includeCompensation=true`) | 12.2 MB | ~19.6 s |
| Anthropic (Greenhouse, `content=true`) | 5.9 MB | ~2.1 s |

A single real conversation produced 10 OpenAI tool calls = 10 full 12 MB
fetches (~120 MB, ~20 s each). The dump endpoints are parameterless full
snapshots — every query hits the identical URL, so responses are identical
within any short window. Other providers (104, cake, linkedin, google,
nvidia, tsmc) repeat identical requests less dramatically but benefit the
same way.

## Decisions

- **Layer: HTTP RoundTripper**, not domain-level. One implementation covers
  every provider (current and future) with zero adapter changes, and the
  1 GB budget is enforceable exactly because entries are bytes.
- **Library: otter v2** (`github.com/maypok86/otter`), `MaximumWeight = 1 GiB`
  with a weigher of `len(key) + len(body) + header estimate`.
- **TTL: 60 minutes, global**, `ExpiryAfterWrite`. Job data changes on an
  hours scale; the user accepted up to 1-hour-stale listings.
- **No ETag revalidation** (both ATS upstreams support If-None-Match/304,
  but at one refetch per board per hour it is not worth the extra state
  management; can be added later without interface changes).
- **No singleflight, no disk persistence, no Vary handling** — YAGNI for a
  single-client stdio server.

## Design

### Package `internal/httpcache`

```go
// Cache wraps one otter instance; all Transports sharing it share the
// same 1 GiB budget.
func New(maxBytes int64, ttl time.Duration) *Cache

// Wrap returns an http.RoundTripper that serves from the cache and
// delegates misses to next.
func (c *Cache) Wrap(next http.RoundTripper) http.RoundTripper
```

### Cache key

`method + " " + full URL + " " + sha256(request body)`.

- POST search endpoints (cake, nvidia/workday) are distinguished by body
  hash. The RoundTripper reads the request body and restores it with
  `io.NopCloser(bytes.NewReader(...))` before delegating.
- Cookie and Authorization headers are **excluded** from the key.
  LinkedIn's guest-session cookies differ per process; keying on them
  would only fragment the cache.

### Cacheability

- Methods: GET and POST only; everything else passes through.
- Responses: 2xx only. Non-2xx and transport errors pass through uncached —
  errors must not be remembered for 60 minutes.

### Stored value

Status code + response headers (minus hop-by-hop headers: Connection,
Keep-Alive, Transfer-Encoding, Upgrade, Proxy-*) + body bytes. On hit the
Transport reconstructs an `*http.Response`; fully transparent to the ogen
clients above.

### Wiring (`cmd/openings-mcp/main.go`)

One `*Cache` instance; every http.Client's transport is wrapped with it:

```go
cache := httpcache.New(1<<30, 60*time.Minute)

hc    := &http.Client{Timeout: 30 * time.Second, Transport: cache.Wrap(http.DefaultTransport)}
hc104 := &http.Client{Timeout: 30 * time.Second, Transport: cache.Wrap(job104.BrowserTransport{})}
hcLI  := &http.Client{Timeout: 30 * time.Second, Jar: jarLinkedin, Transport: cache.Wrap(http.DefaultTransport)}
```

All providers — 104, cake, nvidia, tsmc, google, linkedin, and the four
ATS adapters (workday, lever, ashby, greenhouse) — share the single 1 GiB
budget. Size and TTL are constants; no flags until a need appears.

### Observability

Hits and misses log at debug level via the existing slog logger
(`httpcache hit key=... size=...`) so hit rate can be verified in the
log file.

## Testing

Unit tests against `httptest.Server` stubs:

- same key twice → second call never reaches the network
- TTL expiry → refetch
- differing POST bodies → distinct entries
- non-2xx → not cached
- weight eviction with a tiny MaximumWeight
- requests differing only in Cookie header → same key

Plus a full run of the existing test suite for regressions.

## Out of scope / future work

- ETag revalidation (turn the hourly 12 MB refetch into a 0.1 s 304).
- A parsed-object cache for ATS dumps if the per-hit JSON decode +
  html2text cost proves noticeable (evolves this design into a two-layer
  cache; nothing here is thrown away).
