package ats

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/rippling"
)

// mockRipplingBoard is the board slug the provider mock server serves a
// dump for: 33 captured entries collapsing to 12 jobs (see the provider's
// testdata/jobs_rsp.json).
const mockRipplingBoard = "pythian"

func testRipplingAdapter(t *testing.T) *RipplingAdapter {
	t.Helper()
	srv := rippling.NewMockServer()
	t.Cleanup(srv.Close)
	a, err := NewRipplingAdapter(srv.URL, &http.Client{Timeout: 5 * time.Second})
	require.NoError(t, err)
	return a
}

func TestRipplingRoster(t *testing.T) {
	a := testRipplingAdapter(t)
	assert.Len(t, a.Roster(), len(rippling.Companies))
	for _, c := range a.Roster() {
		assert.NotEmptyf(t, c.Slug, "roster entry with empty field: %+v", c)
		assert.NotEmptyf(t, c.Name, "roster entry with empty field: %+v", c)
	}
}

func TestRipplingParseCareersURL(t *testing.T) {
	a := testRipplingAdapter(t)
	for input, want := range map[string]string{
		"https://ats.rippling.com/pythian/jobs":              "pythian",
		"https://ats.rippling.com/boom-supersonic/jobs/144f": "boom-supersonic",
		"https://ATS.rippling.com/Pythian":                   "pythian",
	} {
		u, err := url.Parse(input)
		require.NoError(t, err)
		slug, ok := a.ParseCareersURL(u)
		assert.Truef(t, ok, "want %q recognized", input)
		assert.Equal(t, want, slug)
	}

	u, err := url.Parse("https://jobs.example.com/pythian")
	require.NoError(t, err)
	_, ok := a.ParseCareersURL(u)
	assert.False(t, ok, "non-Rippling host must not match")
}

// TestRipplingSearchAll guards the dedup quirk: the fixture's 33
// per-(job, location) entries must collapse to 12 jobs with their
// locations merged.
func TestRipplingSearchAll(t *testing.T) {
	a := testRipplingAdapter(t)
	res, err := a.Search(t.Context(), mockRipplingBoard, SearchParams{})
	require.NoError(t, err)
	require.Equal(t, 12, res.TotalCount)
	byID := make(map[string]JobSummary)
	for _, j := range res.Jobs {
		assert.NotEmptyf(t, j.JobID, "summary with empty field: %+v", j)
		assert.NotEmptyf(t, j.Title, "summary with empty field: %+v", j)
		assert.NotEmptyf(t, j.URL, "summary with empty field: %+v", j)
		_, dup := byID[j.JobID]
		assert.Falsef(t, dup, "job %s appears twice in one page", j.JobID)
		byID[j.JobID] = j
	}
	multi, ok := byID["144f31c4-38a4-4666-97b4-2c88a3f123da"]
	require.True(t, ok)
	assert.Equal(t, "Poland; Spain; Romania; India; United Kingdom", multi.Location,
		"a multi-location job must merge every duplicate entry's location")

	// 12 jobs < pageSize, so page 2 is empty but the envelope stays sane.
	page2, err := a.Search(t.Context(), mockRipplingBoard, SearchParams{Page: 2})
	require.NoError(t, err)
	assert.Empty(t, page2.Jobs)
	assert.Equal(t, 2, page2.Page)
	assert.Equal(t, 1, page2.TotalPages)
}

func TestRipplingSearchQueryMatchesTitle(t *testing.T) {
	a := testRipplingAdapter(t)
	res, err := a.Search(t.Context(), mockRipplingBoard, SearchParams{Query: "engineer"})
	require.NoError(t, err)
	assert.Equal(t, 5, res.TotalCount)
	for _, j := range res.Jobs {
		assert.Contains(t, strings.ToLower(j.Title), "engineer")
	}
}

func TestRipplingSearchQueryMatchesOrgUnit(t *testing.T) {
	a := testRipplingAdapter(t)
	// "sales" appears in two jobs' titles and in the Sales department;
	// both title jobs sit in that department, so the org-unit tier is the
	// only extra reach — but a query for the department name alone must
	// still match its jobs.
	res, err := a.Search(t.Context(), mockRipplingBoard, SearchParams{Query: "managed services"})
	require.NoError(t, err)
	assert.Equal(t, 10, res.TotalCount, "query hitting only the department name should match")
}

func TestRipplingSearchLocation(t *testing.T) {
	a := testRipplingAdapter(t)
	remote, err := a.Search(t.Context(), mockRipplingBoard, SearchParams{Location: "remote"})
	require.NoError(t, err)
	assert.Equal(t, 2, remote.TotalCount, `Location "remote" should fall back to location-text match`)

	canada, err := a.Search(t.Context(), mockRipplingBoard, SearchParams{Location: "canada"})
	require.NoError(t, err)
	assert.Equal(t, 2, canada.TotalCount, `Location "canada" should fuzzy-match merged locations`)
}

func TestRipplingSearchFilterDepartment(t *testing.T) {
	a := testRipplingAdapter(t)
	res, err := a.Search(t.Context(), mockRipplingBoard, SearchParams{
		Filters: map[string][]string{"department": {"Sales"}},
	})
	require.NoError(t, err)
	assert.Equal(t, 2, res.TotalCount)
}

func TestRipplingFilters(t *testing.T) {
	a := testRipplingAdapter(t)
	fs, err := a.Filters(t.Context(), mockRipplingBoard)
	require.NoError(t, err)
	assert.Equal(t, []string{"Managed Services", "Sales"}, fs["department"])
}

func TestRipplingDetail(t *testing.T) {
	a := testRipplingAdapter(t)
	d, err := a.Detail(t.Context(), mockRipplingBoard, "144f31c4-38a4-4666-97b4-2c88a3f123da")
	require.NoError(t, err)
	assert.Equal(t, "DevOps Engineer", d.Title)
	assert.Equal(t, "Pythian", d.Company)
	assert.Equal(t, "India; United Kingdom; Poland; Spain; Romania", d.Location)
	assert.Equal(t, "2026-06-25", d.PostedAt)
	assert.Contains(t, d.Description, "Pythian", "Description should carry the company blurb")
	assert.Contains(t, d.Description, "DevOps", "Description should carry the role text")
	assert.NotContains(t, d.Description, "<p", "Description should be plain text")
	assert.NotEmpty(t, d.URL)
	assert.Equal(t, "144f31c4-38a4-4666-97b4-2c88a3f123da", d.JobID)
}

func TestRipplingDetailNotFound(t *testing.T) {
	a := testRipplingAdapter(t)
	_, err := a.Detail(t.Context(), mockRipplingBoard, "1b2c3d4e-5f60-4789-8abc-def012345678")
	assert.Error(t, err, "want error for unknown job id")
}

func TestRipplingUnknownBoardUpstream(t *testing.T) {
	a := testRipplingAdapter(t)
	_, err := a.Search(t.Context(), "this-board-does-not-exist-xyz", SearchParams{})
	assert.Error(t, err, "want error when upstream returns 404")
}

// The detail response carries its own companyName, so even a non-roster
// board (reached via careers URL) reports the upstream's display name
// rather than the slug.
func TestRipplingDetailNonRosterCompanyFromUpstream(t *testing.T) {
	a := testRipplingAdapter(t)
	d, err := a.Detail(t.Context(), rippling.MockNonRosterBoard, "144f31c4-38a4-4666-97b4-2c88a3f123da")
	require.NoError(t, err)
	assert.Equal(t, "Pythian", d.Company)
}
