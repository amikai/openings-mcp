package ats

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/adp_myjobs"
)

func TestADPMyJobsRosterBuildsRegistry(t *testing.T) {
	a := NewADPMyJobsAdapter(http.DefaultClient, nil)
	require.Equal(t, "adp_myjobs", a.Name())
	_, err := NewRegistry(a)
	require.NoError(t, err)
	require.NotEmpty(t, a.Roster())
}

func TestADPMyJobsParseCareersURL(t *testing.T) {
	a := NewADPMyJobsAdapter(http.DefaultClient, nil)
	tests := []struct {
		raw  string
		slug string
		ok   bool
	}{
		{"https://myjobs.adp.com/guitarcenterexternal/cx", "guitarcenterexternal", true},
		{"https://myjobs.adp.com/GuitarCenterExternal/cx/job/123", "guitarcenterexternal", true},
		{"https://workforcenow.adp.com/mascsr/default/mdf/recruitment/recruitment.html?cid=abc", "", false},
		{"https://jobs.adp.com/en/jobs/", "", false},
		{"https://example.com/guitarcenterexternal", "", false},
	}
	for _, tc := range tests {
		u, err := url.Parse(tc.raw)
		require.NoError(t, err)
		slug, ok := a.ParseCareersURL(u)
		assert.Equal(t, tc.ok, ok, tc.raw)
		assert.Equal(t, tc.slug, slug, tc.raw)
	}
}

func TestADPMyJobsSearchUsesServerSearch(t *testing.T) {
	srv := adp_myjobs.NewMockServer()
	t.Cleanup(srv.Close)

	a := NewADPMyJobsAdapter(http.DefaultClient, nil)
	a.careerBase = srv.URL
	a.listingBase = srv.URL

	// Empty query: unfiltered page from mock.
	res, err := a.Search(context.Background(), adp_myjobs.MockSlug, SearchParams{})
	require.NoError(t, err)
	require.Equal(t, 2, res.TotalCount)
	require.Len(t, res.Jobs, 2)

	// Keyword path uses $search; mock returns the filtered fixture.
	res, err = a.Search(context.Background(), adp_myjobs.MockSlug, SearchParams{Query: "teacher"})
	require.NoError(t, err)
	require.Equal(t, 1, res.TotalCount)
	require.Len(t, res.Jobs, 1)
	assert.Contains(t, strings.ToLower(res.Jobs[0].Title), "teacher")

	// Location resolves to requisitionLocations/itemID and can be sent with
	// the keyword in the same upstream request.
	res, err = a.Search(context.Background(), adp_myjobs.MockSlug, SearchParams{
		Query:    "teacher",
		Location: "Orem",
	})
	require.NoError(t, err)
	require.Equal(t, 1, res.TotalCount)
	assert.Contains(t, res.Jobs[0].Location, "Orem")

	filters, err := a.Filters(context.Background(), adp_myjobs.MockSlug)
	require.NoError(t, err)
	assert.Contains(t, filters["location"], "GC - Orem-2")

	d, err := a.Detail(context.Background(), adp_myjobs.MockSlug, res.Jobs[0].JobID)
	require.NoError(t, err)
	assert.Equal(t, res.Jobs[0].JobID, d.JobID)
	assert.NotEmpty(t, d.Description)
}

func TestADPMyJobsHostPatternRegistered(t *testing.T) {
	assert.Contains(t, careersHostPatternsByAdapter, "adp_myjobs")
}
