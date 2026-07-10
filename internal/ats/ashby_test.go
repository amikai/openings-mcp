package ats

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/ashby"
)

func testAshbyAdapter(t *testing.T) *AshbyAdapter {
	t.Helper()
	srv := ashby.NewMockServer()
	t.Cleanup(srv.Close)
	a, err := NewAshbyAdapter(srv.URL, &http.Client{Timeout: 5 * time.Second})
	require.NoError(t, err)
	return a
}

func TestAshbyRoster(t *testing.T) {
	a := testAshbyAdapter(t)
	assert.Len(t, a.Roster(), len(ashby.Companies))
}

func TestAshbySearchAll(t *testing.T) {
	a := testAshbyAdapter(t)
	res, err := a.Search(t.Context(), ashby.MockBoardName, SearchParams{})
	require.NoError(t, err)
	require.Equal(t, 5, res.TotalCount, "all fixture jobs are listed")
	for _, j := range res.Jobs {
		assert.NotEmptyf(t, j.JobID, "summary with empty field: %+v", j)
		assert.NotEmptyf(t, j.Title, "summary with empty field: %+v", j)
		assert.NotEmptyf(t, j.URL, "summary with empty field: %+v", j)
	}
}

func TestAshbySearchQueryAndFilters(t *testing.T) {
	a := testAshbyAdapter(t)
	res, err := a.Search(t.Context(), ashby.MockBoardName, SearchParams{Query: "agent platform"})
	require.NoError(t, err)
	require.NotEmpty(t, res.Jobs)
	assert.Equal(t, "Software Engineer (Agent Platform)", res.Jobs[0].Title)

	filtered, err := a.Search(t.Context(), ashby.MockBoardName, SearchParams{
		Filters: map[string][]string{"employmentType": {"FullTime"}},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, filtered.Jobs, "want at least one FullTime job")
}

func TestAshbyFilters(t *testing.T) {
	a := testAshbyAdapter(t)
	fs, err := a.Filters(t.Context(), ashby.MockBoardName)
	require.NoError(t, err)
	for _, key := range []string{"department", "employmentType", "workplaceType"} {
		assert.NotEmptyf(t, fs[key], "FilterSet missing %q: %v", key, fs)
	}
}

func TestAshbyDetailRefetchesBoard(t *testing.T) {
	a := testAshbyAdapter(t)
	d, err := a.Detail(t.Context(), ashby.MockBoardName, "7724fbe3-6a27-4418-9705-2dcc40751a16")
	require.NoError(t, err)
	assert.Equal(t, "Software Engineer (Agent Platform)", d.Title)
	assert.NotEmpty(t, d.Description, "Description should be non-empty plain text")
}

func TestAshbyDetailNotFound(t *testing.T) {
	a := testAshbyAdapter(t)
	_, err := a.Detail(t.Context(), ashby.MockBoardName, "no-such-id")
	assert.Error(t, err, "want error for unknown job id")
}

func TestAshbyUnknownBoardUpstream(t *testing.T) {
	a := testAshbyAdapter(t)
	_, err := a.Search(t.Context(), "not-in-mock", SearchParams{})
	assert.Error(t, err, "want error when upstream returns 404")
}

func TestAshbySearchIsDeterministic(t *testing.T) {
	a := testAshbyAdapter(t)
	r1, err := a.Search(t.Context(), ashby.MockBoardName, SearchParams{})
	require.NoError(t, err)
	r2, err := a.Search(t.Context(), ashby.MockBoardName, SearchParams{})
	require.NoError(t, err)
	for i := range r1.Jobs {
		assert.Equal(t, r1.Jobs[i].JobID, r2.Jobs[i].JobID, "search order is not deterministic")
	}
	assert.Truef(t, strings.HasPrefix(r1.Jobs[0].PostedAt, "20"), "PostedAt should be an ISO date, got %q", r1.Jobs[0].PostedAt)
}

func TestAshbyDetailCompanyFallsBackToSlug(t *testing.T) {
	a := testAshbyAdapter(t)
	d, err := a.Detail(t.Context(), ashby.MockNonRosterBoard, "7724fbe3-6a27-4418-9705-2dcc40751a16")
	require.NoError(t, err)
	assert.Equal(t, ashby.MockNonRosterBoard, d.Company, "non-roster slug should be used as company name")
}
