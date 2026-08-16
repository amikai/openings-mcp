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
	ctx := context.Background()

	res, err := a.Search(ctx, adp_myjobs.MockSlug, SearchParams{})
	require.NoError(t, err)
	require.Equal(t, 3, res.TotalCount)
	require.Len(t, res.Jobs, 3)

	res, err = a.Search(ctx, adp_myjobs.MockSlug, SearchParams{Query: "teacher"})
	require.NoError(t, err)
	require.Equal(t, 1, res.TotalCount)
	assert.Contains(t, strings.ToLower(res.Jobs[0].Title), "teacher")

	// A published value travels back as its slot code, applied upstream.
	res, err = a.Search(ctx, adp_myjobs.MockSlug, SearchParams{
		Filters: FilterSet{"state": {"Utah"}},
	})
	require.NoError(t, err)
	require.Equal(t, 1, res.TotalCount)
	assert.Contains(t, res.Jobs[0].Location, "Orem")

	// Case is canonicalized before sending; upstream would match nothing.
	res, err = a.Search(ctx, adp_myjobs.MockSlug, SearchParams{
		Filters: FilterSet{"state": {"utah"}},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, res.TotalCount)

	// Different keys AND.
	res, err = a.Search(ctx, adp_myjobs.MockSlug, SearchParams{
		Filters: FilterSet{"state": {"Utah"}, "position_type": {"Full-time"}},
	})
	require.NoError(t, err)
	assert.Equal(t, 0, res.TotalCount)

	d, err := a.Detail(ctx, adp_myjobs.MockSlug, "1002")
	require.NoError(t, err)
	assert.Equal(t, "1002", d.JobID)
	assert.NotEmpty(t, d.Description)
}

func TestADPMyJobsFiltersReportTenantDimensions(t *testing.T) {
	srv := adp_myjobs.NewMockServer()
	t.Cleanup(srv.Close)

	a := NewADPMyJobsAdapter(http.DefaultClient, nil)
	a.careerBase = srv.URL
	a.listingBase = srv.URL

	fs, err := a.Filters(context.Background(), adp_myjobs.MockSlug)
	require.NoError(t, err)
	assert.Equal(t, []string{"Pennsylvania", "Utah"}, fs["state"])
	assert.Equal(t, []string{"Full-time", "Part-time"}, fs["position_type"])
	// A board may configure two dimensions under one label; both stay reachable.
	assert.Equal(t, []string{"Nebraska"}, fs["state_2"])
}

func TestADPMyJobsSearchRejectsWhatItCannotApply(t *testing.T) {
	srv := adp_myjobs.NewMockServer()
	t.Cleanup(srv.Close)

	a := NewADPMyJobsAdapter(http.DefaultClient, nil)
	a.careerBase = srv.URL
	a.listingBase = srv.URL
	ctx := context.Background()

	// Free-text location has nothing to resolve against on a board that files
	// its jobs by store or by state code.
	_, err := a.Search(ctx, adp_myjobs.MockSlug, SearchParams{Location: "Orem"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "location is not supported")

	// An unknown key must never reach upstream: it would return the whole board.
	_, err = a.Search(ctx, adp_myjobs.MockSlug, SearchParams{
		Filters: FilterSet{"brand": {"anything"}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown filter key "brand"`)

	// An unknown value must never reach upstream either: it would return zero
	// jobs, which reads as "this company has none".
	_, err = a.Search(ctx, adp_myjobs.MockSlug, SearchParams{
		Filters: FilterSet{"state": {"Atlantis"}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Upstream cannot OR two values for one dimension.
	_, err = a.Search(ctx, adp_myjobs.MockSlug, SearchParams{
		Filters: FilterSet{"state": {"Utah", "Pennsylvania"}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "one value per search")
}

func TestADPFilterKeyNormalizesLabels(t *testing.T) {
	assert.Equal(t, "area_of_interest", adpFilterKey("Area of Interest", "FIELD3"))
	assert.Equal(t, "full_time_part_time", adpFilterKey("Full-Time/Part-Time", "FIELD3"))
	assert.Equal(t, "salaried_hourly", adpFilterKey("Salaried / Hourly", "FIELD2"))
	// A label that normalizes to nothing falls back to the slot code.
	assert.Equal(t, "field4", adpFilterKey("  ", "FIELD4"))
}

func TestADPMyJobsHostPatternRegistered(t *testing.T) {
	assert.Contains(t, _careersHostPatternsByAdapter, "adp_myjobs")
}
