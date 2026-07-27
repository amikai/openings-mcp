package hrmos

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// pgCountPattern extracts the true total from p.pg-count's
// "全 N 件中 M 件 を表示しています" text (fact 3).
var pgCountPattern = regexp.MustCompile(`全\s*(\d+)\s*件中`)

// parseJobsHTML parses one jobs-listing page. A past-the-end page (fact 14)
// has zero cassettes and no p.pg-count, which is not an error: Total comes
// back 0 and the caller relies on MaxPage / the empty-jobs guard to stop
// paging, not on this function erroring.
func parseJobsHTML(doc *goquery.Document) (*JobsResponse, error) {
	resp := &JobsResponse{MaxPage: 1}

	if countText := doc.Find("p.pg-count").First().Text(); countText != "" {
		m := pgCountPattern.FindStringSubmatch(countText)
		if m == nil {
			return nil, fmt.Errorf("unrecognized search response: pg-count text %q has no total", strings.TrimSpace(countText))
		}
		total, err := strconv.Atoi(m[1])
		if err != nil {
			return nil, fmt.Errorf("unrecognized search response: pg-count total %q is not a number", m[1])
		}
		resp.Total = total
	}

	for _, li := range doc.Find("li.pg-list-cassette").EachIter() {
		job, err := parseJobCassette(li)
		if err != nil {
			return nil, err
		}
		resp.Jobs = append(resp.Jobs, job)
	}

	resp.Facets = parseFacetGroups(doc)
	resp.MaxPage = parseMaxPage(doc)

	return resp, nil
}

// jobIDFromHrefPattern captures the trailing job id segment of a cassette
// or detail-page href: https://hrmos.co/pages/{slug}/jobs/{id}.
var jobIDFromHrefPattern = regexp.MustCompile(`/pages/[^/]+/jobs/([^/?#]+)$`)

// parseJobCassette extracts one li.pg-list-cassette. An empty job ID or an
// href that doesn't match the /pages/{slug}/jobs/{id} shape is a selector-
// drift bug, not a row to silently skip.
func parseJobCassette(li *goquery.Selection) (Job, error) {
	var job Job

	href, ok := li.Find("a").First().Attr("href")
	if !ok {
		return Job{}, errors.New("job cassette has no link")
	}
	job.URL = href
	m := jobIDFromHrefPattern.FindStringSubmatch(href)
	if m == nil || m[1] == "" {
		return Job{}, fmt.Errorf("job cassette href %q does not match /pages/{slug}/jobs/{id}", href)
	}
	job.ID = m[1]

	job.Title = strings.TrimSpace(li.Find("h2").First().Text())
	job.Description = strings.TrimSpace(li.Find("span.pg-list-cassette-body").First().Text())

	for _, chip := range li.Find("ul.sg-tags > li").EachIter() {
		text := strings.TrimSpace(chip.Text())
		job.Chips = append(job.Chips, text)
		if chip.HasClass("sg-tag-location") {
			job.Location = text
		}
	}

	return job, nil
}

// facetAllValues are the data-attribute values HRMOS uses for its "すべて"
// (show all) pseudo-option, which is UI chrome, not a real facet value.
const facetAllValue = "ALL"

// parseFacetGroups reads nav.sg-job-filters: one FacetGroup per <tr> that
// carries a <th> label (fact 4). A tenant may define none — nav.sg-job-
// filters is absent entirely (verified on the hrmos tenant).
func parseFacetGroups(doc *goquery.Document) []FacetGroup {
	var groups []FacetGroup
	for _, tr := range doc.Find("nav.sg-job-filters table tr").EachIter() {
		th := tr.Find("th").First()
		if th.Length() == 0 {
			continue // the search-button row has no <th>
		}
		group := FacetGroup{Label: strings.TrimSpace(th.Text())}
		for _, a := range tr.Find("a.jsc-jobType-trigger, a.jsc-jobCategory-trigger").EachIter() {
			if v, ok := a.Attr("data-job-type"); ok && v == facetAllValue {
				continue
			}
			if v, ok := a.Attr("data-job-category"); ok && v == facetAllValue {
				continue
			}
			group.Options = append(group.Options, strings.TrimSpace(a.Text()))
		}
		groups = append(groups, group)
	}
	return groups
}

// parseMaxPage reads the highest page number nav.pg-pagenation advertises.
// It returns 1 when the nav is absent (a single-page dump). The nav is
// still present and still lists the real page range on a past-the-end page
// (fact 14), which is what makes it the reliable stop condition for AllJobs.
func parseMaxPage(doc *goquery.Document) int {
	maxPage := 1
	for _, li := range doc.Find("nav.pg-pagenation li").EachIter() {
		if n, err := strconv.Atoi(strings.TrimSpace(li.Text())); err == nil && n > maxPage {
			maxPage = n
		}
	}
	return maxPage
}

// jobPostingLD mirrors the detail page's schema.org JobPosting JSON-LD.
// Verified key set (fact 15): @context @type baseSalary datePosted
// description employmentType hiringOrganization identifier jobLocation
// title validThrough — notably no "url" key, unlike Mynavi's schema.
type jobPostingLD struct {
	Type               string             `json:"@type"`
	Title              string             `json:"title"`
	Description        string             `json:"description"`
	EmploymentType     string             `json:"employmentType"`
	DatePosted         string             `json:"datePosted"`
	ValidThrough       string             `json:"validThrough"`
	HiringOrganization organizationLD     `json:"hiringOrganization"`
	JobLocation        oneOrMany[placeLD] `json:"jobLocation"`
	BaseSalary         *baseSalaryLD      `json:"baseSalary"`
}

type organizationLD struct {
	Name   string `json:"name"`
	SameAs string `json:"sameAs"`
}

type placeLD struct {
	Address struct {
		PostalCode string `json:"postalCode"`
		Region     string `json:"addressRegion"`
		Locality   string `json:"addressLocality"`
		Street     string `json:"streetAddress"`
	} `json:"address"`
}

type baseSalaryLD struct {
	Currency string `json:"currency"`
	Value    struct {
		MinValue flexString `json:"minValue"`
		MaxValue flexString `json:"maxValue"`
		UnitText string     `json:"unitText"`
	} `json:"value"`
}

// oneOrMany accepts both a bare JSON object and an array of them.
type oneOrMany[T any] []T

func (o *oneOrMany[T]) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) > 0 && data[0] == '[' {
		return json.Unmarshal(data, (*[]T)(o))
	}
	var one T
	if err := json.Unmarshal(data, &one); err != nil {
		return err
	}
	*o = []T{one}
	return nil
}

// flexString accepts both a JSON string and a bare number.
type flexString string

func (f *flexString) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) > 0 && data[0] == '"' {
		return json.Unmarshal(data, (*string)(f))
	}
	*f = flexString(data)
	return nil
}

// parseJobDetailHTML extracts a posting's JSON-LD plus both pg-descriptions
// tables. rawURL is the request URL, since the JSON-LD carries no "url" key
// (fact 15).
func parseJobDetailHTML(doc *goquery.Document, id, rawURL string) (*JobDetailResponse, error) {
	var posting *jobPostingLD
	for _, script := range doc.Find(`script[type="application/ld+json"]`).EachIter() {
		var ld jobPostingLD
		if err := json.Unmarshal([]byte(script.Text()), &ld); err != nil {
			continue
		}
		if ld.Type == "JobPosting" {
			posting = &ld
			break
		}
	}
	if posting == nil {
		return nil, errors.New("no JobPosting JSON-LD on page")
	}

	detail := &JobDetailResponse{
		ID:             id,
		URL:            rawURL,
		Title:          posting.Title,
		Company:        posting.HiringOrganization.Name,
		CompanyURL:     posting.HiringOrganization.SameAs,
		EmploymentType: posting.EmploymentType,
		DatePosted:     posting.DatePosted,
		ValidThrough:   posting.ValidThrough,
		Description:    htmlToText(posting.Description),
	}
	for _, place := range posting.JobLocation {
		detail.Locations = append(detail.Locations, Location{
			PostalCode: place.Address.PostalCode,
			Region:     place.Address.Region,
			Locality:   place.Address.Locality,
			Street:     place.Address.Street,
		})
	}
	if posting.BaseSalary != nil {
		detail.SalaryCurrency = posting.BaseSalary.Currency
		detail.SalaryMin = string(posting.BaseSalary.Value.MinValue)
		detail.SalaryMax = string(posting.BaseSalary.Value.MaxValue)
		detail.SalaryUnit = posting.BaseSalary.Value.UnitText
	}

	tables := doc.Find("section.pg-descriptions")
	if tables.Length() > 0 {
		jobTable := tables.Eq(0)
		detail.SalaryNote = strings.TrimSpace(jobTable.Find("dl dt").First().Text())
		detail.JobInfo = parseDescriptionTable(jobTable)
	}
	if tables.Length() > 1 {
		detail.CompanyInfo = parseDescriptionTable(tables.Eq(1))
	}

	return detail, nil
}

// parseDescriptionTable flattens one pg-descriptions <table>'s rows to
// ordered key/value pairs, th text -> flattened td text.
func parseDescriptionTable(section *goquery.Selection) []KV {
	var kvs []KV
	for _, row := range section.Find("table tr").EachIter() {
		key := strings.TrimSpace(row.Find("th").First().Text())
		if key == "" {
			continue
		}
		value := strings.TrimSpace(collapseWhitespace(row.Find("td").First().Text()))
		kvs = append(kvs, KV{Key: key, Value: value})
	}
	return kvs
}

var whitespaceRunPattern = regexp.MustCompile(`[ \t]*\n[ \t]*`)

// collapseWhitespace trims leading/trailing space on every line of a
// flattened <td>'s text without discarding the newlines pre-formatted
// content (<pre> blocks) relies on for structure.
func collapseWhitespace(s string) string {
	return whitespaceRunPattern.ReplaceAllString(strings.TrimSpace(s), "\n")
}

// htmlToText flattens an HTML-valued JSON-LD field to plain text: block
// elements and <br> become newlines, entities are decoded, runs of blank
// lines collapse.
func htmlToText(s string) string {
	if s == "" {
		return ""
	}
	nodes, err := html.ParseFragment(strings.NewReader(s), &html.Node{
		Type:     html.ElementNode,
		Data:     "body",
		DataAtom: atom.Body,
	})
	if err != nil {
		return s
	}
	var sb strings.Builder
	for _, n := range nodes {
		appendNodeText(&sb, n)
	}
	return strings.TrimSpace(blankLinePattern.ReplaceAllString(sb.String(), "\n\n"))
}

var blankLinePattern = regexp.MustCompile(`\n{3,}`)

// appendNodeText flattens a node's text, inserting newlines around
// block-level elements so a rich-text field reads as plain text.
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
