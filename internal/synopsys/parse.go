package synopsys

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

func parseSearchResults(resultsHTML string) (*SearchResults, error) {
	doc, err := html.Parse(strings.NewReader(resultsHTML))
	if err != nil {
		return nil, err
	}

	var result SearchResults

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch {
			case n.Data == "section" && attrVal(n, "id") == "search-results":
				// xq -q "section#search-results" -a "data-total-results" --html
				// xq -q "section#search-results" -a "data-total-pages" --html
				// xq -q "section#search-results" -a "data-current-page" --html
				result.TotalResults, _ = strconv.Atoi(attrVal(n, "data-total-results"))
				result.TotalPages, _ = strconv.Atoi(attrVal(n, "data-total-pages"))
				result.CurrentPage, _ = strconv.Atoi(attrVal(n, "data-current-page"))

			case n.Data == "li" && strings.Contains(attrVal(n, "class"), "search-results-list__list-item"):
				// xq -q "li.search-results-list__list-item" --html
				if job, ok := parseJobCard(n); ok {
					result.Jobs = append(result.Jobs, job)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return &result, nil
}

func parseJobCard(li *html.Node) (Job, bool) {
	var job Job

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "a":
				if strings.Contains(attrVal(n, "class"), "sr-job-link") {
					// xq -q "li.search-results-list__list-item a.sr-job-link" -a "href" --html
					href := attrVal(n, "href")
					// href = /job/{city}/{slug}/44408/{jobID}
					parts := strings.Split(strings.TrimPrefix(href, "/job/"), "/")
					if len(parts) >= 4 {
						job.City = parts[0]
						job.Slug = parts[1]
						job.JobID = parts[3]
					}
				}
			case "h2":
				// xq -q "li.search-results-list__list-item h2" --html
				job.Title = strings.TrimSpace(textContent(n))
			case "span":
				class := attrVal(n, "class")
				text := strings.TrimSpace(textContent(n))
				switch {
				case class == "job-location":
					// xq -q "li.search-results-list__list-item span.job-location" --html
					job.Location = text
				case class == "category":
					// xq -q "li.search-results-list__list-item span.category" --html
					job.Category = strings.TrimSpace(strings.TrimPrefix(text, "Category:"))
				case class == "job-date-posted":
					// xq -q "li.search-results-list__list-item span.job-date-posted" --html
					job.Posted = strings.TrimSpace(strings.TrimPrefix(text, "Posted:"))
				case class == "jobId":
					// xq -q "li.search-results-list__list-item span.jobId" --html
					job.DisplayID = strings.TrimSpace(strings.TrimPrefix(text, "Job ID:"))
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(li)

	return job, job.JobID != ""
}

var jsonLDRe = regexp.MustCompile(`(?s)<script[^>]+application/ld\+json[^>]*>(.*?)</script>`)

func parseJobDetail(r io.Reader) (*JobDetail, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	// xq -q "script[type='application/ld+json']" --html | jq '{title, datePosted, identifier, jobLocation}'
	m := jsonLDRe.FindSubmatch(body)
	if m == nil {
		return nil, fmt.Errorf("no JSON-LD found")
	}

	var ld struct {
		Title      string `json:"title"`
		DatePosted string `json:"datePosted"`
		Identifier string `json:"identifier"`
		JobLocation []struct {
			Address struct {
				Locality string `json:"addressLocality"`
				Country  string `json:"addressCountry"`
			} `json:"address"`
		} `json:"jobLocation"`
	}
	if err := json.Unmarshal(m[1], &ld); err != nil {
		return nil, fmt.Errorf("JSON-LD: %w", err)
	}

	var locs []string
	for _, loc := range ld.JobLocation {
		if loc.Address.Locality != "" {
			locs = append(locs, loc.Address.Locality+", "+loc.Address.Country)
		}
	}

	category, hireType, remoteEligible, description := parseAtsDesc(body)

	return &JobDetail{
		Title:          ld.Title,
		DatePosted:     ld.DatePosted,
		Locations:      locs,
		DisplayID:      ld.Identifier,
		Category:       category,
		HireType:       hireType,
		RemoteEligible: remoteEligible,
		Description:    description,
	}, nil
}

func parseAtsDesc(body []byte) (category, hireType, remoteEligible, description string) {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return
	}

	// xq -q "div.ats-description" --html
	var atsDesc *html.Node
	var findAts func(*html.Node)
	findAts = func(n *html.Node) {
		if atsDesc != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "div" &&
			strings.Contains(attrVal(n, "class"), "ats-description") {
			atsDesc = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findAts(c)
		}
	}
	findAts(doc)
	if atsDesc == nil {
		return
	}

	// Direct children:
	//   1. skip leading span.job-info
	//   2. first h3 = metadata container → extract category/hireType/remote
	//   3. rest = job description
	firstH3Seen := false
	var descSB strings.Builder
	for c := atsDesc.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "span" &&
			strings.Contains(attrVal(c, "class"), "job-info") {
			continue
		}
		if c.Type == html.ElementNode && c.Data == "h3" && !firstH3Seen {
			firstH3Seen = true
			var walkH3 func(*html.Node)
			walkH3 = func(n *html.Node) {
				if n.Type == html.ElementNode && n.Data == "span" {
					class := attrVal(n, "class")
					text := strings.TrimSpace(textContent(n))
					switch {
					case strings.Contains(class, "job-category"):
						// xq -q "span.job-category.job-info" --html
						category = strings.TrimSpace(strings.TrimPrefix(text, "Category"))
					case strings.Contains(class, "job-type"):
						// xq -q "span.job-type.job-info" --html
						hireType = strings.TrimSpace(strings.TrimPrefix(text, "Hire Type"))
					case strings.Contains(class, "job-remote"):
						// xq -q "span.job-remote.job-info" --html
						remoteEligible = strings.TrimSpace(strings.TrimPrefix(text, "Remote Eligible"))
					}
				}
				for cc := n.FirstChild; cc != nil; cc = cc.NextSibling {
					walkH3(cc)
				}
			}
			walkH3(c)
			continue
		}
		appendNodeText(&descSB, c)
	}
	description = strings.TrimSpace(strings.ReplaceAll(descSB.String(), "\r", ""))
	return
}

func appendNodeText(sb *strings.Builder, n *html.Node) {
	if n.Type == html.TextNode {
		sb.WriteString(n.Data)
		return
	}
	if n.Type != html.ElementNode {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			appendNodeText(sb, c)
		}
		return
	}
	switch n.Data {
	case "h1", "h2", "h3", "h4", "h5", "p", "li":
		sb.WriteByte('\n')
	case "br":
		sb.WriteByte('\n')
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		appendNodeText(sb, c)
	}
	switch n.Data {
	case "h1", "h2", "h3", "h4", "h5", "p":
		sb.WriteByte('\n')
	}
}

func attrVal(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func textContent(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

func stripHTML(s string) string {
	s = htmlTagRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	return strings.TrimSpace(s)
}
