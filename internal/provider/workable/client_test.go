package workable

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchJobs(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.SearchJobs(t.Context(), &SearchRequest{}, SearchJobsParams{Account: "blueground"})
	require.NoError(t, err)

	page, ok := res.(*SearchResponse)
	require.True(t, ok, "want *SearchResponse, got %T", res)

	assert.Equal(t, 29, page.Total)
	require.Len(t, page.Results, 10)
	assert.Equal(t, NewOptString(MockPage2Token), page.NextPage)

	first := page.Results[0]
	assert.Equal(t, 5954363, first.ID)
	assert.Equal(t, "264C395E51", first.Shortcode)
	assert.Equal(t, "Senior Performance Marketing Strategist", first.Title)
	assert.Equal(t, NewOptBool(false), first.Remote)
	assert.Equal(t, NewOptLocation(Location{
		Country:     NewOptNilString("Greece"),
		CountryCode: NewOptNilString("GR"),
		City:        NewOptNilString("Athens"),
		Region:      NewOptNilString("Attica"),
		Display:     NewOptNilString("Athens, Greece"),
	}), first.Location)
	require.Len(t, first.Locations, 1)
	assert.Equal(t, NewOptBool(false), first.Locations[0].Hidden)
	assert.Equal(t, NewOptString("published"), first.State)
	// code is present-but-null when the account sets no requisition code.
	assert.True(t, first.Code.Set)
	assert.True(t, first.Code.Null)
	assert.Equal(t, NewOptString("2026-07-14T00:00:00.000Z"), first.Published)
	assert.Equal(t, NewOptString("full"), first.Type)
	assert.Equal(t, []string{"Shared Services"}, first.Department)
	assert.Equal(t, NewOptJobSummaryWorkplace(JobSummaryWorkplaceOnSite), first.Workplace)
}

// TestSearchJobsPage2 proves the cursor round-trip: sending page 1's nextPage
// back as the token body field yields the next 10 jobs, not page 1 again.
func TestSearchJobsPage2(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.SearchJobs(t.Context(), &SearchRequest{
		Token: NewOptString(MockPage2Token),
	}, SearchJobsParams{Account: "blueground"})
	require.NoError(t, err)

	page, ok := res.(*SearchResponse)
	require.True(t, ok, "want *SearchResponse, got %T", res)

	assert.Equal(t, 29, page.Total)
	require.Len(t, page.Results, 10)
	assert.Equal(t, "9D3D73F77D", page.Results[0].Shortcode)
	assert.Equal(t, "Operations Partner (Flexible Contract - Emeryville)", page.Results[0].Title)
	assert.True(t, page.NextPage.Set, "a page carrying more results must return a cursor")
	assert.NotEqual(t, NewOptString(MockPage2Token), page.NextPage)
}

// TestSearchJobsFiltered proves query is modeled as a real server-side filter
// rather than an ignored field: the fixture's total narrows from 29 to 9, and
// the whole result fits one page, so nextPage disappears.
func TestSearchJobsFiltered(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.SearchJobs(t.Context(), &SearchRequest{
		Query: NewOptString("engineer"),
	}, SearchJobsParams{Account: "blueground"})
	require.NoError(t, err)

	page, ok := res.(*SearchResponse)
	require.True(t, ok, "want *SearchResponse, got %T", res)

	assert.Equal(t, 9, page.Total)
	require.Len(t, page.Results, 9)
	assert.False(t, page.NextPage.Set, "the last page must omit nextPage")
}

// TestSearchJobsUnknownCompany guards the 404 quirk: an unknown account is a
// text/plain 404, unlike SmartRecruiters' 200-with-empty-page.
func TestSearchJobsUnknownCompany(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.SearchJobs(t.Context(), &SearchRequest{}, SearchJobsParams{Account: MockUnknownCompany})
	require.NoError(t, err)

	_, ok := res.(*NotFound)
	require.True(t, ok, "want *NotFound, got %T", res)
}

func TestListJobFilters(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.ListJobFilters(t.Context(), ListJobFiltersParams{Account: "blueground"})
	require.NoError(t, err)

	facets, ok := res.(*FiltersResponse)
	require.True(t, ok, "want *FiltersResponse, got %T", res)

	require.Len(t, facets.Locations, 16)
	assert.Equal(t, FacetLocation{
		Country:     NewOptNilString("United States"),
		CountryCode: NewOptNilString("US"),
		Display:     NewOptNilString("United States"),
	}, facets.Locations[0])

	require.Len(t, facets.Departments, 2)
	first := facets.Departments[0]
	assert.Equal(t, 435335, first.ID)
	assert.Equal(t, "City Core", first.Name)
	// The filter id set covers the department and its descendants; sending it,
	// not the bare id, is what matches every job under the department.
	assert.Equal(t, []int{435335, 435336}, first.Filter)
	assert.True(t, first.ParentID.Set)
	assert.True(t, first.ParentID.Null)

	assert.Equal(t, []string{"full", "contract", "temporary"}, facets.Worktypes)
	assert.Equal(t, []bool{false, true}, facets.Remotes)
	assert.Equal(t, []string{"on_site", "hybrid", "remote"}, facets.Workplaces)
}

func TestGetJob(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetJob(t.Context(), GetJobParams{Account: "blueground", Shortcode: "B02DA69C8F"})
	require.NoError(t, err)

	job, ok := res.(*JobDetail)
	require.True(t, ok, "want *JobDetail, got %T", res)

	assert.Equal(t, "B02DA69C8F", job.Shortcode)
	assert.Equal(t, "Senior Software Engineer, iOS", job.Title)
	assert.Equal(t, NewOptBool(true), job.Remote)
	assert.Equal(t, NewOptJobDetailWorkplace(JobDetailWorkplaceRemote), job.Workplace)
	assert.Contains(t, job.Description.Value, "Redefining how people live")
	// The three body fields are always present but may be empty: this posting
	// keeps its whole body in description.
	assert.Equal(t, NewOptString(""), job.Requirements)
	assert.Equal(t, NewOptString(""), job.Benefits)
}

func TestGetJobNotFound(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetJob(t.Context(), GetJobParams{Account: "blueground", Shortcode: "0000000000"})
	require.NoError(t, err)

	_, ok := res.(*NotFound)
	require.True(t, ok, "want *NotFound, got %T", res)
}
