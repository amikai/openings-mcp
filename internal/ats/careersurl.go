package ats

import (
	"net/url"
	"strings"
)

// parseCareersInput reports whether a company input is a careers-URL
// candidate and parses it. Scheme-less inputs like "jobs.lever.co/acme"
// get https; anything without both a dot and a path stays a name.
func parseCareersInput(s string) (*url.URL, bool) {
	s = strings.TrimSpace(s)
	if !strings.Contains(s, "://") {
		if !strings.Contains(s, ".") || !strings.Contains(s, "/") {
			return nil, false
		}
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return nil, false
	}
	invalidScheme := u.Scheme != "http" && u.Scheme != "https"
	if u.Hostname() == "" || invalidScheme {
		return nil, false
	}
	return u, true
}

// firstPathSegment returns the first non-empty path segment, URL-decoded,
// or "" when the path has none (or decoding fails).
func firstPathSegment(u *url.URL) string {
	for seg := range strings.SplitSeq(strings.Trim(u.EscapedPath(), "/"), "/") {
		if seg == "" {
			continue
		}
		dec, err := url.PathUnescape(seg)
		if err != nil {
			return ""
		}
		return dec
	}
	return ""
}
