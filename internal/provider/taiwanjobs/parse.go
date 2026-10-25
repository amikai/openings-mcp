package taiwanjobs

import (
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
)

// tagAnnotation matches the Chinese annotation embedded in upstream element
// names, e.g. <OCCU_DESC（職務名稱）>. Fullwidth parentheses are not legal XML
// name characters for Go's decoder, so the annotation is stripped from tag
// names before decoding. The pattern is anchored to a "<" or "</" plus an
// ASCII identifier so CDATA text content is never touched.
var tagAnnotation = regexp.MustCompile(`(</?[A-Za-z0-9_]+)（[^）]*）`)

// feed mirrors the upstream <DataList><Data>…</Data></DataList> envelope
// after annotation stripping.
type feed struct {
	XMLName xml.Name `xml:"DataList"`
	Jobs    []Job    `xml:"Data"`
}

// parseFeed decodes the upstream XML and normalizes compact dates.
func parseFeed(r io.Reader) ([]Job, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("taiwanjobs: read feed: %w", err)
	}
	raw = tagAnnotation.ReplaceAll(raw, []byte("$1"))
	var f feed
	if err := xml.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("taiwanjobs: decode feed: %w", err)
	}
	for i := range f.Jobs {
		f.Jobs[i].ApplyDeadline = isoDate(f.Jobs[i].ApplyDeadline)
		f.Jobs[i].UpdatedAt = isoDate(f.Jobs[i].UpdatedAt)
	}
	return f.Jobs, nil
}

// isoDate rewrites the feed's compact YYYYMMDD dates as ISO 8601 YYYY-MM-DD,
// the repo-wide date convention. Anything else passes through untouched.
func isoDate(s string) string {
	if len(s) != 8 {
		return s
	}
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return s
		}
	}
	return s[:4] + "-" + s[4:6] + "-" + s[6:]
}
