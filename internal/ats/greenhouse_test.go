package ats

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/greenhouse"
)

// mockGreenhouseBoard is the board token the provider mock server serves a
// content=true dump for (5 hand-crafted jobs; see the provider's
// testdata/jobs_content_rsp.json).
const mockGreenhouseBoard = "safariai"

func testGreenhouseAdapter(t *testing.T) *GreenhouseAdapter {
	t.Helper()
	srv := greenhouse.NewMockServer()
	t.Cleanup(srv.Close)
	a, err := NewGreenhouseAdapter(srv.URL, &http.Client{Timeout: 5 * time.Second})
	require.NoError(t, err)
	return a
}

func TestGreenhouseRoster(t *testing.T) {
	a := testGreenhouseAdapter(t)
	assert.Len(t, a.Roster(), len(greenhouse.Companies))
	for _, c := range a.Roster() {
		assert.NotEmptyf(t, c.Slug, "roster entry with empty field: %+v", c)
		assert.NotEmptyf(t, c.Name, "roster entry with empty field: %+v", c)
	}
}

func TestGreenhouseSearchAll(t *testing.T) {
	a := testGreenhouseAdapter(t)
	res, err := a.Search(t.Context(), mockGreenhouseBoard, SearchParams{})
	require.NoError(t, err)
	require.Equal(t, 5, res.TotalCount)
	assert.Equal(t, "6100001004", res.Jobs[0].JobID, "newest job should sort first")
	for _, j := range res.Jobs {
		assert.NotEmptyf(t, j.JobID, "summary with empty field: %+v", j)
		assert.NotEmptyf(t, j.Title, "summary with empty field: %+v", j)
		assert.NotEmptyf(t, j.URL, "summary with empty field: %+v", j)
		assert.Truef(t, strings.HasPrefix(j.PostedAt, "20"), "PostedAt should be an ISO date, got %q", j.PostedAt)
	}
	// 5 jobs < PageSize, so page 2 is empty but the envelope stays sane.
	page2, err := a.Search(t.Context(), mockGreenhouseBoard, SearchParams{Page: 2})
	require.NoError(t, err)
	assert.Empty(t, page2.Jobs)
	assert.Equal(t, 2, page2.Page)
	assert.Equal(t, 1, page2.TotalPages)
}

func TestGreenhouseSearchQueryRanksTitleFirst(t *testing.T) {
	a := testGreenhouseAdapter(t)
	res, err := a.Search(t.Context(), mockGreenhouseBoard, SearchParams{Query: "agent platform"})
	require.NoError(t, err)
	require.Equal(t, 2, res.TotalCount, "want title hit + JD-body hit")
	require.Len(t, res.Jobs, 2)
	assert.Equal(t, "6100001002", res.Jobs[0].JobID, "want title hit before JD-body hit")
	assert.Equal(t, "6100001004", res.Jobs[1].JobID, "want title hit before JD-body hit")
}

func TestGreenhouseSearchQueryMatchesJDBody(t *testing.T) {
	a := testGreenhouseAdapter(t)
	res, err := a.Search(t.Context(), mockGreenhouseBoard, SearchParams{Query: "kubernetes"})
	require.NoError(t, err)
	require.Equal(t, 1, res.TotalCount, "query hitting only entity-encoded JD content should match")
	assert.Equal(t, "Senior Backend Engineer", res.Jobs[0].Title)
}

func TestGreenhouseSearchQueryMatchesOrgUnit(t *testing.T) {
	a := testGreenhouseAdapter(t)
	// "people" appears only in the Technical Recruiter job's department.
	res, err := a.Search(t.Context(), mockGreenhouseBoard, SearchParams{Query: "people"})
	require.NoError(t, err)
	require.Equal(t, 1, res.TotalCount, "query hitting only the department name should match")
	assert.Equal(t, "Technical Recruiter", res.Jobs[0].Title)
}

func TestGreenhouseSearchLocation(t *testing.T) {
	a := testGreenhouseAdapter(t)
	remote, err := a.Search(t.Context(), mockGreenhouseBoard, SearchParams{Location: "remote"})
	require.NoError(t, err)
	require.Equal(t, 1, remote.TotalCount, `Location "remote" should fall back to location-text match`)
	assert.Equal(t, "Product Designer", remote.Jobs[0].Title)

	london, err := a.Search(t.Context(), mockGreenhouseBoard, SearchParams{Location: "london"})
	require.NoError(t, err)
	require.Equal(t, 1, london.TotalCount, `Location "london" should fuzzy-match`)
	assert.Equal(t, "Data Scientist", london.Jobs[0].Title)
}

func TestGreenhouseSearchFilterDepartment(t *testing.T) {
	a := testGreenhouseAdapter(t)
	res, err := a.Search(t.Context(), mockGreenhouseBoard, SearchParams{
		Filters: map[string][]string{"department": {"Engineering"}},
	})
	require.NoError(t, err)
	assert.Equal(t, 2, res.TotalCount)
}

func TestGreenhouseFilters(t *testing.T) {
	a := testGreenhouseAdapter(t)
	fs, err := a.Filters(t.Context(), mockGreenhouseBoard)
	require.NoError(t, err)
	for _, key := range []string{"department", "office"} {
		assert.NotEmptyf(t, fs[key], "FilterSet missing %q: %v", key, fs)
	}
	wantDepts := []string{"Data", "Design", "Engineering", "People"}
	assert.Equal(t, wantDepts, fs["department"])
}

func TestGreenhouseDetail(t *testing.T) {
	a := testGreenhouseAdapter(t)
	d, err := a.Detail(t.Context(), "anthropic", "4461450008")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(d.Title, "Account Executive"), "Title = %q", d.Title)
	assert.Equal(t, "Anthropic", d.Company, "Company should be the display name from the roster")
	assert.Contains(t, d.Description, "About Anthropic", "Description should carry decoded JD text")
	assert.NotContains(t, d.Description, "&lt;", "Description should be plain text")
	assert.NotContains(t, d.Description, "<div", "Description should be plain text")
	assert.NotEmpty(t, d.URL)
	assert.Equal(t, "4461450008", d.JobID)
}

func TestGreenhouseDetailBadID(t *testing.T) {
	a := testGreenhouseAdapter(t)
	_, err := a.Detail(t.Context(), "anthropic", "not-a-number")
	assert.Error(t, err, "want teaching error for non-numeric job id")
}

func TestGreenhouseDetailNotFound(t *testing.T) {
	a := testGreenhouseAdapter(t)
	_, err := a.Detail(t.Context(), "anthropic", "999999999999")
	assert.Error(t, err, "want error for unknown job id")
}

func TestGreenhouseUnknownBoardUpstream(t *testing.T) {
	a := testGreenhouseAdapter(t)
	_, err := a.Search(t.Context(), "doesnotexist", SearchParams{})
	assert.Error(t, err, "want error when upstream returns 404")
}

func TestGreenhouseDetailCompanyFallsBackToSlug(t *testing.T) {
	a := testGreenhouseAdapter(t)
	d, err := a.Detail(t.Context(), greenhouse.MockNonRosterBoard, "4461450008")
	require.NoError(t, err)
	assert.Equal(t, greenhouse.MockNonRosterBoard, d.Company, "non-roster slug should be used as company name")
}
