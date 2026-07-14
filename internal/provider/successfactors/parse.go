package successfactors

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// parseSearchHTML parses the results table (tr.data-row) and the
// "Results X – Y of N" pagination label. A page with the search form but no
// rows and no label is a genuine zero-result search, not a parse failure; a
// page missing the search form entirely is unrecognized and must error.
//
// The sentinel for "the search form rendered" is the keyword box's icon
// (span.keywordsearch-icon), not any one optionsFacetsDD_* filter dropdown:
// which filter dimensions a tenant configures is arbitrary (observed:
// Borealis and E.ON have no department dropdown at all), but every tenant's
// search form keeps the keyword box.
func parseSearchHTML(doc *goquery.Document) ([]Job, int, error) {
	var jobs []Job
	for _, row := range doc.Find("tr.data-row").EachIter() {
		job, ok := parseJobRow(row)
		if !ok {
			continue
		}
		jobs = append(jobs, job)
	}

	total := parseResultsTotal(doc)
	if len(jobs) == 0 && total == 0 && doc.Find(".keywordsearch-icon").Length() == 0 {
		return nil, 0, errors.New("unrecognized search page: no job rows, no results count, and no search form")
	}
	return jobs, total, nil
}

func parseJobRow(row *goquery.Selection) (Job, bool) {
	link := row.Find("a.jobTitle-link").First()
	title := strings.TrimSpace(link.Text())
	href, _ := link.Attr("href")
	id := jobIDFromHref(href)
	if title == "" || id == "" {
		return Job{}, false
	}
	location := strings.TrimSpace(row.Find("td.colLocation span.jobLocation").First().Text())
	return Job{ID: id, Title: title, Location: location}, true
}

// jobIDFromHref extracts the trailing numeric ID from a /job/{slug}/{id}/
// detail link (see openapi.yaml: the slug is cosmetic, only the ID
// resolves the posting).
var jobHrefID = regexp.MustCompile(`/job/[^/]+/(\d+)/?$`)

func jobIDFromHref(href string) string {
	m := jobHrefID.FindStringSubmatch(href)
	if m == nil {
		return ""
	}
	return m[1]
}

// resultsTotalPattern extracts every bold number in the pagination label,
// e.g. `Results <b>1 – 25</b> of <b>633</b>` (English) or
// `Ergebnisse <b>1 – 25</b> von <b>205</b>` (German, observed on RWE's
// default-German site). The connector word between the two <b> tags is
// locale text ("of", "von", ...) that isn't worth matching per-locale; the
// total is always the last <b>-tagged number regardless of language.
var resultsTotalPattern = regexp.MustCompile(`<b>([\d,]+)</b>`)

func parseResultsTotal(doc *goquery.Document) int {
	label := doc.Find("span.paginationLabel").First()
	if label.Length() == 0 {
		return 0
	}
	html, err := label.Html()
	if err != nil {
		return 0
	}
	matches := resultsTotalPattern.FindAllStringSubmatch(html, -1)
	if len(matches) == 0 {
		return 0
	}
	last := matches[len(matches)-1][1]
	n, err := strconv.Atoi(strings.ReplaceAll(last, ",", ""))
	if err != nil {
		return 0
	}
	return n
}

// parseJobDetailHTML parses one detail page. Only title and description are
// guaranteed present across tenants (see openapi.yaml); location, employer,
// and posted date are read best-effort and left empty when the tenant's
// template omits them.
func parseJobDetailHTML(doc *goquery.Document, id string) (*JobDetailResponse, bool) {
	title := strings.TrimSpace(doc.Find(`span[itemprop="title"]`).First().Text())
	if title == "" {
		return nil, false
	}

	descriptionHTML, _ := doc.Find(`span[itemprop="description"] span.jobdescription`).First().Html()

	location, _ := doc.Find(`meta[itemprop="streetAddress"]`).First().Attr("content")
	employer, _ := doc.Find(`meta[itemprop="hiringOrganization"]`).First().Attr("content")
	postedAt, _ := doc.Find(`meta[itemprop="datePosted"]`).First().Attr("content")

	return &JobDetailResponse{
		ID:              id,
		Title:           title,
		Location:        strings.TrimSpace(location),
		Employer:        strings.TrimSpace(employer),
		PostedAtRaw:     strings.TrimSpace(postedAt),
		DescriptionHTML: strings.TrimSpace(descriptionHTML),
	}, true
}

// facetValuesJSON mirrors the facetValues response body; see
// FacetValuesResponse.Facets for the exported shape.
type facetValuesJSON struct {
	Facets struct {
		Map map[string][]struct {
			Translated string `json:"translated"`
			Name       string `json:"name"`
			Count      int    `json:"count"`
		} `json:"map"`
	} `json:"facets"`
}

func (r *facetValuesJSON) toResponse() *FacetValuesResponse {
	out := make(map[string][]FacetOption, len(r.Facets.Map))
	for dimension, options := range r.Facets.Map {
		opts := make([]FacetOption, 0, len(options))
		for _, o := range options {
			opts = append(opts, FacetOption{Name: o.Name, Translated: o.Translated, Count: o.Count})
		}
		out[dimension] = opts
	}
	return &FacetValuesResponse{Facets: out}
}
