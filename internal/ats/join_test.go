package ats

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/join"
)

func testJoinAdapter(t *testing.T) *JoinAdapter {
	t.Helper()
	srv := join.NewMockServer()
	t.Cleanup(srv.Close)
	return NewJoinAdapter(srv.URL, &http.Client{Timeout: 5 * time.Second})
}

func TestJoinRoster(t *testing.T) {
	a := testJoinAdapter(t)
	assert.Len(t, a.Roster(), len(join.Companies))
	for _, c := range a.Roster() {
		assert.NotEmptyf(t, c.Slug, "roster entry with empty field: %+v", c)
		assert.NotEmptyf(t, c.Name, "roster entry with empty field: %+v", c)
	}
}

func TestJoinSearchAll(t *testing.T) {
	a := testJoinAdapter(t)
	res, err := a.Search(t.Context(), join.MockJobSlug, SearchParams{})
	require.NoError(t, err)
	require.Equal(t, 3, res.TotalCount)
	for _, j := range res.Jobs {
		assert.NotEmptyf(t, j.JobID, "summary with empty field: %+v", j)
		assert.NotEmptyf(t, j.Title, "summary with empty field: %+v", j)
		assert.NotEmptyf(t, j.URL, "summary with empty field: %+v", j)
	}
}

func TestJoinSearchQueryMatchesTitle(t *testing.T) {
	a := testJoinAdapter(t)
	res, err := a.Search(t.Context(), join.MockJobSlug, SearchParams{Query: "backend"})
	require.NoError(t, err)
	require.Equal(t, 1, res.TotalCount)
	assert.Equal(t, "Senior Software Engineer (Backend/LLM Infrastructure)", res.Jobs[0].Title)
}

func TestJoinSearchQueryMatchesCategory(t *testing.T) {
	a := testJoinAdapter(t)
	// "Software Development" is job 1's category and not in any title.
	res, err := a.Search(t.Context(), join.MockJobSlug, SearchParams{Query: "software development"})
	require.NoError(t, err)
	require.Equal(t, 1, res.TotalCount)
}

func TestJoinSearchLocation(t *testing.T) {
	a := testJoinAdapter(t)
	res, err := a.Search(t.Context(), join.MockJobSlug, SearchParams{Location: "berlin"})
	require.NoError(t, err)
	assert.Equal(t, 3, res.TotalCount)
}

func TestJoinSearchFilterCategory(t *testing.T) {
	a := testJoinAdapter(t)
	res, err := a.Search(t.Context(), join.MockJobSlug, SearchParams{
		Filters: map[string][]string{"category": {"Other"}},
	})
	require.NoError(t, err)
	assert.Equal(t, 2, res.TotalCount)
}

func TestJoinFilters(t *testing.T) {
	a := testJoinAdapter(t)
	fs, err := a.Filters(t.Context(), join.MockJobSlug)
	require.NoError(t, err)
	assert.Contains(t, fs["category"], "Software Development")
	assert.Contains(t, fs["category"], "Other")
	assert.Contains(t, fs["employment_type"], "Employee")
	assert.Contains(t, fs["employment_type"], "Internship")
}

func TestJoinSearchEmptyCompany(t *testing.T) {
	a := testJoinAdapter(t)
	res, err := a.Search(t.Context(), "hey-contact-heroes", SearchParams{})
	require.NoError(t, err)
	assert.Equal(t, 0, res.TotalCount)
}

func TestJoinSearchUnknownCompany(t *testing.T) {
	a := testJoinAdapter(t)
	_, err := a.Search(t.Context(), "not-in-roster", SearchParams{})
	assert.Error(t, err)
}

func TestJoinDetail(t *testing.T) {
	a := testJoinAdapter(t)
	d, err := a.Detail(t.Context(), join.MockJobSlug, join.MockJobIdParam)
	require.NoError(t, err)
	assert.Equal(t, "Senior Software Engineer (Backend/LLM Infrastructure)", d.Title)
	assert.Equal(t, "Routine Labs", d.Company)
	assert.Contains(t, d.Description, "Routine Labs")
	assert.NotEmpty(t, d.URL)
	assert.Equal(t, join.MockJobIdParam, d.JobID)
}

// A REMOTE/ANYWHERE job's city/country still carry the employer's base
// location, but Location must say something other than the bare city — a
// bare "Berlin" would misrepresent an anywhere-remote role as on-site
// there. See joinLocation and API.md's remoteType note.
func TestJoinDetailRemoteAnywhere(t *testing.T) {
	a := testJoinAdapter(t)
	d, err := a.Detail(t.Context(), join.MockRemoteJobSlug, join.MockRemoteJobIdParam)
	require.NoError(t, err)
	assert.Contains(t, d.Location, "Remote")
	assert.NotEqual(t, "Berlin", d.Location)
}

func TestJoinDetailNotFound(t *testing.T) {
	a := testJoinAdapter(t)
	_, err := a.Detail(t.Context(), join.MockJobSlug, "00000000-nonexistent-job")
	assert.Error(t, err)
}

func TestJoinDetailUnknownCompany(t *testing.T) {
	a := testJoinAdapter(t)
	_, err := a.Detail(t.Context(), "not-in-roster", join.MockJobIdParam)
	assert.Error(t, err)
}

func TestJoinParseCareersURL(t *testing.T) {
	a := testJoinAdapter(t)
	u, err := url.Parse("https://join.com/companies/" + join.MockJobSlug)
	require.NoError(t, err)
	slug, ok := a.ParseCareersURL(u)
	assert.True(t, ok)
	assert.Equal(t, join.MockJobSlug, slug)
}

func TestJoinParseCareersURLNonRoster(t *testing.T) {
	a := testJoinAdapter(t)
	u, err := url.Parse("https://join.com/companies/not-in-roster")
	require.NoError(t, err)
	_, ok := a.ParseCareersURL(u)
	assert.False(t, ok, "an arbitrary slug can't be resolved to a companyId without a network call")
}

func TestJoinParseCareersURLUnrelatedHost(t *testing.T) {
	a := testJoinAdapter(t)
	u, err := url.Parse("https://example.com/companies/" + join.MockJobSlug)
	require.NoError(t, err)
	_, ok := a.ParseCareersURL(u)
	assert.False(t, ok)
}

func TestJoinLocation(t *testing.T) {
	tests := []struct {
		name                             string
		city, country, workplace, remote string
		want                             string
	}{
		{"onsite", "Berlin", "Germany", "ONSITE", "", "Berlin, Germany"},
		{"hybrid", "Berlin", "Germany", "HYBRID", "", "Berlin, Germany"},
		{"onsite no country", "Berlin", "", "ONSITE", "", "Berlin"},
		{"onsite no city", "", "Germany", "ONSITE", "", "Germany"},
		{"remote anywhere", "Berlin", "Germany", "REMOTE", "ANYWHERE", "Remote (Anywhere) · Berlin, Germany"},
		{"remote anywhere no base", "", "", "REMOTE", "ANYWHERE", "Remote (Anywhere)"},
		{"remote country", "Hamburg", "Germany", "REMOTE", "COUNTRY", "Remote (Germany)"},
		{"remote country no country", "Hamburg", "", "REMOTE", "COUNTRY", "Remote"},
		{"remote unspecified scope", "Berlin", "Germany", "REMOTE", "", "Remote · Berlin, Germany"},
		{"remote unspecified scope no base", "", "", "REMOTE", "", "Remote"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, joinLocation(tt.city, tt.country, tt.workplace, tt.remote))
		})
	}
}
