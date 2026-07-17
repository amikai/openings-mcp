package ats

import (
	"math"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/ultipro"
)

// mockUltiProSlug is a roster-shaped slug (lowercase company code) whose
// resolved Company happens to match the mock fixtures' company code and
// board id, so testUltiProAdapter's baseURL override always applies
// regardless of which company the slug names.
const mockUltiProSlug = "tec1006teser"

func testUltiProAdapter(t *testing.T) *UltiProAdapter {
	t.Helper()
	mock := ultipro.NewMockServer()
	t.Cleanup(mock.Close)
	a := NewUltiProAdapter(&http.Client{Timeout: 5 * time.Second})
	// Swap only the host; keep the real companyCode/boardId path so
	// requests still land on the mock's fixture routes (registered under
	// ultipro.MockCompanyCode/MockBoardID, the same values TechnoServe's
	// roster entry carries).
	a.baseURL = func(s ultipro.CareersSite) string {
		return mock.URL + "/" + s.CompanyCode + "/JobBoard/" + s.BoardID
	}
	return a
}

func TestUltiProRosterBuildsRegistry(t *testing.T) {
	_, err := NewRegistry(NewUltiProAdapter(http.DefaultClient))
	require.NoError(t, err)
}

func TestUltiProRosterReturnsCompanyNames(t *testing.T) {
	a := NewUltiProAdapter(http.DefaultClient)
	roster := a.Roster()
	require.NotEmpty(t, roster)
	found := false
	for _, c := range roster {
		if c.Slug == mockUltiProSlug {
			found = true
			assert.Equal(t, "TechnoServe", c.Name)
		}
	}
	assert.True(t, found, "expected %q in roster", mockUltiProSlug)
}

func TestUltiProParseCareersURL(t *testing.T) {
	a := NewUltiProAdapter(http.DefaultClient)
	cases := []struct {
		raw  string
		ok   bool
		slug string
	}{
		{"https://recruiting.ultipro.com/TEC1006TESER/JobBoard/18180d88-ced0-4361-bd09-d5eef66dab24/", true, mockUltiProSlug},
		{"https://recruiting2.ultipro.com/SAL1002/JobBoard/bcc2e2d1-d94c-2041-4126-28086417eb0a/", true, "sal1002"},
		{
			// Not on the roster: falls back to the canonical URL slug.
			"https://recruiting.ultipro.com/UNKNOWNCODE/JobBoard/00000000-0000-0000-0000-000000000000/",
			true,
			"https://recruiting.ultipro.com/UNKNOWNCODE/JobBoard/00000000-0000-0000-0000-000000000000/",
		},
		{"https://boards.greenhouse.io/x", false, ""},
	}
	for _, tc := range cases {
		u, err := url.Parse(tc.raw)
		require.NoError(t, err)
		slug, ok := a.ParseCareersURL(u)
		assert.Equal(t, tc.ok, ok, tc.raw)
		assert.Equal(t, tc.slug, slug, tc.raw)
	}
}

func TestUltiProSearch(t *testing.T) {
	a := testUltiProAdapter(t)
	res, err := a.Search(t.Context(), mockUltiProSlug, SearchParams{})
	require.NoError(t, err)
	assert.Equal(t, 90, res.TotalCount)
	assert.Len(t, res.Jobs, 20)
	first := res.Jobs[0]
	assert.NotEmpty(t, first.JobID)
	assert.NotEmpty(t, first.Title)
	assert.NotEmpty(t, first.Location)
	assert.Len(t, first.PostedAt, len("2006-01-02"))
}

func TestUltiProSearchPageOverflow(t *testing.T) {
	a := testUltiProAdapter(t)
	_, err := a.Search(t.Context(), mockUltiProSlug, SearchParams{Page: math.MaxInt})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too large")
}

func TestUltiProSearchFilterDepartment(t *testing.T) {
	a := testUltiProAdapter(t)
	res, err := a.Search(t.Context(), mockUltiProSlug, SearchParams{
		Filters: FilterSet{"department": {"Finance"}},
	})
	require.NoError(t, err)
	require.NotEmpty(t, res.Jobs)
}

func TestUltiProSearchFilterTeachingErrors(t *testing.T) {
	a := testUltiProAdapter(t)

	_, err := a.Search(t.Context(), mockUltiProSlug, SearchParams{
		Filters: FilterSet{"schedule": {"FullTime"}},
	})
	require.ErrorContains(t, err, "unknown filter key")

	_, err = a.Search(t.Context(), mockUltiProSlug, SearchParams{
		Filters: FilterSet{"department": {"Nonexistent Department"}},
	})
	require.ErrorContains(t, err, `filter value "Nonexistent Department" not found`)
	assert.Contains(t, err.Error(), "available:")

	_, err = a.Search(t.Context(), mockUltiProSlug, SearchParams{
		Filters: FilterSet{"location_type": {"nowhere"}},
	})
	require.ErrorContains(t, err, `filter value "nowhere" not found`)
	assert.Contains(t, err.Error(), "Hybrid, Onsite, Remote")
}

func TestUltiProFilters(t *testing.T) {
	a := testUltiProAdapter(t)
	fs, err := a.Filters(t.Context(), mockUltiProSlug)
	require.NoError(t, err)
	assert.Equal(t, []string{"Hybrid", "Onsite", "Remote"}, fs["location_type"])
	assert.Contains(t, fs["department"], "Finance")
}

func TestUltiProDetail(t *testing.T) {
	a := testUltiProAdapter(t)
	d, err := a.Detail(t.Context(), mockUltiProSlug, ultipro.MockOpportunityID)
	require.NoError(t, err)
	assert.Equal(t, ultipro.MockOpportunityID, d.JobID)
	assert.Equal(t, "Conseiller Senior en Partenariat-BeniBiz", d.Title)
	assert.Equal(t, "TechnoServe", d.Company)
	assert.NotEmpty(t, d.Location)
	assert.NotEmpty(t, d.Description)
	assert.Contains(t, d.URL, "opportunityId="+ultipro.MockOpportunityID)
}

func TestUltiProDetailNotFound(t *testing.T) {
	a := testUltiProAdapter(t)
	_, err := a.Detail(t.Context(), mockUltiProSlug, ultipro.MockNotFoundOpportunityID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
