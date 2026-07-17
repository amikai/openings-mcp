package jobindex

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

var (
	reStashMarker = []byte("var Stash = ")
	reMetaContent = regexp.MustCompile(`(?i)<meta[^>]+content="([^"]*)"[^>]+property="([^"]+)"|<meta[^>]+property="([^"]+)"[^>]+content="([^"]*)"`)
	reJobID       = regexp.MustCompile(`(?i)^[a-z]\d+$`)
)

// parseSearchHTML extracts Stash searchResponse and returns it with upstream
// field names. Only the per-result "html" card markup is removed.
//
// Stash extraction (marker + brace-balanced JSON + nested searchResponse walk)
// follows the Jobindex skill in ai-job-search:
//
//	case-study/ai-job-search/.agents/skills/jobindex-search/cli/src/helpers.ts
//	https://github.com/MadsLorentzen/ai-job-search/blob/dd6d7efea6c9d0c0d439871c5fc323e57b6a1f58/.agents/skills/jobindex-search/cli/src/helpers.ts
//
// (extractStash / parseSearchPage / findSearchResponse; see comments there on
// /jobsoegning.json returning 204 and results living in var Stash.)
func parseSearchHTML(page string, pageNum int) (*SearchResponse, error) {
	stash, err := extractStash(page)
	if err != nil {
		return nil, err
	}
	sr := findSearchResponse(stash)
	if sr == nil {
		return nil, fmt.Errorf("jobindex: searchResponse not found in Stash")
	}

	hitcount, _ := asInt(sr["hitcount"])
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
	if totalPages == 0 && hitcount > 0 {
		totalPages = (hitcount + DefaultPageSize - 1) / DefaultPageSize
	}
	return &SearchResponse{
		Hitcount:   hitcount,
		TotalPages: totalPages,
		Results:    results,
		Page:       pageNum,
	}, nil
}

// extractStash pulls the `var Stash = {...}` blob out of the search HTML.
// Port of extractStash in:
//
//	case-study/ai-job-search/.agents/skills/jobindex-search/cli/src/helpers.ts
//	https://github.com/MadsLorentzen/ai-job-search/blob/dd6d7efea6c9d0c0d439871c5fc323e57b6a1f58/.agents/skills/jobindex-search/cli/src/helpers.ts#L86-L115
func extractStash(page string) (map[string]any, error) {
	idx := strings.Index(page, string(reStashMarker))
	if idx < 0 {
		return nil, fmt.Errorf("jobindex: Stash blob not found")
	}
	open := idx + len(reStashMarker)
	end, err := endOfJSONObject(page, open)
	if err != nil {
		return nil, err
	}
	var stash map[string]any
	if err := json.Unmarshal([]byte(page[open:end]), &stash); err != nil {
		return nil, fmt.Errorf("jobindex: parse Stash JSON: %w", err)
	}
	return stash, nil
}

// endOfJSONObject returns the index just past a balanced {...} starting at open,
// respecting JSON string escapes. Same brace/string walk as extractStash in
// the ai-job-search helpers.ts linked above.
func endOfJSONObject(s string, open int) (int, error) {
	if open >= len(s) || s[open] != '{' {
		return 0, fmt.Errorf("jobindex: Stash does not start with '{'")
	}
	depth := 0
	inStr := false
	esc := false
	for i := open; i < len(s); i++ {
		c := s[i]
		if inStr {
			if esc {
				esc = false
			} else if c == '\\' {
				esc = true
			} else if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1, nil
			}
		}
	}
	return 0, fmt.Errorf("jobindex: unterminated Stash blob")
}

// findSearchResponse walks the Stash tree for searchResponse.results, matching
// findSearchResponse in helpers.ts (same GitHub blob as extractStash above).
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

func parseDetailHTML(page, tid string) (*JobDetail, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(page))
	if err != nil {
		return nil, fmt.Errorf("jobindex: parse detail HTML: %w", err)
	}

	d := &JobDetail{Tid: tid}
	og := metaProperties(page)

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

func metaProperties(page string) map[string]string {
	out := make(map[string]string)
	for _, m := range reMetaContent.FindAllStringSubmatch(page, -1) {
		var prop, content string
		if m[2] != "" {
			content, prop = html.UnescapeString(m[1]), m[2]
		} else {
			prop, content = m[3], html.UnescapeString(m[4])
		}
		if prop != "" && content != "" {
			out[prop] = content
		}
	}
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
