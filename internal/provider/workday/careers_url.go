package workday

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// CareersSite addresses one Workday career site by its public-URL parts,
// for tenants outside the curated roster. It carries the same values
// Company encodes; Host keeps the instance and domain verbatim.
type CareersSite struct {
	Host   string // e.g. "stripe.wd5.myworkdayjobs.com"
	Tenant string // first host label
	Site   string // career-site path segment
}

// localeSegment matches the optional language prefix careers URLs carry
// before the site segment ("en-US", "zh-tw", "fr").
var localeSegment = regexp.MustCompile(`^[a-zA-Z]{2}(?:-[a-zA-Z]{2})?$`)

// ParseCareersURL reports whether u is a Workday career-site URL and
// extracts its parts. It accepts only the public host shape
// <tenant>.<wd*>.myworkdayjobs.com with a site path segment; locale
// prefixes and job deep links are tolerated and stripped.
//
// KNOWN ISSUE: myworkdaysite.com is listed in the domain check below, but
// real URLs on that domain carry the tenant in the path, not the host
// (wd<N>.myworkdaysite.com/<locale?>/recruiting/<tenant>/<site>), so they
// never survive the four-label check and are always rejected. See
// https://github.com/amikai/openings-mcp/issues/113 before extending.
func ParseCareersURL(u *url.URL) (CareersSite, bool) {
	host := strings.ToLower(u.Hostname())
	labels := strings.Split(host, ".")
	if len(labels) != 4 {
		return CareersSite{}, false
	}
	if domain := labels[2] + "." + labels[3]; domain != "myworkdayjobs.com" && domain != "myworkdaysite.com" {
		return CareersSite{}, false
	}
	if labels[0] == "" || !strings.HasPrefix(labels[1], "wd") {
		return CareersSite{}, false
	}
	segs := strings.Split(strings.Trim(u.EscapedPath(), "/"), "/")
	if len(segs) > 0 && localeSegment.MatchString(segs[0]) {
		segs = segs[1:]
	}
	if len(segs) == 0 || segs[0] == "" {
		return CareersSite{}, false
	}
	return CareersSite{Host: host, Tenant: labels[0], Site: segs[0]}, true
}

// BaseURL derives the CXS API base URL, mirroring Company.BaseURL.
func (s CareersSite) BaseURL() string {
	return fmt.Sprintf("https://%s/wday/cxs/%s/%s", s.Host, s.Tenant, s.Site)
}

// CanonicalURL renders the slug form the ats layer circulates for
// non-roster tenants: locale, deep links, query, and fragment stripped.
func (s CareersSite) CanonicalURL() string {
	return fmt.Sprintf("https://%s/%s", s.Host, s.Site)
}
