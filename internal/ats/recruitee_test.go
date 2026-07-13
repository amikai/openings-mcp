package ats

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/recruitee"
)

const remoteLocation = "remote"

func testRecruiteeAdapter(t *testing.T) (*RecruiteeAdapter, string) {
	t.Helper()
	srv := recruitee.NewMockServer()
	t.Cleanup(srv.Close)
	a := NewRecruiteeAdapter(&http.Client{Timeout: 5 * time.Second})
	a.baseURL = func(string) string { return srv.URL }
	return a, srv.URL
}

func TestRecruiteeRoster(t *testing.T) {
	a, _ := testRecruiteeAdapter(t)
	assert.Len(t, a.Roster(), len(recruitee.Companies))
}

func TestRecruiteeParseCareersURL(t *testing.T) {
	a, _ := testRecruiteeAdapter(t)

	tests := []struct {
		name     string
		rawURL   string
		wantSlug string
		wantOK   bool
	}{
		{name: "curated", rawURL: "https://bunq.recruitee.com/o/role-1", wantSlug: "bunq", wantOK: true},
		{
			name:     "unlisted",
			rawURL:   "https://" + recruitee.MockNonRosterSlug + ".recruitee.com/o/job",
			wantSlug: recruitee.MockNonRosterSlug,
			wantOK:   true,
		},
		{name: "reserved host", rawURL: "https://www.recruitee.com", wantOK: false},
		{name: "api host", rawURL: "https://api.recruitee.com", wantOK: false},
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

func TestRecruiteeSearchAll(t *testing.T) {
	a, _ := testRecruiteeAdapter(t)
	res, err := a.Search(t.Context(), recruitee.MockSlug, SearchParams{})
	require.NoError(t, err)
	// bunq mock has 58 offers (let's check total returned in Search page 1 which is limited to pageSize=20)
	assert.Greater(t, res.TotalCount, 0)
	assert.Len(t, res.Jobs, min(pageSize, res.TotalCount))
	for _, j := range res.Jobs {
		assert.NotEmpty(t, j.JobID)
		assert.NotEmpty(t, j.Title)
		assert.NotEmpty(t, j.Location)
		assert.NotEmpty(t, j.PostedAt)
		assert.NotEmpty(t, j.URL)
	}
}

func TestRecruiteeSearchQueryLocationAndFilters(t *testing.T) {
	a, _ := testRecruiteeAdapter(t)

	queried, err := a.Search(t.Context(), recruitee.MockSlug, SearchParams{Query: "Fraud Ops Expert"})
	require.NoError(t, err)
	assert.NotEmpty(t, queried.Jobs)
	assert.Equal(t, "Fraud Ops Expert", queried.Jobs[0].Title)

	located, err := a.Search(t.Context(), recruitee.MockSlug, SearchParams{Location: "Amsterdam"})
	require.NoError(t, err)
	for _, j := range located.Jobs {
		assert.Contains(t, j.Location, "Amsterdam")
	}

	filtered, err := a.Search(t.Context(), recruitee.MockSlug, SearchParams{
		Filters: FilterSet{"city": {"Sofia"}},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, filtered.Jobs)
}

func TestRecruiteeSearchSecondaryLocationFilters(t *testing.T) {
	a, _ := testRecruiteeAdapter(t)

	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "secondary city", key: "city", value: "Amsterdam"},
		{name: "secondary country", key: "country", value: "Netherlands"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertSearchContainsJobID(
				t,
				a,
				SearchParams{Filters: FilterSet{tt.key: {tt.value}}},
				"2675345",
			)
		})
	}
}

func TestRecruiteeUnknownLocationIsNotRemote(t *testing.T) {
	srv := recruitee.NewNullMockServer()
	t.Cleanup(srv.Close)
	a := NewRecruiteeAdapter(&http.Client{Timeout: 5 * time.Second})
	a.baseURL = func(string) string { return srv.URL }

	remote, err := a.Search(t.Context(), "minimal", SearchParams{Location: remoteLocation})
	require.NoError(t, err)
	assert.Zero(t, remote.TotalCount)
	assert.Empty(t, remote.Jobs)

	all, err := a.Search(t.Context(), "minimal", SearchParams{})
	require.NoError(t, err)
	require.Len(t, all.Jobs, 1)
	assert.Empty(t, all.Jobs[0].Location)
}

func TestRecruiteeDepartmentMapsToOrganizationUnit(t *testing.T) {
	a, _ := testRecruiteeAdapter(t)
	jobs, err := a.dump(t.Context(), recruitee.MockSlug)
	require.NoError(t, err)

	for _, job := range jobs {
		if job.summary.JobID == "2675345" {
			assert.Equal(t, "Support & Operations", job.orgUnit)
			return
		}
	}
	t.Fatal("fixture job 2675345 not found")
}

func TestRecruiteeSearchRejectsUnknownFilter(t *testing.T) {
	a, _ := testRecruiteeAdapter(t)
	_, err := a.Search(t.Context(), recruitee.MockSlug, SearchParams{
		Filters: FilterSet{"unsupported-recruitee-filter": {"unused"}},
	})
	require.ErrorContains(t, err, "unknown filter key")
	assert.ErrorContains(t, err, "city, country, department, employmentType, experience")
}

func TestRecruiteeFilters(t *testing.T) {
	a, _ := testRecruiteeAdapter(t)
	filters, err := a.Filters(t.Context(), recruitee.MockSlug)
	require.NoError(t, err)
	assert.Contains(t, filters["city"], "Amsterdam")
	assert.Contains(t, filters["country"], "Netherlands")
	assert.Contains(t, filters["department"], "Support & Operations")
}

func TestRecruiteeDetail(t *testing.T) {
	a, _ := testRecruiteeAdapter(t)
	detail, err := a.Detail(
		t.Context(),
		recruitee.MockSlug,
		"2675345",
	)
	require.NoError(t, err)
	assert.Equal(t, "Fraud Ops Expert", detail.Title)
	assert.Equal(t, "bunq", detail.Company)
	assert.Equal(t, "Sofia, Bulgaria; Amsterdam, Netherlands; İstanbul, Türkiye", detail.Location)
	assert.Equal(t, "2026-07-13", detail.PostedAt)
	assert.NotEmpty(t, detail.Description)
	assert.NotContains(t, detail.Description, "<p>")
}

func TestRecruiteeDetailNotFound(t *testing.T) {
	a, _ := testRecruiteeAdapter(t)
	_, err := a.Detail(t.Context(), recruitee.MockSlug, "no-such-id")
	assert.ErrorContains(t, err, "pass a job_id exactly as returned")
}

func TestRecruiteeUnknownHostUpstream(t *testing.T) {
	a, baseURL := testRecruiteeAdapter(t)
	a.baseURL = func(slug string) string {
		if slug == "missing" {
			return baseURL + "/missing"
		}
		return baseURL
	}
	_, err := a.Search(t.Context(), "missing", SearchParams{})
	assert.ErrorContains(t, err, "not found upstream")
}

func TestRecruiteeSearchIsDeterministic(t *testing.T) {
	a, _ := testRecruiteeAdapter(t)
	first, err := a.Search(t.Context(), recruitee.MockSlug, SearchParams{})
	require.NoError(t, err)
	second, err := a.Search(t.Context(), recruitee.MockSlug, SearchParams{})
	require.NoError(t, err)
	require.Len(t, second.Jobs, len(first.Jobs))
	for i := range first.Jobs {
		assert.Equal(t, first.Jobs[i].JobID, second.Jobs[i].JobID)
	}
	assert.True(t, strings.HasPrefix(first.Jobs[0].PostedAt, "20"))
}

func assertSearchContainsJobID(
	t *testing.T,
	a *RecruiteeAdapter,
	params SearchParams,
	jobID string,
) {
	t.Helper()
	for page := 1; ; page++ {
		params.Page = page
		result, err := a.Search(t.Context(), recruitee.MockSlug, params)
		require.NoError(t, err)
		for _, job := range result.Jobs {
			if job.JobID == jobID {
				return
			}
		}
		if page >= result.TotalPages {
			assert.Failf(t, "job not found", "job %s missing from filtered results", jobID)
			return
		}
	}
}
