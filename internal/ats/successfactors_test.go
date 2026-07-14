package ats

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/successfactors"
)

// testSuccessFactorsAdapter points the adapter at the mock server, which was
// captured from jobs.sap.com. Company in JobDetail comes from the roster
// lookup, so tests can address it through any roster slug; only "SAP" gets
// exact fixture-content assertions (title, location, description).
func testSuccessFactorsAdapter(t *testing.T) *SuccessFactorsAdapter {
	t.Helper()
	mock := successfactors.NewMockServer()
	t.Cleanup(mock.Close)
	a := NewSuccessFactorsAdapter(&http.Client{Timeout: 5 * time.Second})
	a.baseURL = func(string) string { return mock.URL }
	return a
}

func TestSuccessFactorsRosterBuildsRegistry(t *testing.T) {
	_, err := NewRegistry(NewSuccessFactorsAdapter(http.DefaultClient))
	require.NoError(t, err)
}

func TestSuccessFactorsRosterReturnsCompanyNames(t *testing.T) {
	a := NewSuccessFactorsAdapter(http.DefaultClient)
	roster := a.Roster()
	require.NotEmpty(t, roster)
	found := false
	for _, c := range roster {
		if c.Slug == "jobs.sap.com" {
			found = true
			assert.Equal(t, "SAP", c.Name)
		}
	}
	assert.True(t, found, "expected jobs.sap.com in roster")
}

func TestSuccessFactorsSearch(t *testing.T) {
	a := testSuccessFactorsAdapter(t)
	res, err := a.Search(t.Context(), "jobs.sap.com", SearchParams{Query: "engineer"})
	require.NoError(t, err)
	assert.Equal(t, 633, res.TotalCount)
	assert.Equal(t, 1, res.Page)
	// The upstream table always returns 25 rows; the adapter trims to the
	// unified page size.
	assert.Len(t, res.Jobs, pageSize)

	first := res.Jobs[0]
	assert.Equal(t, "1414343333", first.JobID)
	assert.Equal(t, "Developer Associate", first.Title)
	assert.Equal(t, "Bangalore, IN, 560066", first.Location)
	assert.Equal(t, "https://jobs.sap.com/job/1414343333/1414343333/", first.URL)
}

func TestSuccessFactorsSearchNoResults(t *testing.T) {
	a := testSuccessFactorsAdapter(t)
	res, err := a.Search(t.Context(), "jobs.sap.com", SearchParams{Query: "zzzznonexistentkeyword12345"})
	require.NoError(t, err)
	assert.Empty(t, res.Jobs)
	assert.Equal(t, 0, res.TotalCount)
}

func TestSuccessFactorsFilters(t *testing.T) {
	a := testSuccessFactorsAdapter(t)
	fs, err := a.Filters(t.Context(), "jobs.sap.com")
	require.NoError(t, err)
	require.NotEmptyf(t, fs["country"], "FilterSet missing expected dimension: %v", fs)
	assert.Contains(t, fs["country"], "Germany")
	assert.Contains(t, fs["department"], "Software-Design and Development")
}

// TestSuccessFactorsSearchWithFilterResolvesLabelToValue proves a display
// label ("Germany") is resolved to the upstream's raw value ("DE") via a
// probe facetValues call before the real filtered request.
func TestSuccessFactorsSearchWithFilterResolvesLabelToValue(t *testing.T) {
	a := testSuccessFactorsAdapter(t)
	res, err := a.Search(t.Context(), "jobs.sap.com", SearchParams{
		Query:   "engineer",
		Filters: FilterSet{"country": {"Germany"}, "department": {"Software-Design and Development"}},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, res.Jobs)
}

func TestSuccessFactorsFilterKeyNotFoundTeaches(t *testing.T) {
	a := testSuccessFactorsAdapter(t)
	_, err := a.Search(t.Context(), "jobs.sap.com", SearchParams{
		Filters: FilterSet{"bogus": {"x"}},
	})
	require.ErrorContains(t, err, "country", "error should list valid keys")
}

func TestSuccessFactorsFilterValueNotFoundTeaches(t *testing.T) {
	a := testSuccessFactorsAdapter(t)
	_, err := a.Search(t.Context(), "jobs.sap.com", SearchParams{
		Filters: FilterSet{"country": {"Not A Real Country"}},
	})
	require.ErrorContains(t, err, "Germany", "error should list available values")
}

// TestSuccessFactorsFilterMultipleValuesFansOutAndMerges proves OR
// semantics within one filter key: upstream's optionsFacetsDD_country
// dropdown is single-select, so two OR'd values ("Germany", "United
// States") must become two upstream requests whose results the adapter
// unions and dedupes — not a single request that drops one value, and not
// an error rejecting the valid unified-contract input.
func TestSuccessFactorsFilterMultipleValuesFansOutAndMerges(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/services/jobs/options/facetValues/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"facets":{"map":{"country":[
			{"translated":"Germany","name":"DE","count":1},
			{"translated":"United States","name":"US","count":1}
		]}}}`))
	})
	mux.HandleFunc("/search/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Query().Get("optionsFacetsDD_country") {
		case "DE":
			w.Write([]byte(successFactorsFanoutFixture("1000000001", "Berlin Job", "Berlin, DE")))
		case "US":
			w.Write([]byte(successFactorsFanoutFixture("1000000002", "Austin Job", "Austin, US")))
		default:
			t.Errorf("unexpected country filter in request %s", r.URL)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	a := NewSuccessFactorsAdapter(&http.Client{Timeout: 5 * time.Second})
	a.baseURL = func(string) string { return srv.URL }

	res, err := a.Search(t.Context(), "jobs.sap.com", SearchParams{
		Filters: FilterSet{"country": {"Germany", "United States"}},
	})
	require.NoError(t, err)
	require.Len(t, res.Jobs, 2, "must merge both filter values' results, not just one")
	assert.Equal(t, 2, res.TotalCount)
	ids := []string{res.Jobs[0].JobID, res.Jobs[1].JobID}
	assert.ElementsMatch(t, []string{"1000000001", "1000000002"}, ids)
}

func TestFilterCombinationsRejectsOversizedProductBeforeExpansion(t *testing.T) {
	values := make([]string, maxFilterCombinations+1)
	for i := range values {
		values[i] = fmt.Sprintf("value-%d", i)
	}

	_, err := filterCombinations(map[string][]string{"country": values})
	require.ErrorContains(t, err, "more than 12 combinations")
}

func TestFilterCombinationsDeduplicatesValues(t *testing.T) {
	got, err := filterCombinations(map[string][]string{
		"country": {"DE", "DE", "US"},
	})
	require.NoError(t, err)
	assert.Equal(t, []map[string]string{{"country": "DE"}, {"country": "US"}}, got)
}

func TestSuccessFactorsFanoutDoesNotFetchAllLargeCombinations(t *testing.T) {
	requests := make(map[string][]string)
	mux := http.NewServeMux()
	mux.HandleFunc("/services/jobs/options/facetValues/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"facets":{"map":{"customfield3":[
			{"name":"Professional","count":300},
			{"name":"Student","count":1}
		]}}}`))
	})
	mux.HandleFunc("/search/", func(w http.ResponseWriter, r *http.Request) {
		filter := r.URL.Query().Get("optionsFacetsDD_customfield3")
		startRow := r.URL.Query().Get("startrow")
		requests[filter] = append(requests[filter], startRow)
		start, err := strconv.Atoi(startRow)
		if err != nil {
			t.Errorf("invalid startrow %q", startRow)
		}
		w.Header().Set("Content-Type", "text/html")
		switch filter {
		case "Professional":
			w.Write([]byte(successFactorsFanoutPageFixture(start, min(25, 300-start), 300, "Professional")))
		case "Student":
			w.Write([]byte(successFactorsFanoutPageFixture(start, min(1, 1-start), 1, "Student")))
		default:
			t.Errorf("unexpected filter in request %s", r.URL)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	a := NewSuccessFactorsAdapter(&http.Client{Timeout: 5 * time.Second})
	a.baseURL = func(string) string { return srv.URL }
	res, err := a.Search(t.Context(), "jobs.sap.com", SearchParams{
		Filters: FilterSet{"customfield3": {"Professional", "Student"}},
	})
	require.NoError(t, err)
	assert.Equal(t, 301, res.TotalCount)
	assert.Len(t, res.Jobs, pageSize)
	assert.Equal(t, []string{"0"}, requests["Professional"])
	assert.Equal(t, []string{"0"}, requests["Student"])

	res, err = a.Search(t.Context(), "jobs.sap.com", SearchParams{
		Page:    14,
		Filters: FilterSet{"customfield3": {"Professional", "Student"}},
	})
	require.NoError(t, err)
	assert.Equal(t, 301, res.TotalCount)
	assert.Len(t, res.Jobs, pageSize)
	assert.Equal(t, []string{"0", "0", "260"}, requests["Professional"])
	assert.Equal(t, []string{"0", "0"}, requests["Student"])
}

// successFactorsFanoutFixture renders one minimal search-results page
// carrying a single job row, with the markup parseSearchHTML requires: the
// keyword-search icon (the "this is a real search form" sentinel), a
// data-row with the job's title link, and a pagination label reporting one
// total match.
func successFactorsFanoutFixture(id, title, location string) string {
	return `<html><body>
<span class="keywordsearch-icon"></span>
<span class="paginationLabel">Results <b>1 – 1</b> of <b>1</b></span>
<table><tbody>
<tr class="data-row">
<td class="colTitle"><a class="jobTitle-link" href="/job/x/` + id + `/">` + title + `</a></td>
<td class="colLocation"><span class="jobLocation">` + location + `</span></td>
</tr>
</tbody></table>
</body></html>`
}

func successFactorsFanoutPageFixture(start, count, total int, titlePrefix string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<html><body>
<span class="keywordsearch-icon"></span>
<span class="paginationLabel">Results <b>%d – %d</b> of <b>%d</b></span>
<table><tbody>`, start+1, start+count, total)
	for i := start; i < start+count; i++ {
		fmt.Fprintf(&b, `<tr class="data-row">
<td class="colTitle"><a class="jobTitle-link" href="/job/x/%d/">%s %d</a></td>
<td class="colLocation"><span class="jobLocation">Remote</span></td>
</tr>`, 2_000_000_000-i, titlePrefix, i)
	}
	b.WriteString(`</tbody></table></body></html>`)
	return b.String()
}

func TestSuccessFactorsDetail(t *testing.T) {
	a := testSuccessFactorsAdapter(t)
	d, err := a.Detail(t.Context(), "jobs.sap.com", "1414343333")
	require.NoError(t, err)
	assert.Equal(t, "Developer Associate", d.Title)
	assert.Equal(t, "SAP", d.Company, "Company comes from the SAP roster lookup, not the fixture's employer meta tag")
	assert.Equal(t, "Bangalore, IN, 560066", d.Location)
	assert.Equal(t, "2026-07-13", d.PostedAt)
	assert.Equal(t, "https://jobs.sap.com/job/1414343333/1414343333/", d.URL)
	assert.NotContains(t, d.Description, "<p>", "Description should be converted from HTML")
	assert.Contains(t, d.Description, "Application Engineering")
}

func TestSuccessFactorsDetailNotFound(t *testing.T) {
	a := testSuccessFactorsAdapter(t)
	_, err := a.Detail(t.Context(), "jobs.sap.com", "999999999")
	require.ErrorContains(t, err, "not found")
}

// TestSuccessFactorsDetailOperationalFailureIsNotMislabeledNotFound proves
// a 5xx (or any non-parse failure) surfaces as what it is — the original
// error wrapped with context — rather than being collapsed into the same
// "job not found; pass a job_id exactly as returned" message a genuinely
// expired ID gets. Conflating the two would tell a caller retrying after a
// timeout or upstream outage that the job simply doesn't exist.
func TestSuccessFactorsDetailOperationalFailureIsNotMislabeledNotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/job/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	a := NewSuccessFactorsAdapter(&http.Client{Timeout: 5 * time.Second})
	a.baseURL = func(string) string { return srv.URL }

	_, err := a.Detail(t.Context(), "jobs.sap.com", "123")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "not found",
		"a 500 must not be reported as an expired/unknown job id")
	assert.ErrorContains(t, err, "500")
}

func TestSuccessFactorsUnknownSlugTeaches(t *testing.T) {
	a := NewSuccessFactorsAdapter(http.DefaultClient)
	_, err := a.Search(t.Context(), "not-a-tenant.example.com", SearchParams{})
	require.ErrorContains(t, err, "unknown company")
}

func TestSuccessFactorsParseCareersURL(t *testing.T) {
	a := NewSuccessFactorsAdapter(http.DefaultClient)

	slug, ok := a.ParseCareersURL(mustParseURL(t, "https://jobs.sap.com/job/1414343333/1414343333/"))
	require.True(t, ok)
	assert.Equal(t, "jobs.sap.com", slug)

	// An uncurated custom domain can't be recognized: SuccessFactors CSB
	// tenants share no common host pattern to match against (see
	// SuccessFactorsAdapter's doc comment).
	_, ok = a.ParseCareersURL(mustParseURL(t, "https://jobs.some-other-tenant.com/search/"))
	assert.False(t, ok)

	_, ok = a.ParseCareersURL(mustParseURL(t, "https://jobs.lever.co/acme"))
	assert.False(t, ok)
}
