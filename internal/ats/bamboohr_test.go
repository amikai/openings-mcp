package ats

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/bamboohr"
)

func testBambooHRAdapter(t *testing.T) *BambooHRAdapter {
	t.Helper()
	srv := bamboohr.NewMockServer()
	t.Cleanup(srv.Close)
	a := NewBambooHRAdapter(&http.Client{Timeout: 5 * time.Second})
	a.baseURL = func(string) string { return srv.URL }
	return a
}

func testBambooHRVarietyAdapter(t *testing.T) *BambooHRAdapter {
	t.Helper()
	srv := bamboohr.NewVarietyMockServer()
	t.Cleanup(srv.Close)
	a := NewBambooHRAdapter(&http.Client{Timeout: 5 * time.Second})
	a.baseURL = func(string) string { return srv.URL }
	return a
}

func TestBambooHRRoster(t *testing.T) {
	a := testBambooHRAdapter(t)
	assert.Len(t, a.Roster(), len(bamboohr.Companies))
}

func TestBambooHRParseCareersURL(t *testing.T) {
	a := testBambooHRAdapter(t)

	tests := []struct {
		name     string
		rawURL   string
		wantSlug string
		wantOK   bool
	}{
		{name: "curated", rawURL: "https://concept2.bamboohr.com/careers/43", wantSlug: "concept2", wantOK: true},
		{
			name:     "unlisted",
			rawURL:   "https://" + bamboohr.MockNonRosterSlug + ".bamboohr.com/careers",
			wantSlug: bamboohr.MockNonRosterSlug,
			wantOK:   true,
		},
		{name: "marketing host", rawURL: "https://www.bamboohr.com/careers/", wantOK: false},
		{name: "docs host", rawURL: "https://documentation.bamboohr.com/reference", wantOK: false},
		{name: "unrelated", rawURL: "https://careers.google.com/jobs", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.rawURL)
			require.NoError(t, err)
			slug, ok := a.ParseCareersURL(u)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantSlug, slug)
		})
	}
}

func TestBambooHRSearchAll(t *testing.T) {
	a := testBambooHRAdapter(t)
	res, err := a.Search(t.Context(), bamboohr.MockSlug, SearchParams{})
	require.NoError(t, err)
	assert.Equal(t, 5, res.TotalCount)
	assert.Len(t, res.Jobs, 5)
	for _, j := range res.Jobs {
		assert.NotEmpty(t, j.JobID)
		assert.NotEmpty(t, j.Title)
		assert.NotEmpty(t, j.Location)
		// The list feed carries no posting date, so summaries can't either.
		assert.Empty(t, j.PostedAt)
		assert.NotEmpty(t, j.URL)
	}
}

func TestBambooHRSearchQueryLocationAndFilters(t *testing.T) {
	a := testBambooHRAdapter(t)

	queried, err := a.Search(t.Context(), bamboohr.MockSlug, SearchParams{Query: "IT Operations"})
	require.NoError(t, err)
	require.NotEmpty(t, queried.Jobs)
	assert.Equal(t, "IT Operations Lead", queried.Jobs[0].Title)

	// Department text matches through the query's organization-unit tier.
	deptQueried, err := a.Search(t.Context(), bamboohr.MockSlug, SearchParams{Query: "SmartMix"})
	require.NoError(t, err)
	require.NotEmpty(t, deptQueried.Jobs)

	// Description-only skill match: "concrete" appears in job 167's detail
	// body (detail_nulls_rsp.json) but not in any list-row field.
	descQueried, err := a.Search(t.Context(), bamboohr.MockSlug, SearchParams{Query: "concrete"})
	require.NoError(t, err)
	require.NotEmpty(t, descQueried.Jobs)
	assert.Equal(t, bamboohr.MockNullsJobID, descQueried.Jobs[0].JobID)

	located, err := a.Search(t.Context(), bamboohr.MockSlug, SearchParams{Location: "Ottawa"})
	require.NoError(t, err)
	assert.NotEmpty(t, located.Jobs)
	for _, j := range located.Jobs {
		assert.Contains(t, j.Location, "Ottawa")
	}

	filtered, err := a.Search(t.Context(), bamboohr.MockSlug, SearchParams{
		Filters: FilterSet{"department": {"Software Development - SmartMix"}},
	})
	require.NoError(t, err)
	require.Len(t, filtered.Jobs, 1)
	assert.Equal(t, "292", filtered.Jobs[0].JobID)
}

// TestBambooHRRemoteAndAtsLocation exercises the variety board: "remote"
// matches rows via locationType, and a row whose `location` is all-null
// renders its display location from atsLocation instead.
func TestBambooHRRemoteAndAtsLocation(t *testing.T) {
	a := testBambooHRVarietyAdapter(t)

	remote, err := a.Search(t.Context(), bamboohr.MockVarietySlug, SearchParams{Location: remoteLocation})
	require.NoError(t, err)
	assert.NotEmpty(t, remote.Jobs)

	all, err := a.Search(t.Context(), bamboohr.MockVarietySlug, SearchParams{Query: "Tugboat Engineer"})
	require.NoError(t, err)
	require.NotEmpty(t, all.Jobs)
	assert.Equal(t, "339", all.Jobs[0].JobID)
	assert.Equal(t, "Long Beach, California, United States", all.Jobs[0].Location)
}

func TestBambooHRSearchRejectsUnknownFilter(t *testing.T) {
	a := testBambooHRAdapter(t)
	_, err := a.Search(t.Context(), bamboohr.MockSlug, SearchParams{
		Filters: FilterSet{"unsupported-bamboohr-filter": {"unused"}},
	})
	require.ErrorContains(t, err, "unknown filter key")
	assert.ErrorContains(t, err, "department, employmentType, workplaceType")
}

func TestBambooHRFilters(t *testing.T) {
	a := testBambooHRAdapter(t)
	filters, err := a.Filters(t.Context(), bamboohr.MockSlug)
	require.NoError(t, err)
	assert.Contains(t, filters["department"], "Software Development - SmartMix")
	assert.Contains(t, filters["employmentType"], "Full-Time")
	assert.Contains(t, filters["workplaceType"], "Hybrid")
}

func TestBambooHRDetail(t *testing.T) {
	a := testBambooHRAdapter(t)
	detail, err := a.Detail(t.Context(), "curtinmaritime", bamboohr.MockJobID)
	require.NoError(t, err)
	assert.Equal(t, "Vessel Chef", detail.Title)
	assert.Equal(t, "Curtin Maritime", detail.Company)
	assert.Equal(t, "Long Beach, California, United States", detail.Location)
	assert.Equal(t, "2025-04-22", detail.PostedAt)
	assert.Equal(t, "https://curtinmaritime.bamboohr.com/careers/201", detail.URL)
	assert.Contains(t, detail.Description, "Curtin Maritime")
	assert.NotContains(t, detail.Description, "<p>")
}

// TestBambooHRDetailAtsLocationFallback covers postings that leave
// jobOpening.location all-null and put the only usable locality on
// atsLocation (observed live on Ashtead job 35).
func TestBambooHRDetailAtsLocationFallback(t *testing.T) {
	const body = `{
  "meta": {"totalCount": 1},
  "result": {
    "jobOpening": {
      "jobOpeningShareUrl": "https://ashteadtechnology.bamboohr.com/careers/35",
      "jobOpeningName": "Workshop Technician",
      "jobOpeningStatus": "Open",
      "jobCategoryId": null,
      "departmentId": null,
      "departmentLabel": "",
      "employmentStatusLabel": "Full-Time",
      "location": {"city": null, "state": null, "postalCode": null, "addressCountry": null},
      "atsLocation": {"country": "United Kingdom", "countryId": "GB", "state": null, "city": null},
      "description": "<p>Workshop role</p>",
      "compensation": null,
      "datePosted": "2025-01-15",
      "minimumExperience": null,
      "locationType": "0",
      "seekPromoted": false
    },
    "formFields": {}
  }
}`
	mux := http.NewServeMux()
	mux.HandleFunc("/careers/35/detail", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	a := NewBambooHRAdapter(&http.Client{Timeout: 5 * time.Second})
	a.baseURL = func(string) string { return srv.URL }

	detail, err := a.Detail(t.Context(), "ashteadtechnology", "35")
	require.NoError(t, err)
	assert.Equal(t, "United Kingdom", detail.Location)
	assert.Equal(t, "Workshop Technician", detail.Title)
}

// TestBambooHRDetailNonRoster covers a URL-resolved tenant outside the
// curated roster: Company falls back to the slug.
func TestBambooHRDetailNonRoster(t *testing.T) {
	a := testBambooHRAdapter(t)
	detail, err := a.Detail(t.Context(), bamboohr.MockNonRosterSlug, bamboohr.MockJobID)
	require.NoError(t, err)
	assert.Equal(t, bamboohr.MockNonRosterSlug, detail.Company)
}

func TestBambooHRDetailNotFound(t *testing.T) {
	a := testBambooHRAdapter(t)
	_, err := a.Detail(t.Context(), bamboohr.MockSlug, "999999")
	assert.ErrorContains(t, err, "pass a job_id exactly as returned")
}

func TestBambooHRSearchEmptyBoard(t *testing.T) {
	srv := bamboohr.NewEmptyMockServer()
	t.Cleanup(srv.Close)
	a := NewBambooHRAdapter(&http.Client{Timeout: 5 * time.Second})
	a.baseURL = func(string) string { return srv.URL }

	res, err := a.Search(t.Context(), "zapier", SearchParams{})
	require.NoError(t, err)
	assert.Zero(t, res.TotalCount)
	assert.Empty(t, res.Jobs)
}

// TestBambooHRUnknownTenantUpstream covers the 302-to-marketing-site
// behavior: the adapter's redirect-blocking client must surface it as a
// "not found upstream" error, not follow it into an HTML decode failure.
func TestBambooHRUnknownTenantUpstream(t *testing.T) {
	srv := bamboohr.NewRedirectMockServer()
	t.Cleanup(srv.Close)
	a := NewBambooHRAdapter(&http.Client{Timeout: 5 * time.Second})
	a.baseURL = func(string) string { return srv.URL }

	_, err := a.Search(t.Context(), "no-such-tenant", SearchParams{})
	assert.ErrorContains(t, err, `"no-such-tenant" not found upstream`)

	_, err = a.Detail(t.Context(), "no-such-tenant", "1")
	assert.ErrorContains(t, err, `"no-such-tenant" not found upstream`)
}
