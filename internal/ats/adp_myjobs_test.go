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
	require.Equal(t, 3, res.TotalCount)
	require.Len(t, res.Jobs, 3)

	// Keyword path uses $search; mock returns the filtered fixture.
	res, err = a.Search(context.Background(), adp_myjobs.MockSlug, SearchParams{Query: "teacher"})
	require.NoError(t, err)
	require.Equal(t, 1, res.TotalCount)
	require.Len(t, res.Jobs, 1)
	assert.Contains(t, strings.ToLower(res.Jobs[0].Title), "teacher")

	// Location resolves to the matching location's coordinates and travels as a
	// geographic box, alongside the keyword in the same upstream request.
	res, err = a.Search(context.Background(), adp_myjobs.MockSlug, SearchParams{
		Query:    "teacher",
		Location: "Orem",
	})
	require.NoError(t, err)
	require.Equal(t, 1, res.TotalCount)
	assert.Contains(t, res.Jobs[0].Location, "Orem")

	// The box really constrains: a keyword that only matches elsewhere on the
	// board comes back empty rather than unfiltered.
	res, err = a.Search(context.Background(), adp_myjobs.MockSlug, SearchParams{
		Query:    "engineer",
		Location: "Orem",
	})
	require.NoError(t, err)
	assert.Equal(t, 0, res.TotalCount)
	assert.Empty(t, res.Jobs)

	// Location alone excludes the rest of the board.
	res, err = a.Search(context.Background(), adp_myjobs.MockSlug, SearchParams{Location: "Langhorne"})
	require.NoError(t, err)
	require.Equal(t, 1, res.TotalCount)
	assert.Contains(t, res.Jobs[0].Location, "Langhorne")

	// Every listed value is one Search can honor, so the ungeocoded location is
	// absent even though the board publishes it.
	filters, err := a.Filters(context.Background(), adp_myjobs.MockSlug)
	require.NoError(t, err)
	assert.Contains(t, filters["location"], "GC - Orem-2")
	assert.Contains(t, filters["location"], "GC - Langhorne-1")
	assert.NotContains(t, filters["location"], "GC - Ungeocoded-3")

	res, err = a.Search(context.Background(), adp_myjobs.MockSlug, SearchParams{Query: "teacher"})
	require.NoError(t, err)
	d, err := a.Detail(context.Background(), adp_myjobs.MockSlug, res.Jobs[0].JobID)
	require.NoError(t, err)
	assert.Equal(t, res.Jobs[0].JobID, d.JobID)
	assert.NotEmpty(t, d.Description)
}

func TestADPMyJobsSearchLocationErrors(t *testing.T) {
	srv := adp_myjobs.NewMockServer()
	t.Cleanup(srv.Close)

	a := NewADPMyJobsAdapter(http.DefaultClient, nil)
	a.careerBase = srv.URL
	a.listingBase = srv.URL

	// Unknown label: point the caller at the filter list rather than silently
	// returning the whole board.
	_, err := a.Search(context.Background(), adp_myjobs.MockSlug, SearchParams{Location: "Atlantis"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no location matching")

	// A known label the board never geocoded cannot be expressed as a box.
	_, err = a.Search(context.Background(), adp_myjobs.MockSlug, SearchParams{Location: "Ungeocoded"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "without coordinates")

	// Upstream takes one box per request, so two locations is an error, not a
	// silently widened box.
	_, err = a.Search(context.Background(), adp_myjobs.MockSlug, SearchParams{
		Filters: FilterSet{"location": {"Orem", "Langhorne"}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "one location per search")
}

func TestADPMyJobsResolveGeoBoxEnclosesEveryMatch(t *testing.T) {
	options := []adpLocationOption{
		{id: "a", label: "West", aliases: []string{"West Store, CA"}, points: []adpGeoPoint{{lat: 34, lon: -118}}},
		{id: "b", label: "East", aliases: []string{"East Store, CA"}, points: []adpGeoPoint{{lat: 36, lon: -116}}},
		{id: "c", label: "Other", aliases: []string{"Other Store, TX"}, points: []adpGeoPoint{{lat: 30, lon: -97}}},
	}

	// "Store, CA" matches two locations; the box must cover both and exclude TX.
	box, err := resolveADPGeoBox(options, "Store, CA")
	require.NoError(t, err)
	assert.InDelta(t, -118-adpGeoPad, box.West, 1e-9)
	assert.InDelta(t, -116+adpGeoPad, box.East, 1e-9)
	assert.InDelta(t, 34-adpGeoPad, box.South, 1e-9)
	assert.InDelta(t, 36+adpGeoPad, box.North, 1e-9)

	// A single match still yields a non-degenerate box; upstream 500s on one.
	box, err = resolveADPGeoBox(options, "Other")
	require.NoError(t, err)
	assert.Less(t, box.West, box.East)
	assert.Less(t, box.South, box.North)
}

func TestADPMyJobsLocationLabelsCarryCodeAndLongName(t *testing.T) {
	labels := adpLocationLabels(adp_myjobs.RequisitionLocation{
		NameCode: &adp_myjobs.NameCode{CodeValue: "12", LongName: "Store - Peterborough-12"},
		Address: &adp_myjobs.LocationAddress{
			CityName:                 "Peterborough",
			CountrySubdivisionLevel1: &adp_myjobs.CodeVal{CodeValue: "ON", LongName: "Ontario"},
			Country:                  &adp_myjobs.CodeVal{CodeValue: "CAN", LongName: "Canada"},
		},
	})

	// Both spellings, so "ON"/"CAN" and "Ontario"/"Canada" all resolve.
	assert.Contains(t, labels, "Peterborough, ON, CAN")
	assert.Contains(t, labels, "Peterborough, Ontario, Canada")

	option := adpLocationOption{id: "1", label: "Store - Peterborough-12", aliases: labels,
		points: []adpGeoPoint{{lat: 44.3, lon: -78.3}}}
	for _, value := range []string{"ON", "Ontario", "CAN", "Canada", "Peterborough"} {
		_, err := resolveADPGeoBox([]adpLocationOption{option}, value)
		assert.NoError(t, err, value)
	}
}

func TestADPMyJobsResolveGeoBoxPrefersWholeTokens(t *testing.T) {
	// A state code is a substring of unrelated town names, and on a nationwide
	// board those accidental hits would stretch the box across the country.
	options := []adpLocationOption{
		{id: "1", label: "OH - Columbus", aliases: []string{"OH - Columbus", "Columbus, OH, USA"},
			points: []adpGeoPoint{{lat: 39.9571056, lon: -83.1273705}}},
		{id: "2", label: "OH - Cleveland", aliases: []string{"OH - Cleveland", "Cleveland, OH, USA"},
			points: []adpGeoPoint{{lat: 41.4995, lon: -81.6959}}},
		{id: "3", label: "CO - Johnstown", aliases: []string{"CO - Johnstown", "Johnstown, CO, USA"},
			points: []adpGeoPoint{{lat: 40.33434, lon: -104.933721}}},
		{id: "4", label: "TN - Johnson City", aliases: []string{"TN - Johnson City", "Johnson City, TN, USA"},
			points: []adpGeoPoint{{lat: 36.320002, lon: -82.330882}}},
	}

	box, err := resolveADPGeoBox(options, "OH")
	require.NoError(t, err)
	assert.Greater(t, box.West, -84.0, "Johnstown, CO must not widen an Ohio box")
	assert.Greater(t, box.South, 39.0, "Johnson City, TN must not widen an Ohio box")

	// An exact label beats the same string appearing inside longer labels.
	box, err = resolveADPGeoBox(options, "OH - Columbus")
	require.NoError(t, err)
	assert.InDelta(t, -83.1273705, box.West, 1e-3)
	assert.InDelta(t, -83.1273705, box.East, 1e-3)

	// A partial word still resolves through the substring fallback.
	box, err = resolveADPGeoBox(options, "Johnst")
	require.NoError(t, err)
	assert.InDelta(t, -104.933721, box.West, 1e-3)
}

func TestADPMyJobsHostPatternRegistered(t *testing.T) {
	assert.Contains(t, careersHostPatternsByAdapter, "adp_myjobs")
}
