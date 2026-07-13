package ats

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/teamtailor"
)

func testTeamtailorAdapter(t *testing.T) (*TeamtailorAdapter, string) {
	t.Helper()
	srv := teamtailor.NewMockServer()
	t.Cleanup(srv.Close)
	a := NewTeamtailorAdapter(&http.Client{Timeout: 5 * time.Second})
	a.baseURL = func(string) string { return srv.URL }
	return a, srv.URL
}

func TestTeamtailorRoster(t *testing.T) {
	a, _ := testTeamtailorAdapter(t)
	assert.Len(t, a.Roster(), len(teamtailor.Companies))
}

func TestTeamtailorParseCareersURL(t *testing.T) {
	a, _ := testTeamtailorAdapter(t)
	teamtailor.CompaniesByHost["careers.example.com"] = teamtailor.Company{
		Name: "Custom",
		Host: "careers.example.com",
	}
	t.Cleanup(func() { delete(teamtailor.CompaniesByHost, "careers.example.com") })

	tests := []struct {
		name     string
		rawURL   string
		wantSlug string
		wantOK   bool
	}{
		{name: "curated", rawURL: "https://career.teamtailor.com/jobs/1-role", wantSlug: "career.teamtailor.com", wantOK: true},
		{name: "unlisted eu", rawURL: "https://acme.teamtailor.com/jobs", wantSlug: "acme.teamtailor.com", wantOK: true},
		{name: "unlisted na", rawURL: "https://acme.na.teamtailor.com/jobs", wantSlug: "acme.na.teamtailor.com", wantOK: true},
		{name: "unlisted au", rawURL: "https://acme.au.teamtailor.com/jobs", wantSlug: "acme.au.teamtailor.com", wantOK: true},
		{name: "curated custom domain", rawURL: "https://careers.example.com/jobs", wantSlug: "careers.example.com", wantOK: true},
		{name: "reserved product host", rawURL: "https://www.teamtailor.com/en", wantOK: false},
		{name: "reserved regional api", rawURL: "https://api.na.teamtailor.com/v1/jobs", wantOK: false},
		{name: "unrelated", rawURL: "https://careers.example.org/jobs", wantOK: false},
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

func TestTeamtailorSearchAll(t *testing.T) {
	a, _ := testTeamtailorAdapter(t)
	res, err := a.Search(t.Context(), teamtailor.MockHost, SearchParams{})
	require.NoError(t, err)
	require.Equal(t, 3, res.TotalCount)
	for _, j := range res.Jobs {
		assert.NotEmpty(t, j.JobID)
		assert.NotEmpty(t, j.Title)
		assert.NotEmpty(t, j.Location)
		assert.NotEmpty(t, j.PostedAt)
		assert.NotEmpty(t, j.URL)
	}
}

func TestTeamtailorSearchQueryLocationAndFilters(t *testing.T) {
	a, _ := testTeamtailorAdapter(t)

	queried, err := a.Search(t.Context(), teamtailor.MockHost, SearchParams{Query: "sap fico"})
	require.NoError(t, err)
	require.Len(t, queried.Jobs, 1)
	assert.Equal(t, "Expert SAP FICO & Reporting Finance", queried.Jobs[0].Title)

	located, err := a.Search(t.Context(), teamtailor.MockHost, SearchParams{Location: "Wielsbeke"})
	require.NoError(t, err)
	require.Len(t, located.Jobs, 1)
	assert.Equal(t, "Lab Manager", located.Jobs[0].Title)

	filtered, err := a.Search(t.Context(), teamtailor.MockHost, SearchParams{
		Filters: FilterSet{"city": {"Engis"}, "country": {"BE"}},
	})
	require.NoError(t, err)
	assert.Equal(t, 2, filtered.TotalCount)
}

func TestTeamtailorSearchRejectsUnknownFilter(t *testing.T) {
	a, _ := testTeamtailorAdapter(t)
	_, err := a.Search(t.Context(), teamtailor.MockHost, SearchParams{
		Filters: FilterSet{"unsupported-teamtailor-filter": {"unused"}},
	})
	require.ErrorContains(t, err, "unknown filter key")
	assert.ErrorContains(t, err, "city, country, region")
}

func TestTeamtailorFilters(t *testing.T) {
	a, _ := testTeamtailorAdapter(t)
	filters, err := a.Filters(t.Context(), teamtailor.MockHost)
	require.NoError(t, err)
	assert.Equal(t, []string{"Engis", "Wielsbeke"}, filters["city"])
	assert.Equal(t, []string{"BE"}, filters["country"])
	assert.Equal(t, []string{"WE&I"}, filters["region"])
}

func TestTeamtailorDetail(t *testing.T) {
	a, _ := testTeamtailorAdapter(t)
	detail, err := a.Detail(
		t.Context(),
		teamtailor.MockHost,
		"a97df59d-7a99-4387-8956-8e032e8bf793",
	)
	require.NoError(t, err)
	assert.Equal(t, "Electromécanicien de maintenance (H/F/X)", detail.Title)
	assert.Equal(t, "Knauf Belgium", detail.Company)
	assert.Equal(t, "Engis", detail.Location)
	assert.Equal(t, "2026-06-11", detail.PostedAt)
	assert.NotEmpty(t, detail.Description)
	assert.NotContains(t, detail.Description, "<p>")
}

func TestTeamtailorDetailNotFound(t *testing.T) {
	a, _ := testTeamtailorAdapter(t)
	_, err := a.Detail(t.Context(), teamtailor.MockHost, "no-such-id")
	assert.ErrorContains(t, err, "pass a job_id exactly as returned")
}

func TestTeamtailorUnknownHostUpstream(t *testing.T) {
	a, baseURL := testTeamtailorAdapter(t)
	a.baseURL = func(host string) string {
		if host == "missing.teamtailor.com" {
			return baseURL + "/missing"
		}
		return baseURL
	}
	_, err := a.Search(t.Context(), "missing.teamtailor.com", SearchParams{})
	assert.ErrorContains(t, err, "not found upstream")
}

func TestTeamtailorRejectsInvalidHostSlug(t *testing.T) {
	a, _ := testTeamtailorAdapter(t)
	_, err := a.Search(t.Context(), "metadata.google.internal", SearchParams{})
	assert.ErrorContains(t, err, "unknown career-site host")
}

func TestTeamtailorSearchIsDeterministic(t *testing.T) {
	a, _ := testTeamtailorAdapter(t)
	first, err := a.Search(t.Context(), teamtailor.MockHost, SearchParams{})
	require.NoError(t, err)
	second, err := a.Search(t.Context(), teamtailor.MockHost, SearchParams{})
	require.NoError(t, err)
	require.Len(t, second.Jobs, len(first.Jobs))
	for i := range first.Jobs {
		assert.Equal(t, first.Jobs[i].JobID, second.Jobs[i].JobID)
	}
	assert.True(t, strings.HasPrefix(first.Jobs[0].PostedAt, "20"))
}
