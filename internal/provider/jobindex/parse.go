package jobindex

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const stashMarker = "var Stash = "

var reJobID = regexp.MustCompile(`(?i)^[a-z]\d+$`)

// parseSearchHTML extracts Stash searchResponse and returns it with upstream
// field names. Only the per-result "html" card markup is removed.
//
// Stash extraction (marker scan + JSON decode + nested searchResponse walk)
// follows the Jobindex skill helpers in:
//
//	https://github.com/MadsLorentzen/ai-job-search/blob/dd6d7efea6c9d0c0d439871c5fc323e57b6a1f58/.agents/skills/jobindex-search/cli/src/helpers.ts
//
// (extractStash / parseSearchPage / findSearchResponse; see comments there on
// /jobsoegning.json returning 204 and results living in var Stash.)
func parseSearchHTML(r io.Reader, pageNum int) (*SearchResponse, error) {
	stash, err := extractStash(r)
	if err != nil {
		return nil, err
	}
	sr := findSearchResponse(stash)
	if sr == nil {
		return nil, fmt.Errorf("jobindex: searchResponse not found in Stash")
	}

	hitcount, _ := asInt(sr["hitcount"])
	// total_pages is only set when present upstream; do not synthesize it under
	// the same field name (callers must not confuse derived values with Stash).
	totalPages, _ := asInt(sr["total_pages"])
	rawResults, _ := sr["results"].([]any)
	results := make([]map[string]any, 0, len(rawResults))
	for _, item := range rawResults {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		// Drop card HTML only; keep every other upstream key as-is.
		out := make(map[string]any, len(m))
		for k, v := range m {
			if k == "html" {
				continue
			}
			out[k] = v
		}
		results = append(results, out)
	}
	if pageNum < 1 {
		pageNum = 1
	}
	return &SearchResponse{
		Hitcount:   hitcount,
		TotalPages: totalPages,
		Results:    results,
		Page:       pageNum,
	}, nil
}

// extractStash scans the search HTML stream for the `var Stash = ` marker and
// decodes the JSON object that follows. json.Decoder stops at the closing
// brace, so the trailing `;</script>` and the rendered page tail are never
// read.
func extractStash(r io.Reader) (map[string]any, error) {
	br := bufio.NewReader(r)
	if err := skipToMarker(br, stashMarker); err != nil {
		return nil, err
	}
	var stash map[string]any
	if err := json.NewDecoder(br).Decode(&stash); err != nil {
		return nil, fmt.Errorf("jobindex: parse Stash JSON: %w", err)
	}
	return stash, nil
}

// skipToMarker consumes br until marker has been read, leaving br positioned
// just past it. The single-byte fallback on mismatch is exact only because
// marker's first byte never recurs later in it.
func skipToMarker(br *bufio.Reader, marker string) error {
	matched := 0
	for {
		b, err := br.ReadByte()
		if err != nil {
			return fmt.Errorf("jobindex: Stash blob not found")
		}
		switch b {
		case marker[matched]:
			matched++
			if matched == len(marker) {
				return nil
			}
		case marker[0]:
			matched = 1
		default:
			matched = 0
		}
	}
}

// findSearchResponse walks the Stash tree for searchResponse.results.
func findSearchResponse(node any) map[string]any {
	switch n := node.(type) {
	case map[string]any:
		if sr, ok := n["searchResponse"].(map[string]any); ok {
			if _, has := sr["results"]; has {
				return sr
			}
		}
		for _, v := range n {
			if found := findSearchResponse(v); found != nil {
				return found
			}
		}
	case []any:
		for _, v := range n {
			if found := findSearchResponse(v); found != nil {
				return found
			}
		}
	}
	return nil
}

func parseDetailHTML(r io.Reader, tid string) (*JobDetail, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, fmt.Errorf("jobindex: parse detail HTML: %w", err)
	}

	d := &JobDetail{Tid: tid}
	og := metaProperties(doc)

	if t := og["og:title"]; t != "" {
		d.Headline = t
	}
	if u := og["og:url"]; u != "" {
		d.ShareURL = u
	}
	if desc := og["og:description"]; desc != "" {
		d.Description = desc
	}

	if d.Headline == "" {
		h1 := strings.TrimSpace(doc.Find("h1").First().Text())
		h1 = strings.TrimPrefix(h1, "Job ad: ")
		h1 = strings.TrimPrefix(h1, "Jobannonce: ")
		d.Headline = strings.TrimSpace(h1)
	}
	if d.Headline == "" {
		return nil, fmt.Errorf("jobindex: could not parse job headline (not a job page?)")
	}

	company := map[string]any{}
	if a := doc.Find(".jix-toolbar-top__company a").First(); a.Length() > 0 {
		if name := strings.TrimSpace(a.Text()); name != "" {
			company["name"] = name
		}
		if href, ok := a.Attr("href"); ok && href != "" {
			company["homeurl"] = href
		}
	}
	if len(company) > 0 {
		d.Company = company
	}

	d.Area = strings.TrimSpace(doc.Find("span.jix_robotjob--area").First().Text())
	if t := doc.Find("time[datetime]").First(); t.Length() > 0 {
		if dt, ok := t.Attr("datetime"); ok {
			d.Firstdate = strings.TrimSpace(dt)
			// Keep full attribute value; if ISO datetime, leave as-is (upstream
			// search firstdate is often YYYY-MM-DD only).
			if len(d.Firstdate) >= 10 && d.Firstdate[4] == '-' {
				d.Firstdate = d.Firstdate[:10]
			}
		}
	}

	if a := doc.Find("a.seejobdesktop, a.seejobmobil").First(); a.Length() > 0 {
		if href, ok := a.Attr("href"); ok {
			d.ApplyURL = href
		}
	}

	var paras []string
	// PaidJob (hosted ads) and jix_robotjob-inner (aggregated r* ads).
	doc.Find("div.PaidJob p, div.jix_robotjob-inner p").Each(func(_ int, s *goquery.Selection) {
		t := strings.TrimSpace(s.Text())
		if t != "" {
			paras = append(paras, t)
		}
	})
	if body := strings.Join(paras, "\n\n"); len(body) > len(d.Description) {
		d.Description = body
	}

	doc.Find(".jix-info p").Each(func(_ int, s *goquery.Selection) {
		label := strings.ToLower(strings.TrimSpace(s.Find("b").First().Text()))
		val := strings.TrimSpace(strings.TrimPrefix(s.Text(), s.Find("b").First().Text()))
		val = strings.TrimSpace(val)
		switch {
		case strings.Contains(label, "ansættelsestype") || strings.Contains(label, "employment"):
			d.EmploymentType = val
		case strings.Contains(label, "arbejdstid") || strings.Contains(label, "working time"):
			d.Hours = val
		case strings.Contains(label, "ansøgningsfrist") || strings.Contains(label, "deadline"):
			// Only when the page literally labels a deadline — do not invent ASAP.
			d.ApplyDeadline = val
		}
	})

	if d.Tid == "" || !reJobID.MatchString(d.Tid) {
		if extracted := tidFromURL(d.ShareURL); extracted != "" {
			d.Tid = extracted
		}
	}
	if d.ShareURL == "" && d.Tid != "" {
		d.ShareURL = "https://www.jobindex.dk/vis-job/" + d.Tid
	}
	return d, nil
}

// metaProperties reads Open Graph (and other) meta tags via goquery.
func metaProperties(doc *goquery.Document) map[string]string {
	out := make(map[string]string)
	doc.Find("meta[property]").Each(func(_ int, s *goquery.Selection) {
		prop, _ := s.Attr("property")
		content, _ := s.Attr("content")
		if prop != "" && content != "" {
			out[prop] = content
		}
	})
	return out
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case json.Number:
		i, err := n.Int64()
		return int(i), err == nil
	case string:
		s := strings.ReplaceAll(n, ".", "")
		var i int
		_, err := fmt.Sscanf(s, "%d", &i)
		return i, err == nil
	default:
		return 0, false
	}
}
