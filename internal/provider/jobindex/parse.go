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
	reTimeDT      = regexp.MustCompile(`(?i)<time[^>]+datetime="([^"]+)"`)
	// Job ad IDs look like h1683131 or r13911770.
	reJobID = regexp.MustCompile(`(?i)^[a-z]\d+$`)
)

// parseSearchHTML extracts jobs from a /jobsoegning HTML document's Stash blob.
func parseSearchHTML(page string, pageNum int) (*JobsResponse, error) {
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
	jobs := make([]Job, 0, len(rawResults))
	for _, item := range rawResults {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if j, ok := mapSearchResult(m); ok {
			jobs = append(jobs, j)
		}
	}
	if len(jobs) == 0 && hitcount == 0 && strings.TrimSpace(stripTags(page)) != "" {
		// Empty results are valid; only error if the page looks wrong and we
		// found no searchResponse results array at all — already handled.
	}
	if pageNum < 1 {
		pageNum = 1
	}
	if totalPages == 0 && hitcount > 0 {
		totalPages = (hitcount + DefaultPageSize - 1) / DefaultPageSize
	}
	return &JobsResponse{
		Jobs:       jobs,
		TotalCount: hitcount,
		Page:       pageNum,
		TotalPages: totalPages,
	}, nil
}

func mapSearchResult(m map[string]any) (Job, bool) {
	tid, _ := m["tid"].(string)
	title, _ := m["headline"].(string)
	if tid == "" || title == "" {
		return Job{}, false
	}
	j := Job{
		ID:         tid,
		Title:      title,
		Location:   stringField(m, "area"),
		PostedDate: stringField(m, "firstdate"),
		URL:        jobURL(tid),
	}
	if j.Location == "" {
		j.Location = geoTitle(m["geojson"])
	}
	if company, ok := m["company"].(map[string]any); ok {
		j.Company = stringField(company, "name")
		j.CompanyURL = stringField(company, "homeurl")
	}
	if j.Company == "" {
		j.Company = stringField(m, "companytext")
	}
	if asap, _ := m["apply_deadline_asap"].(bool); asap {
		j.Deadline = "ASAP"
	} else if d := stringField(m, "apply_deadline"); d != "" {
		j.Deadline = truncateDate(d)
	} else if d := stringField(m, "lastdate"); d != "" {
		j.Deadline = truncateDate(d)
	}
	return j, true
}

func jobURL(tid string) string {
	return "https://www.jobindex.dk/vis-job/" + tid
}

func geoTitle(v any) string {
	// geojson.features[0].properties.title
	root, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	features, _ := root["features"].([]any)
	if len(features) == 0 {
		return ""
	}
	feat, _ := features[0].(map[string]any)
	props, _ := feat["properties"].(map[string]any)
	return stringField(props, "title")
}

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
// respecting JSON string escapes.
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

func parseDetailHTML(page, id, fallbackURL string) (*JobDetail, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(page))
	if err != nil {
		return nil, fmt.Errorf("jobindex: parse detail HTML: %w", err)
	}

	d := &JobDetail{ID: id, URL: fallbackURL}

	// Prefer og: meta; fall back to visible markup.
	og := metaProperties(page)
	if t := og["og:title"]; t != "" {
		d.Title = t
	}
	if u := og["og:url"]; u != "" {
		d.URL = u
	}
	if desc := og["og:description"]; desc != "" {
		d.Description = desc
	}

	if d.Title == "" {
		d.Title = strings.TrimSpace(doc.Find("h1").First().Text())
		d.Title = strings.TrimPrefix(d.Title, "Job ad: ")
		d.Title = strings.TrimPrefix(d.Title, "Jobannonce: ")
		d.Title = strings.TrimSpace(d.Title)
	}
	if d.Title == "" {
		return nil, fmt.Errorf("jobindex: could not parse job title (not a job page?)")
	}

	if a := doc.Find(".jix-toolbar-top__company a").First(); a.Length() > 0 {
		d.Company = strings.TrimSpace(a.Text())
		if href, ok := a.Attr("href"); ok {
			d.CompanyURL = href
		}
	}
	d.Location = strings.TrimSpace(doc.Find("span.jix_robotjob--area").First().Text())
	if t := doc.Find("time[datetime]").First(); t.Length() > 0 {
		if dt, ok := t.Attr("datetime"); ok {
			d.PostedDate = truncateDate(dt)
		}
	}

	// Apply / "Se jobbet" deep link.
	if a := doc.Find("a.seejobdesktop, a.seejobmobil").First(); a.Length() > 0 {
		if href, ok := a.Attr("href"); ok {
			d.ApplyURL = href
		}
	}

	// Body paragraphs inside PaidJob (appetizer). Prefer these over og:description when longer.
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

	// Employment / hours / deadline labels in jix-info if present.
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
			d.Deadline = val
		}
	})

	if d.ID == "" || !reJobID.MatchString(d.ID) {
		if tid := tidFromURL(d.URL); tid != "" {
			d.ID = tid
		}
	}
	return d, nil
}

func metaProperties(page string) map[string]string {
	out := make(map[string]string)
	for _, m := range reMetaContent.FindAllStringSubmatch(page, -1) {
		// Two alternate capture groups depending on attribute order.
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

func stringField(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	switch v := m[key].(type) {
	case string:
		return v
	case float64:
		// JSON numbers (unlikely for these fields)
		return fmt.Sprintf("%.0f", v)
	default:
		return ""
	}
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
		// Danish thousands: "18.903"
		s := strings.ReplaceAll(n, ".", "")
		var i int
		_, err := fmt.Sscanf(s, "%d", &i)
		return i, err == nil
	default:
		return 0, false
	}
}

func truncateDate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 10 && s[4] == '-' && s[7] == '-' {
		return s[:10]
	}
	return s
}

func stripTags(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == '<':
			in = true
		case r == '>':
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return b.String()
}
