package oracle

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	searchFields = "TotalJobsCount,Limit,Offset;" +
		"requisitionList:Id,Title,PostedDate,PrimaryLocation,WorkplaceType,WorkplaceTypeCode;" +
		"requisitionList.secondaryLocations:Name;" +
		"titlesFacet:Id,Name,TotalCount;locationsFacet:Id,Name,TotalCount;" +
		"categoriesFacet:Id,Name,TotalCount;postingDatesFacet:Id,Name,TotalCount;" +
		"workLocationsFacet:Id,Name,TotalCount;organizationsFacet:Id,Name,TotalCount;" +
		"workplaceTypesFacet:Id,Name,TotalCount"
	detailFields = "Id,Title,ExternalPostedStartDate,PrimaryLocation,WorkplaceType," +
		"ExternalDescriptionStr,CorporateDescriptionStr,ExternalResponsibilitiesStr," +
		"ExternalQualificationsStr;secondaryLocations:Name"
)

func TestSearchJobs(t *testing.T) {
	client := newTestClient(t)

	got, err := client.SearchJobs(t.Context(), searchParams(
		"findReqs;siteNumber=CX_1,facetsList=NONE,limit=3,offset=0",
	))
	require.NoError(t, err)
	require.Len(t, got.Items, 1)

	page := got.Items[0]
	assert.Equal(t, 1330, page.TotalJobsCount)
	assert.Equal(t, 3, page.Limit)
	require.Len(t, page.RequisitionList, 3)

	first := page.RequisitionList[0]
	assert.Equal(t, "361564", first.ID.Or(""))
	assert.Equal(t, "Cardiologist - Echo / Imaging", first.Title.Or(""))
	assert.Equal(t, "Phoenix, AZ, United States", first.PrimaryLocation.Or(""))
	assert.Equal(t, time.Date(2026, time.July, 14, 0, 0, 0, 0, time.UTC), first.PostedDate.Or(time.Time{}))
	assert.True(t, first.WorkplaceTypeCode.IsNull())
}

func TestSearchJobsFiltered(t *testing.T) {
	client := newTestClient(t)

	got, err := client.SearchJobs(t.Context(), searchParams(
		`findReqs;siteNumber=CX_1,facetsList=NONE,limit=3,offset=0,keyword="analyst"`,
	))
	require.NoError(t, err)
	require.Len(t, got.Items, 1)

	page := got.Items[0]
	assert.Equal(t, 33, page.TotalJobsCount)
	require.Len(t, page.RequisitionList, 3)
	assert.Contains(t, page.RequisitionList[0].Title.Or(""), "Analyst")
}

func TestSearchJobsFacets(t *testing.T) {
	client := newTestClient(t)

	got, err := client.SearchJobs(t.Context(), searchParams(
		"findReqs;siteNumber=CX_1,facetsList=TITLES;LOCATIONS;CATEGORIES;WORKPLACE_TYPES;POSTING_DATES;WORK_LOCATIONS;ORGANIZATIONS,limit=1,offset=0",
	))
	require.NoError(t, err)
	require.Len(t, got.Items, 1)

	page := got.Items[0]
	require.Len(t, page.TitlesFacet, 10)
	require.Len(t, page.LocationsFacet, 10)
	require.Len(t, page.CategoriesFacet, 10)
	require.Len(t, page.WorkplaceTypesFacet, 1)
	require.Len(t, page.PostingDatesFacet, 3)
	require.Len(t, page.WorkLocationsFacet, 10)
	require.Len(t, page.OrganizationsFacet, 8)

	assert.Equal(t, "NURSING", page.TitlesFacet[0].ID.Or(""))
	assert.EqualValues(t, 300000006426003, page.LocationsFacet[2].ID.Or(0))
	assert.Equal(t, "Rochester, MN, United States", page.LocationsFacet[2].Name.Or(""))
	assert.Equal(t, "ORA_ON_SITE", page.WorkplaceTypesFacet[0].ID.Or(""))
	assert.EqualValues(t, 7, page.PostingDatesFacet[0].ID.Or(0))
	assert.EqualValues(t, 300000034547579, page.WorkLocationsFacet[0].ID.Or(0))
	assert.EqualValues(t, 1, page.OrganizationsFacet[0].ID.Or(0))
}

func TestGetJobDetail(t *testing.T) {
	client := newTestClient(t)

	got, err := client.GetJobDetail(t.Context(), detailParams(
		`ById;Id="386920",siteNumber=CX_1`,
	))
	require.NoError(t, err)
	require.Len(t, got.Items, 1)

	job := got.Items[0]
	assert.Equal(t, "386920", job.ID.Or(""))
	assert.Equal(t, "Senior Analyst - ATS", job.Title.Or(""))
	assert.Equal(t, "Phoenix, AZ, United States", job.PrimaryLocation.Or(""))
	assert.Contains(t, job.ExternalDescriptionStr.Or(""), "Access Technologies and Systems")
	assert.Contains(t, job.ExternalQualificationsStr.Or(""), "Cadence/Epic certification")
	assert.Equal(
		t,
		"2026-07-14T14:00:09Z",
		job.ExternalPostedStartDate.Or(time.Time{}).Format(time.RFC3339),
	)
}

func TestGetJobDetailNotFound(t *testing.T) {
	client := newTestClient(t)

	got, err := client.GetJobDetail(t.Context(), detailParams(
		`ById;Id="999999999999",siteNumber=CX_1`,
	))
	require.NoError(t, err)
	assert.Empty(t, got.Items)
}

func newTestClient(t *testing.T) *Client {
	t.Helper()
	server := NewMockServer()
	t.Cleanup(server.Close)

	client, err := NewClient(server.URL, WithClient(server.Client()))
	require.NoError(t, err)
	return client
}

func searchParams(finder string) SearchJobsParams {
	return SearchJobsParams{
		OnlyData:       SearchJobsOnlyDataTrue,
		Fields:         NewOptString(searchFields),
		Finder:         finder,
		AcceptLanguage: NewOptString("en"),
		OraIrcLanguage: NewOptString("en"),
	}
}

func detailParams(finder string) GetJobDetailParams {
	return GetJobDetailParams{
		OnlyData:       GetJobDetailOnlyDataTrue,
		Fields:         NewOptString(detailFields),
		Finder:         finder,
		AcceptLanguage: NewOptString("en"),
		OraIrcLanguage: NewOptString("en"),
	}
}
