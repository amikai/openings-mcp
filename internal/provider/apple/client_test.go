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

func TestSearchJobsFiltered(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	client, err := NewJobsClient(srv.URL, srv.Client())
	require.NoError(t, err)

	response, err := client.SearchJobs(t.Context(), SearchRequest{
		Keyword:     "engineer",
		CountryCode: "USA",
		Sort:        SortNewest,
		Page:        2,
		Keywords:    []string{"camera"},
		Teams:       []TeamFilter{{TeamCode: "hrdwr", SubTeamCode: "cam"}},
		Products:    []string{"iphn"},
		Languages:   []string{"en_US"},
	})
	require.NoError(t, err)
	assert.Equal(t, 63, response.Res.TotalRecords)
	require.Len(t, response.Res.SearchResults, 20)
	assert.Equal(t, "200666897", response.Res.SearchResults[0].PositionId)
}

func TestSearchAPIRequestFilters(t *testing.T) {
	request, err := searchAPIRequest(SearchRequest{
		Keyword:     "go",
		CountryCode: testCountryCode,
		HomeOffice:  true,
	})
	require.NoError(t, err)
	homeOffice, ok := request.Filters.HomeOffice.Get()
	require.True(t, ok)
	assert.True(t, homeOffice)

	request, err = searchAPIRequest(SearchRequest{Keyword: "go", CountryCode: testCountryCode})
	require.NoError(t, err)
	_, ok = request.Filters.HomeOffice.Get()
	assert.False(t, ok, "homeOffice must be omitted unless requested, matching the site")
	assert.Empty(t, request.Filters.Keywords)
	assert.Empty(t, request.Filters.Teams)
	assert.Empty(t, request.Filters.Products)
	assert.Empty(t, request.Filters.Languages)
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
		{name: "blank keyword filter", request: SearchRequest{Keyword: "go", CountryCode: testCountryCode, Keywords: []string{" "}}, want: "keyword filters must not be blank"},
		{name: "blank team code", request: SearchRequest{Keyword: "go", CountryCode: testCountryCode, Teams: []TeamFilter{{SubTeamCode: "AF"}}}, want: "team code must not be blank"},
		{name: "invalid sub-team code", request: SearchRequest{Keyword: "go", CountryCode: testCountryCode, Teams: []TeamFilter{{TeamCode: "SFTWR", SubTeamCode: "A-F"}}}, want: "sub-team code must contain only ascii letters and digits"},
		{name: "invalid product code", request: SearchRequest{Keyword: "go", CountryCode: testCountryCode, Products: []string{"iPhone 17"}}, want: "product code must contain only ascii letters and digits"},
		{name: "invalid language code", request: SearchRequest{Keyword: "go", CountryCode: testCountryCode, Languages: []string{"zh-TW"}}, want: "language code must contain only ascii letters and underscores"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := client.SearchJobs(t.Context(), test.request)
			assert.ErrorContains(t, err, test.want)
		})
	}
}

func TestListTeams(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	client, err := NewJobsClient(srv.URL, srv.Client())
	require.NoError(t, err)

	response, err := client.ListTeams(t.Context())
	require.NoError(t, err)
	require.Len(t, response.Res, 11)

	first := response.Res[0]
	assert.Equal(t, "teamsAndSubTeams-MLAI", first.ID)
	assert.Equal(t, "Machine Learning and AI", first.Type)
	require.NotEmpty(t, first.Teams)
	assert.Equal(t, "MLI", first.Teams[0].Code)
	assert.Equal(t, "MLAI", first.Teams[0].TeamCode)
	assert.Equal(t, "Machine Learning and AI: Machine Learning Infrastructure", first.Teams[0].DisplayName)
}

func TestParseTeamFilter(t *testing.T) {
	team, err := ParseTeamFilter("HRDWR/CAM")
	require.NoError(t, err)
	assert.Equal(t, TeamFilter{TeamCode: "HRDWR", SubTeamCode: "CAM"}, team)

	_, err = ParseTeamFilter("HRDWR")
	assert.ErrorContains(t, err, "TEAM/SUBTEAM")
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
