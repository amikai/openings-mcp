package ats

import (
	"net/url"
	"regexp"
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

// matchCareersSlug matches re against lowercase(host)+escapedPath and
// returns the URL-decoded named capture group "slug".
//
// Example:
//
//	re := regexp.MustCompile(`(?i)^jobs\.ashbyhq\.com/(?P<slug>[^/]+)`)
//	u, _ := url.Parse("https://jobs.ashbyhq.com/Acme%20Inc")
//	slug, ok := matchCareersSlug(re, u) // "Acme Inc", true
func matchCareersSlug(re *regexp.Regexp, u *url.URL) (string, bool) {
	m := re.FindStringSubmatch(strings.ToLower(u.Hostname()) + u.EscapedPath())
	if m == nil {
		return "", false
	}
	slug, err := url.PathUnescape(namedGroup(re, m, "slug"))
	if err != nil || slug == "" {
		return "", false
	}
	return slug, true
}

// namedGroup returns the named capture from m, or "" if missing.
func namedGroup(re *regexp.Regexp, m []string, name string) string {
	i := re.SubexpIndex(name)
	if i < 0 || i >= len(m) {
		return ""
	}
	return m[i]
}
