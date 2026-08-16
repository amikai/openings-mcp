package ultipro

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// CareersSite addresses one UltiPro board by its public-URL parts, for
// tenants outside the curated roster. It carries the same values Company
// encodes.
type CareersSite struct {
	Host        string
	CompanyCode string
	BoardID     string
}

// careersURLRE matches UltiPro career-board URLs and captures companyCode
// and boardId.
//
// Examples (hostname + escaped path):
//   - recruiting.ultipro.com/TEC1006TESER/JobBoard/18180d88-ced0-4361-bd09-d5eef66dab24/
//   - recruiting2.ultipro.com/SAL1002/JobBoard/bcc2e2d1-d94c-2041-4126-28086417eb0a/OpportunityDetail?opportunityId=...
var careersURLRE = regexp.MustCompile(
	`(?i)^(?P<host>recruiting\d*\.ultipro\.com)/(?P<code>[^/]+)/JobBoard/(?P<board>[0-9a-fA-F-]{36})`,
)

// ParseCareersURL reports whether u is an UltiPro career-board URL and
// extracts its parts.
func ParseCareersURL(u *url.URL) (CareersSite, bool) {
	host := strings.ToLower(u.Hostname())
	m := careersURLRE.FindStringSubmatch(host + u.EscapedPath())
	if m == nil {
		return CareersSite{}, false
	}
	code := m[careersURLRE.SubexpIndex("code")]
	board := m[careersURLRE.SubexpIndex("board")]
	if code == "" {
		return CareersSite{}, false
	}
	return CareersSite{Host: host, CompanyCode: code, BoardID: strings.ToLower(board)}, true
}

// CanonicalURL renders the slug form the ats layer circulates for
// non-roster boards.
func (s CareersSite) CanonicalURL() string {
	return fmt.Sprintf("https://%s/%s/JobBoard/%s/", s.Host, s.CompanyCode, s.BoardID)
}

// BaseURL returns the board's API base URL, for [NewClient].
func (s CareersSite) BaseURL() string {
	return fmt.Sprintf("https://%s/%s/JobBoard/%s", s.Host, s.CompanyCode, s.BoardID)
}
