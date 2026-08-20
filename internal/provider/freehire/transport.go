package freehire

import "net/http"

// DefaultUserAgent names this project and points to its public repository,
// matching the identification request in freehire's openapi.yaml.
const DefaultUserAgent = "amikai/openings-mcp (+https://github.com/amikai/openings-mcp)"

// Transport sets the identifying User-Agent freehire asks callers to send.
// Wrap it around an *http.Client and pass that to NewClient via WithClient.
type Transport struct {
	Base http.RoundTripper
}

func (t Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("User-Agent", DefaultUserAgent)
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}
