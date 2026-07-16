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

// careersURLRE matches myworkdayjobs.com career URLs and captures tenant
// and site. An optional locale segment before the site is tolerated and
// stripped; job deep links after the site are ignored.
//
// Examples (hostname + escaped path):
//   - stripe.wd5.myworkdayjobs.com/Stripe_Careers
//   - stripe.wd5.myworkdayjobs.com/en-US/Stripe_Careers
//   - acme.wd103.myworkdayjobs.com/zh-tw/jobs4acme/job/Taipei/Engineer_JR1
//
// myworkdaysite.com is deliberately unsupported (#113).
var careersURLRE = regexp.MustCompile(
	`(?i)^([^.]+)\.wd[^.]+\.myworkdayjobs\.com/(?:([a-z]{2}(?:-[a-z]{2})?)/)?([^/]+)`,
)

// localeSegment matches a lone language prefix used to reject locale-only
// paths like /en-US with no site segment after.
var localeSegment = regexp.MustCompile(`^[a-zA-Z]{2}(?:-[a-zA-Z]{2})?$`)

// ParseCareersURL reports whether u is a Workday career-site URL and
// extracts its parts. It accepts only the public host shape
// <tenant>.<wd*>.myworkdayjobs.com with a site path segment; locale
// prefixes and job deep links are tolerated and stripped.
//
// Workday's other public domain, myworkdaysite.com, is deliberately not
// supported: its URLs carry the tenant in the path, not the host
// (wd<N>.myworkdaysite.com/<locale?>/recruiting/<tenant>/<site>), and
// every tenant investigated is equally reachable through its
// myworkdayjobs.com form. See
// https://github.com/amikai/openings-mcp/issues/113 for the evidence.
func ParseCareersURL(u *url.URL) (CareersSite, bool) {
	host := strings.ToLower(u.Hostname())
	m := careersURLRE.FindStringSubmatch(host + u.EscapedPath())
	if m == nil {
		return CareersSite{}, false
	}
	site := m[3]
	// Locale-only paths like /en-US leave the locale in the site group.
	if m[2] == "" && localeSegment.MatchString(site) {
		return CareersSite{}, false
	}
	return CareersSite{Host: host, Tenant: m[1], Site: site}, true
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
