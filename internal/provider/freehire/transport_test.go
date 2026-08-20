package freehire

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureRoundTripper answers with a canned response and keeps the request it
// was handed, so a test can read what Transport forwarded to its Base.
type captureRoundTripper struct {
	req *http.Request
}

func (c *captureRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	c.req = req
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

func TestTransport(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
	}))
	t.Cleanup(srv.Close)

	client := &http.Client{Transport: Transport{}}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { resp.Body.Close() })

	assert.Equal(t, DefaultUserAgent, gotUA)
	// RoundTrip clones before setting the header because the RoundTripper
	// contract forbids touching the request the caller still holds.
	assert.Empty(t, req.Header.Get("User-Agent"), "caller's request must not be modified")
}

func TestTransportBase(t *testing.T) {
	capture := &captureRoundTripper{}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://freehire.me/api/v1/jobs/search", nil)
	require.NoError(t, err)

	_, err = Transport{Base: capture}.RoundTrip(req)
	require.NoError(t, err)

	require.NotNil(t, capture.req)
	assert.Equal(t, DefaultUserAgent, capture.req.Header.Get("User-Agent"))
	assert.NotSame(t, req, capture.req, "Base must see the clone, not the caller's request")
}
