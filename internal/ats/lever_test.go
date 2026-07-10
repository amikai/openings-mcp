package ats

import (
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/lever"
)

func testLeverAdapter(t *testing.T) *LeverAdapter {
	t.Helper()
	srv := lever.NewMockServer()
	t.Cleanup(srv.Close)
	a, err := NewLeverAdapter(srv.URL, &http.Client{Timeout: 5 * time.Second})
	require.NoError(t, err)
	return a
}

func TestLeverRoster(t *testing.T) {
	a := testLeverAdapter(t)
	roster := a.Roster()
	require.Len(t, roster, len(lever.Companies))
	for _, c := range roster {
		assert.NotEmptyf(t, c.Slug, "roster entry with empty field: %+v", c)
		assert.NotEmptyf(t, c.Name, "roster entry with empty field: %+v", c)
	}
}

func TestLeverSearchAll(t *testing.T) {
	a := testLeverAdapter(t)
	res, err := a.Search(t.Context(), "leverdemo", SearchParams{})
	require.NoError(t, err)
	require.Equal(t, 3, res.TotalCount)
	require.Len(t, res.Jobs, 3)
	for _, j := range res.Jobs {
		assert.NotEmptyf(t, j.JobID, "summary with empty field: %+v", j)
		assert.NotEmptyf(t, j.Title, "summary with empty field: %+v", j)
		assert.NotEmptyf(t, j.URL, "summary with empty field: %+v", j)
		assert.NotEmptyf(t, j.PostedAt, "summary with empty field: %+v", j)
	}
}

func TestLeverSearchQuery(t *testing.T) {
	a := testLeverAdapter(t)
	res, err := a.Search(t.Context(), "leverdemo", SearchParams{Query: "AbelsonTaylor"})
	require.NoError(t, err)
	require.NotEmpty(t, res.Jobs)
	assert.Equal(t, "AbelsonTaylor Writer", res.Jobs[0].Title, "want AbelsonTaylor Writer first")
}

func TestLeverSearchFilters(t *testing.T) {
	a := testLeverAdapter(t)
	res, err := a.Search(t.Context(), "leverdemo", SearchParams{
		Filters: map[string][]string{"team": {"Professional Services"}},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, res.Jobs, "want at least one Professional Services job")
}

func TestLeverFilters(t *testing.T) {
	a := testLeverAdapter(t)
	fs, err := a.Filters(t.Context(), "leverdemo")
	require.NoError(t, err)
	assert.True(t, slices.Contains(fs["team"], "Professional Services"), `fs["team"] = %v, want it to contain "Professional Services"`, fs["team"])
}

func TestLeverDetail(t *testing.T) {
	a := testLeverAdapter(t)
	d, err := a.Detail(t.Context(), "leverdemo", "33538a2f-d27d-4a96-8f05-fa4b0e4d940e")
	require.NoError(t, err)
	assert.Equal(t, "AbelsonTaylor Writer", d.Title)
	assert.Contains(t, d.Description, "Welcome to the Demo", "Description should contain the fixture opening")
	assert.NotContains(t, d.Description, "<", "Description should be plain text")
}

func TestLeverDetailNotFound(t *testing.T) {
	a := testLeverAdapter(t)
	_, err := a.Detail(t.Context(), "leverdemo", lever.MockNotFoundPostingID)
	assert.Error(t, err, "want error for unknown posting id")
}

func TestLeverDetailCompanyFallsBackToSlug(t *testing.T) {
	a := testLeverAdapter(t)
	res, err := a.Search(t.Context(), "somestartup", SearchParams{})
	require.NoError(t, err)
	require.NotEmpty(t, res.Jobs)
	d, err := a.Detail(t.Context(), "somestartup", res.Jobs[0].JobID)
	require.NoError(t, err)
	assert.Equal(t, "somestartup", d.Company, "non-roster slug should be used as company name")
}
