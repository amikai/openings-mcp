package synopsys

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

func parseSearchResults(resultsHTML string) (*JobsResponse, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(resultsHTML))
	if err != nil {
		return nil, err
	}

	var result JobsResponse

	if section := doc.Find("section#search-results").First(); section.Length() > 0 {
		result.TotalResults, _ = strconv.Atoi(section.AttrOr("data-total-results", ""))
		result.TotalPages, _ = strconv.Atoi(section.AttrOr("data-total-pages", ""))
		result.CurrentPage, _ = strconv.Atoi(section.AttrOr("data-current-page", ""))
	}

	for _, li := range doc.Find("li.search-results-list__list-item").EachIter() {
		if job, ok := parseJobCard(li); ok {
			result.Jobs = append(result.Jobs, job)
		}
	}

	return &result, nil
}

func parseJobCard(li *goquery.Selection) (Job, bool) {
	var job Job

	if a := li.Find("a.sr-job-link").First(); a.Length() > 0 {
		href, _ := a.Attr("href")
		// href = /job/{city}/{slug}/44408/{jobID}
		parts := strings.Split(strings.TrimPrefix(href, "/job/"), "/")
		if len(parts) >= 4 {
			job.City = parts[0]
			job.Slug = parts[1]
			job.JobID = parts[3]
		}
	}

	job.Title = strings.TrimSpace(li.Find("h2").First().Text())
	job.Location = strings.TrimSpace(li.Find("span.job-location").First().Text())
	job.Category = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(li.Find("span.category").First().Text()), "Category:"))
	job.Posted = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(li.Find("span.job-date-posted").First().Text()), "Posted:"))
	job.DisplayID = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(li.Find("span.jobId").First().Text()), "Job ID:"))

	return job, job.JobID != ""
}

var jsonLDRe = regexp.MustCompile(`(?s)<script[^>]+application/ld\+json[^>]*>(.*?)</script>`)

func parseJobDetail(r io.Reader) (*JobDetailResponse, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	// xq -q "script[type='application/ld+json']" --html | jq '{title, datePosted, identifier, jobLocation}'
	m := jsonLDRe.FindSubmatch(body)
	if m == nil {
		return nil, errors.New("no JSON-LD found")
	}

	var ld struct {
		Title       string `json:"title"`
		DatePosted  string `json:"datePosted"`
		Identifier  string `json:"identifier"`
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

	return &JobDetailResponse{
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
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return
	}

	// xq -q "div.ats-description" --html
	atsDesc := doc.Find("div.ats-description").First()
	if atsDesc.Length() == 0 {
		return
	}

	// Direct children:
	//   1. skip leading span.job-info
	//   2. first h3 = metadata container → extract category/hireType/remote
	//   3. rest = job description
	firstH3Seen := false
	var descSB strings.Builder
	for _, c := range atsDesc.Contents().EachIter() {
		n := c.Nodes[0]
		if n.Type == html.ElementNode && n.Data == "span" &&
			strings.Contains(c.AttrOr("class", ""), "job-info") {
			continue
		}
		if n.Type == html.ElementNode && n.Data == "h3" && !firstH3Seen {
			firstH3Seen = true
			for _, s := range c.Find("span").EachIter() {
				class := s.AttrOr("class", "")
				text := strings.TrimSpace(s.Text())
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
			continue
		}
		appendNodeText(&descSB, n)
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
