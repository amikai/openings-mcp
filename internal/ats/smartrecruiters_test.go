package ats

import (
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/smartrecruiters"
)

// recordingQueryProxy forwards every request to inner and records each
// request's path+query, so tests can assert what the adapter sent
// upstream (the workday tests' recordingProxy records POST bodies; the
// SmartRecruiters API is GET-only, so the URL is the whole request).
func recordingQueryProxy(t *testing.T, inner string) (*httptest.Server, *[]string) {
	t.Helper()
	var urls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		urls = append(urls, r.URL.String())
		req, err := http.NewRequestWithContext(r.Context(), r.Method, inner+r.URL.String(), nil)
		if !assert.NoError(t, err, "proxy") {
			return
		}
		rsp, err := http.DefaultClient.Do(req)
		if !assert.NoError(t, err, "proxy") {
			return
		}
		defer rsp.Body.Close()
		w.Header().Set("Content-Type", rsp.Header.Get("Content-Type"))
		w.WriteHeader(rsp.StatusCode)
		io.Copy(w, rsp.Body)
	}))
	t.Cleanup(srv.Close)
	return srv, &urls
}

func testSmartRecruitersAdapter(t *testing.T) (*SmartRecruitersAdapter, *[]string) {
	t.Helper()
	mock := smartrecruiters.NewMockServer()
	t.Cleanup(mock.Close)
	proxy, urls := recordingQueryProxy(t, mock.URL)
	a, err := NewSmartRecruitersAdapter(proxy.URL, &http.Client{Timeout: 5 * time.Second})
	require.NoError(t, err)
	return a, urls
}

// lastQueryParams parses the query string of the most recent upstream call.
func lastQueryParams(t *testing.T, urls []string) url.Values {
	t.Helper()
	require.NotEmpty(t, urls)
	u, err := url.Parse(urls[len(urls)-1])
	require.NoError(t, err)
	return u.Query()
}

func TestSmartRecruitersFilters(t *testing.T) {
	a, _ := testSmartRecruitersAdapter(t)
	fs, err := a.Filters(t.Context(), "equinox")
	require.NoError(t, err)

	assert.Equal(t, []string{"Hybrid", "Onsite", "Remote"}, fs["location_type"])

	deps := fs["department"]
	// 58 departments in the fixture, exactly one archived.
	assert.Len(t, deps, 57)
	assert.Contains(t, deps, "Club - Staff")
	assert.Contains(t, deps, "Club - Sales")
	assert.NotContains(t, deps, "Club - Pilot PT", "archived departments must be excluded")
	assert.True(t, slices.IsSorted(deps), "department labels must be sorted")
}

func TestSmartRecruitersRosterMirrorsProviderRoster(t *testing.T) {
	a, err := NewSmartRecruitersAdapter("https://api.smartrecruiters.com", http.DefaultClient)
	require.NoError(t, err)
	roster := a.Roster()
	require.Len(t, roster, len(smartrecruiters.Companies))
	seen := map[string]bool{}
	for _, c := range roster {
		assert.Equal(t, strings.ToLower(c.Slug), c.Slug, "slug %q must be lowercase", c.Slug)
		require.Falsef(t, seen[c.Slug], "duplicate slug %q in roster", c.Slug)
		seen[c.Slug] = true
	}
	assert.True(t, seen["equinox"], "expected equinox in roster")
}

func TestSmartRecruitersRosterBuildsRegistry(t *testing.T) {
	a, err := NewSmartRecruitersAdapter("https://api.smartrecruiters.com", http.DefaultClient)
	require.NoError(t, err)
	_, err = NewRegistry(a)
	require.NoError(t, err)
}

func TestSmartRecruitersParseCareersURL(t *testing.T) {
	a, err := NewSmartRecruitersAdapter("https://api.smartrecruiters.com", http.DefaultClient)
	require.NoError(t, err)
	tests := []struct {
		name string
		url  string
		slug string
		ok   bool
	}{
		{"roster company", "https://jobs.smartrecruiters.com/Equinox", "equinox", true},
		{"posting page", "https://jobs.smartrecruiters.com/Equinox/744000137225639-female-locker-room-associate-houston", "equinox", true},
		{"non-roster company", "https://jobs.smartrecruiters.com/SomeUnknownCo", "someunknownco", true},
		{"host only", "https://jobs.smartrecruiters.com/", "", false},
		{"other ats", "https://jobs.lever.co/acme", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.url)
			require.NoError(t, err)
			slug, ok := a.ParseCareersURL(u)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.slug, slug)
		})
	}
}

func TestSmartRecruitersSearch(t *testing.T) {
	a, _ := testSmartRecruitersAdapter(t)
	res, err := a.Search(t.Context(), "equinox", SearchParams{})
	require.NoError(t, err)

	assert.Equal(t, 662, res.TotalCount)
	assert.Equal(t, 1, res.Page)
	assert.Equal(t, 34, res.TotalPages) // ceil(662/20)
	require.NotEmpty(t, res.Jobs)

	first := res.Jobs[0]
	assert.Equal(t, "744000137225639", first.JobID)
	assert.Equal(t, "Female Locker Room Associate, Houston", first.Title)
	assert.Equal(t, "Houston, TX, United States", first.Location)
	assert.Equal(t, "2026-07-10", first.PostedAt)
	// Roster casing in the derived public URL; slug-less posting URLs
	// resolve fine on jobs.smartrecruiters.com.
	assert.Equal(t, "https://jobs.smartrecruiters.com/Equinox/744000137225639", first.URL)
}

func TestSmartRecruitersSearchQueryReachesUpstream(t *testing.T) {
	a, _ := testSmartRecruitersAdapter(t)
	// The mock serves the filtered fixture only for exactly q=trainer,
	// proving the query passes through server-side.
	res, err := a.Search(t.Context(), "equinox", SearchParams{Query: "trainer"})
	require.NoError(t, err)
	assert.Equal(t, 138, res.TotalCount)
	assert.Len(t, res.Jobs, 3)
}

func TestSmartRecruitersSearchFoldsLocationIntoQ(t *testing.T) {
	a, urls := testSmartRecruitersAdapter(t)
	_, err := a.Search(t.Context(), "equinox", SearchParams{Query: "trainer", Location: "Houston"})
	require.NoError(t, err)
	assert.Equal(t, "trainer Houston", lastQueryParams(t, *urls).Get("q"))

	_, err = a.Search(t.Context(), "equinox", SearchParams{Location: "  Houston  "})
	require.NoError(t, err)
	assert.Equal(t, "Houston", lastQueryParams(t, *urls).Get("q"))
}

func TestSmartRecruitersSearchPagination(t *testing.T) {
	a, urls := testSmartRecruitersAdapter(t)
	res, err := a.Search(t.Context(), "equinox", SearchParams{Page: 3})
	require.NoError(t, err)
	assert.Equal(t, 3, res.Page)
	q := lastQueryParams(t, *urls)
	assert.Equal(t, "20", q.Get("limit"))
	assert.Equal(t, "40", q.Get("offset"))

	_, err = a.Search(t.Context(), "equinox", SearchParams{Page: math.MaxInt})
	require.ErrorContains(t, err, "too large")
}

func TestSmartRecruitersSearchResolvesDepartmentFilter(t *testing.T) {
	a, urls := testSmartRecruitersAdapter(t)
	_, err := a.Search(t.Context(), "equinox", SearchParams{
		Filters: FilterSet{"department": []string{"Club - Staff", "club - sales"}},
	})
	require.NoError(t, err)
	// Two upstream calls: the departments probe, then the search.
	require.Len(t, *urls, 2)
	// Comma-joined ids OR together (verified live against Equinox).
	assert.Equal(t, "660916,660882", lastQueryParams(t, *urls).Get("department"))
}

func TestSmartRecruitersSearchLocationTypeFilter(t *testing.T) {
	a, urls := testSmartRecruitersAdapter(t)
	_, err := a.Search(t.Context(), "equinox", SearchParams{
		Filters: FilterSet{"location_type": []string{"Remote", "hybrid"}},
	})
	require.NoError(t, err)
	// No departments probe for location_type alone.
	require.Len(t, *urls, 1)
	assert.Equal(t, []string{"REMOTE", "HYBRID"}, lastQueryParams(t, *urls)["locationType"])
}

func TestSmartRecruitersSearchFilterErrors(t *testing.T) {
	a, _ := testSmartRecruitersAdapter(t)

	_, err := a.Search(t.Context(), "equinox", SearchParams{
		Filters: FilterSet{"office": []string{"HQ"}},
	})
	require.ErrorContains(t, err, `unknown filter key "office"; valid keys: department, location_type`)

	_, err = a.Search(t.Context(), "equinox", SearchParams{
		Filters: FilterSet{"department": []string{"Nonexistent Dept"}},
	})
	require.ErrorContains(t, err, `filter value "Nonexistent Dept" not found for "department"`)
	require.ErrorContains(t, err, "…", "long label lists must truncate")

	_, err = a.Search(t.Context(), "equinox", SearchParams{
		Filters: FilterSet{"location_type": []string{"underwater"}},
	})
	require.ErrorContains(t, err, `filter value "underwater" not found for "location_type"; available: Hybrid, Onsite, Remote`)
}

func TestSmartRecruitersSearchUnknownCompanyIsEmptyNotError(t *testing.T) {
	a, _ := testSmartRecruitersAdapter(t)
	res, err := a.Search(t.Context(), smartrecruiters.MockUnknownCompany, SearchParams{})
	require.NoError(t, err)
	assert.Equal(t, 0, res.TotalCount)
	assert.Empty(t, res.Jobs)
}

func TestSmartRecruitersDetail(t *testing.T) {
	a, _ := testSmartRecruitersAdapter(t)
	d, err := a.Detail(t.Context(), "equinox", "744000137225639")
	require.NoError(t, err)

	assert.Equal(t, "744000137225639", d.JobID)
	assert.Equal(t, "Female Locker Room Associate, Houston", d.Title)
	assert.Equal(t, "Equinox", d.Company)
	assert.Equal(t, "Houston, TX, United States", d.Location)
	assert.Equal(t, "2026-07-10", d.PostedAt)
	assert.Equal(t, "https://jobs.smartrecruiters.com/Equinox/744000137225639-female-locker-room-associate-houston", d.URL)

	// All four jobAd sections joined as titled plain-text blocks, HTML
	// stripped.
	assert.Contains(t, d.Description, "Company Description:")
	assert.Contains(t, d.Description, "Job Description:")
	assert.Contains(t, d.Description, "Qualifications:")
	assert.Contains(t, d.Description, "Additional Information:")
	assert.NotContains(t, d.Description, "<p>")
}

func TestSmartRecruitersDetailNotFound(t *testing.T) {
	a, _ := testSmartRecruitersAdapter(t)
	_, err := a.Detail(t.Context(), "equinox", "000000000000")
	require.ErrorContains(t, err, "pass a job_id exactly as returned by the job search")
}

func TestSmartRecruitersCareersHostPatternRegistered(t *testing.T) {
	// The registry only advertises careers-URL shapes for adapters listed
	// in careersHostPatternsByAdapter; a missing entry silently degrades
	// the "unrecognized careers URL" teaching error.
	assert.Contains(t, careersHostPatternsByAdapter, "smartrecruiters")
}

func TestSmartRecruitersResolvesThroughRegistry(t *testing.T) {
	a, err := NewSmartRecruitersAdapter("https://api.smartrecruiters.com", http.DefaultClient)
	require.NoError(t, err)
	r, err := NewRegistry(a)
	require.NoError(t, err)

	got, slug, err := r.Resolve("Equinox")
	require.NoError(t, err)
	assert.Equal(t, "smartrecruiters", got.Name())
	assert.Equal(t, "equinox", slug)

	got, slug, err = r.Resolve("https://jobs.smartrecruiters.com/SomeUnknownCo")
	require.NoError(t, err)
	assert.Equal(t, "smartrecruiters", got.Name())
	assert.Equal(t, "someunknownco", slug)
}
