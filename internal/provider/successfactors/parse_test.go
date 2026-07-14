package successfactors

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSearchHTML(t *testing.T) {
	f, err := os.Open("testdata/search_rsp.html")
	require.NoError(t, err)
	defer f.Close()

	doc, err := goquery.NewDocumentFromReader(f)
	require.NoError(t, err)

	jobs, total, err := parseSearchHTML(doc)
	require.NoError(t, err)
	assert.Equal(t, 633, total)
	require.Len(t, jobs, 25)
	assert.Equal(t, Job{ID: "1414343333", Title: "Developer Associate", Location: "Bangalore, IN, 560066"}, jobs[0])
}

// parseResultsTotal must not hardcode the English connector word between
// the two <b> tags: RWE's site defaults to German ("Ergebnisse <b>1 – 25</b>
// von <b>205</b>"), observed live returning total=0 before this was fixed
// to just take the last <b>-tagged number regardless of locale text.
func TestParseSearchHTMLGermanLocale(t *testing.T) {
	const page = `<html><body>
<span class="keywordsearch-icon"></span>
<span class="paginationLabel" aria-label="Ergebnisse 1 – 25">Ergebnisse <b>1 – 25</b> von <b>205</b></span>
</body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(page))
	require.NoError(t, err)

	_, total, err := parseSearchHTML(doc)
	require.NoError(t, err)
	assert.Equal(t, 205, total)
}

// A tenant that configures no department dropdown at all (observed live on
// Borealis and E.ON) must not be mistaken for an unrecognized page — only
// the keyword search icon is a reliable "this is the search form" signal.
func TestParseSearchHTMLNoDepartmentDropdown(t *testing.T) {
	const page = `<html><body>
<span class="keywordsearch-icon"></span>
<select id="optionsFacetsDD_country"></select>
</body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(page))
	require.NoError(t, err)

	jobs, total, err := parseSearchHTML(doc)
	require.NoError(t, err)
	assert.Empty(t, jobs)
	assert.Equal(t, 0, total)
}

// A genuine zero-result page keeps the search form (and its department
// dropdown) but has no result rows and no pagination label; that must read
// as an empty search, not an error.
func TestParseSearchHTMLNoResults(t *testing.T) {
	f, err := os.Open("testdata/search_no_results_rsp.html")
	require.NoError(t, err)
	defer f.Close()

	doc, err := goquery.NewDocumentFromReader(f)
	require.NoError(t, err)

	jobs, total, err := parseSearchHTML(doc)
	require.NoError(t, err)
	assert.Empty(t, jobs)
	assert.Equal(t, 0, total)
}

func TestParseSearchHTMLUnrecognizedPageErrors(t *testing.T) {
	const page = `<html><body>unusual response</body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(page))
	require.NoError(t, err)

	_, _, err = parseSearchHTML(doc)
	require.Error(t, err)
}

func TestParseJobDetailHTML(t *testing.T) {
	f, err := os.Open("testdata/job_detail_rsp.html")
	require.NoError(t, err)
	defer f.Close()

	doc, err := goquery.NewDocumentFromReader(f)
	require.NoError(t, err)

	got, ok := parseJobDetailHTML(doc, "1414343333")
	require.True(t, ok)
	assert.Equal(t, "Developer Associate", got.Title)
	assert.Equal(t, "Bangalore, IN, 560066", got.Location)
	assert.Equal(t, "SAP", got.Employer)
	assert.NotEmpty(t, got.PostedAtRaw)
	assert.Contains(t, got.DescriptionHTML, "Application Engineering")
}

// The errorpage a not-found job ID redirects to (see openapi.yaml) renders
// 200 but carries no itemprop="title" marker at all.
func TestParseJobDetailHTMLNotFound(t *testing.T) {
	f, err := os.Open("testdata/job_detail_not_found_rsp.html")
	require.NoError(t, err)
	defer f.Close()

	doc, err := goquery.NewDocumentFromReader(f)
	require.NoError(t, err)

	_, ok := parseJobDetailHTML(doc, "999999999")
	assert.False(t, ok)
}

// Some tenants (e.g. adidas, Eastman) omit the location/employer/date meta
// tags entirely from their detail-page template; those fields must come
// back empty rather than erroring.
func TestParseJobDetailHTMLMinimalTemplate(t *testing.T) {
	const page = `<html><body><div class="jobDisplayShell" itemscope itemtype="http://schema.org/JobPosting">
<h1><span itemprop="title">Senior Manager, Materials Innovation Scaling</span></h1>
<span itemprop="description"><span class="jobdescription"><p>Job summary.</p></span></span>
</div></body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(page))
	require.NoError(t, err)

	got, ok := parseJobDetailHTML(doc, "1382210733")
	require.True(t, ok)

	want := &JobDetailResponse{
		ID:              "1382210733",
		Title:           "Senior Manager, Materials Innovation Scaling",
		DescriptionHTML: "<p>Job summary.</p>",
	}
	assert.Equal(t, want, got)
}

func TestFacetValuesJSON(t *testing.T) {
	f, err := os.Open("testdata/facet_values_rsp.json")
	require.NoError(t, err)
	defer f.Close()

	var raw facetValuesJSON
	require.NoError(t, json.NewDecoder(f).Decode(&raw))

	got := raw.toResponse()
	require.Contains(t, got.Facets, "country")
	assert.Contains(t, got.Facets["country"], FacetOption{Name: "DE", Translated: "Germany", Count: 192})
}

// facetValues can return an empty facets.map on tenants/queries with no
// configured or matching filter dimensions (observed live on adidas and
// Eastman); that must decode as an empty, non-nil map, not an error.
func TestFacetValuesJSONEmpty(t *testing.T) {
	var raw facetValuesJSON
	require.NoError(t, json.Unmarshal([]byte(`{"facets":{"map":{}}}`), &raw))

	got := raw.toResponse()
	assert.Empty(t, got.Facets)
}
