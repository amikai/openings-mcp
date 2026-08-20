package freehire

import "net/http"

// DefaultUserAgent names this project and points to its public repository,
// matching the identification request in freehire's openapi.yaml.
const DefaultUserAgent = "amikai/openings-mcp (+https://github.com/amikai/openings-mcp)"

// Transport sets the User-Agent header on outgoing requests to freehire.me.
// When UserAgent is empty, [DefaultUserAgent] is used. Wrap it around an
// *http.Client and pass that to NewClient via WithClient.
type Transport struct {
	Base      http.RoundTripper
	UserAgent string
}

func (t Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	ua := t.UserAgent
	if ua == "" {
		ua = DefaultUserAgent
	}
	req.Header.Set("User-Agent", ua)
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}
