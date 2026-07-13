package eightfold

import "net/http"

// BrowserTransport sets the headers Eightfold's edge requires to see on
// every request. Go's default User-Agent ("Go-http-client/1.1") gets a
// bare HTTP 403 instead of JSON; a browser-shaped one is accepted. Wrap it
// around an *http.Client and pass that to NewClient via WithClient.
type BrowserTransport struct {
	Base http.RoundTripper
}

func (t BrowserTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}
