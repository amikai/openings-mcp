package httpcache

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
