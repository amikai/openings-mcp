package apple

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testCountryCode = "TWN"

func TestSearchJobs(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	client, err := NewJobsClient(srv.URL, srv.Client())
	require.NoError(t, err)

	response, err := client.SearchJobs(t.Context(), SearchRequest{
		Keyword:     "software engineer",
		CountryCode: "twn",
	})
	require.NoError(t, err)
	require.Len(t, response.Res.SearchResults, 11)

	first := response.Res.SearchResults[0]
	assert.Equal(t, 11, response.Res.TotalRecords)
	assert.Equal(t, MockJobID, first.PositionId)
	assert.Equal(t, "SoC Packaging Engineer", first.PostingTitle)
	assert.Equal(t, "Hardware", first.Team.TeamName)
	require.Len(t, first.Locations, 1)
	assert.Equal(t, "Taipei", first.Locations[0].Name)
}

func TestSearchJobsPageAndSort(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	client, err := NewJobsClient(srv.URL, srv.Client())
	require.NoError(t, err)

	response, err := client.SearchJobs(t.Context(), SearchRequest{
		Keyword:     "camera",
		CountryCode: "USA",
		Sort:        SortNewest,
		Page:        2,
	})
	require.NoError(t, err)
	assert.Equal(t, 250, response.Res.TotalRecords)
	require.Len(t, response.Res.SearchResults, 20)
	assert.Equal(t, "200669881", response.Res.SearchResults[0].PositionId)
}

func TestSearchJobsValidation(t *testing.T) {
	client, err := NewJobsClient("https://jobs.apple.com", http.DefaultClient)
	require.NoError(t, err)

	tests := []struct {
		name    string
		want    string
		request SearchRequest
	}{
		{name: "missing keyword", request: SearchRequest{CountryCode: testCountryCode}, want: "keyword is required"},
		{name: "short country", request: SearchRequest{Keyword: "go", CountryCode: "TW"}, want: "three ascii letters"},
		{name: "non-ascii country", request: SearchRequest{Keyword: "go", CountryCode: "台灣"}, want: "three ascii letters"},
		{name: "negative page", request: SearchRequest{Keyword: "go", CountryCode: testCountryCode, Page: -1}, want: "page must be >= 1"},
		{name: "invalid sort", request: SearchRequest{Keyword: "go", CountryCode: testCountryCode, Sort: Sort("oldest")}, want: "invalid sort"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := client.SearchJobs(t.Context(), test.request)
			assert.ErrorContains(t, err, test.want)
		})
	}
}

func TestJobDetail(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	client, err := NewJobsClient(srv.URL, srv.Client())
	require.NoError(t, err)

	response, err := client.JobDetail(t.Context(), MockJobID)
	require.NoError(t, err)
	assert.Equal(t, MockJobID, response.Res.PositionId)
	assert.Equal(t, "SoC Packaging Engineer", response.Res.PostingTitle)
	assert.Equal(t, "Taiwan", response.Res.Locations[0].CountryName)
	assert.NotEmpty(t, response.Res.Responsibilities.Or(""))
}

func TestJobDetailNotFound(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	client, err := NewJobsClient(srv.URL, srv.Client())
	require.NoError(t, err)

	_, err = client.JobDetail(t.Context(), MockNotFoundJobID)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrJobNotFound)
}

func TestJobDetailRejectsInvalidID(t *testing.T) {
	client, err := NewJobsClient("https://jobs.apple.com", http.DefaultClient)
	require.NoError(t, err)

	for _, jobID := range []string{"", "PIPE-200624996", "200624996-3950"} {
		_, err := client.JobDetail(t.Context(), jobID)
		require.Error(t, err)
		require.NotErrorIs(t, err, ErrJobNotFound)
		assert.ErrorContains(t, err, "only digits")
	}
}

func TestJobURL(t *testing.T) {
	assert.Equal(t,
		"https://jobs.apple.com/en-us/details/200624996/soc-packaging-engineer",
		JobURL("200624996", "soc-packaging-engineer"),
	)
}
