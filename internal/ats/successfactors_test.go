package ats

import (
	"net/http"
	"net/http/httptest"
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

// TestSuccessFactorsFilterMultipleValuesReturnsSupersetWithoutFanout proves
// multi-value input does not cause several upstream requests. SuccessFactors
// supports only one value per dropdown, so the multi-value country filter is
// omitted and the broader result is returned for the caller to filter.
func TestSuccessFactorsFilterMultipleValuesReturnsSupersetWithoutFanout(t *testing.T) {
	searchRequests := 0
	facetRequests := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/services/jobs/options/facetValues/", func(w http.ResponseWriter, r *http.Request) {
		facetRequests++
		w.Write([]byte(`{"facets":{"map":{}}}`))
	})
	mux.HandleFunc("/search/", func(w http.ResponseWriter, r *http.Request) {
		searchRequests++
		w.Header().Set("Content-Type", "text/html")
		if got := r.URL.Query().Get("optionsFacetsDD_country"); got != "" {
			t.Errorf("country filter = %q, want omitted for multi-value input", got)
		}
		w.Write([]byte(successFactorsSupersetFixture))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	a := NewSuccessFactorsAdapter(&http.Client{Timeout: 5 * time.Second})
	a.baseURL = func(string) string { return srv.URL }

	res, err := a.Search(t.Context(), "jobs.sap.com", SearchParams{
		Filters: FilterSet{"country": {"Germany", "United States"}},
	})
	require.NoError(t, err)
	assert.Len(t, res.Jobs, 3)
	assert.Equal(t, 3, res.TotalCount)
	assert.Equal(t, 1, searchRequests)
	assert.Zero(t, facetRequests)
}

const successFactorsSupersetFixture = `<html><body>
<span class="keywordsearch-icon"></span>
<span class="paginationLabel">Results <b>1 – 3</b> of <b>3</b></span>
<table><tbody>
<tr class="data-row">
<td class="colTitle"><a class="jobTitle-link" href="/job/x/1000000001/">Berlin Job</a></td>
<td class="colLocation"><span class="jobLocation">Berlin, DE</span></td>
</tr>
<tr class="data-row">
<td class="colTitle"><a class="jobTitle-link" href="/job/x/1000000002/">Austin Job</a></td>
<td class="colLocation"><span class="jobLocation">Austin, US</span></td>
</tr>
<tr class="data-row">
<td class="colTitle"><a class="jobTitle-link" href="/job/x/1000000003/">Tokyo Job</a></td>
<td class="colLocation"><span class="jobLocation">Tokyo, JP</span></td>
</tr>
</tbody></table>
</body></html>`

func TestSuccessFactorsMultiValueFilterKeepsSingleValueFilters(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/services/jobs/options/facetValues/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"facets":{"map":{
			"country":[{"translated":"Germany","name":"DE"},{"translated":"United States","name":"US"}],
			"department":[{"translated":"Engineering","name":"ENG"}]
		}}}`))
	})
	mux.HandleFunc("/search/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		assert.Empty(t, r.URL.Query().Get("optionsFacetsDD_country"))
		assert.Equal(t, "ENG", r.URL.Query().Get("optionsFacetsDD_department"))
		w.Write([]byte(successFactorsSupersetFixture))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	a := NewSuccessFactorsAdapter(&http.Client{Timeout: 5 * time.Second})
	a.baseURL = func(string) string { return srv.URL }
	res, err := a.Search(t.Context(), "jobs.sap.com", SearchParams{
		Filters: FilterSet{
			"country":    {"Germany", "United States"},
			"department": {"Engineering"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 3, res.TotalCount)
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
