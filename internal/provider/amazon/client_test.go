package amazon

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearch(t *testing.T) {
	server := NewMockServer()
	t.Cleanup(server.Close)
	client, err := NewClient(server.URL, WithClient(server.Client()))
	require.NoError(t, err)

	result, err := client.Search(t.Context(), SearchRequest{
		Query:  "software engineer",
		Sort:   SearchJobsSortRelevant,
		Limit:  2,
		Offset: 0,
	})
	require.NoError(t, err)
	require.Len(t, result.Jobs, 2)
	assert.Equal(t, 1986, result.Total)
	assert.Equal(t, "10386857", result.Jobs[0].IDIcims)
	assert.Equal(t, "Software Engineer", result.Jobs[0].Title)
	assert.Equal(t, "AU, NSW, Sydney", result.Jobs[0].Location)
	assert.Equal(t, "https://account.amazon.com/jobs/10386857/apply", result.Jobs[0].URLNextStep.String())
}

func TestSearchEncodesFilters(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/en/search.json", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		assert.Equal(t, []string{"TWN"}, query["normalized_country_code[]"])
		assert.Equal(t, []string{"Taipei City"}, query["normalized_city_name[]"])
		assert.Equal(t, []string{"Software Development"}, query["category[]"])
		assert.Equal(t, []string{"alexa-and-amazon-devices"}, query["business_category[]"])
		assert.Equal(t, []string{"Full-Time"}, query["schedule_type_id[]"])
		assert.Equal(t, "recent", query.Get("sort"))
		assert.Equal(t, "20", query.Get("offset"))
		assert.Equal(t, "2", query.Get("result_limit"))
		serveMockJSON(mockFilteredJobsResponse)(w, r)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	client, err := NewClient(server.URL, WithClient(server.Client()))
	require.NoError(t, err)

	result, err := client.Search(t.Context(), SearchRequest{
		Query:              "software engineer",
		Countries:          []string{"TWN"},
		Cities:             []string{"Taipei City"},
		JobCategories:      []string{"Software Development"},
		BusinessCategories: []string{"alexa-and-amazon-devices"},
		ScheduleTypes:      []string{"Full-Time"},
		Sort:               SearchJobsSortRecent,
		Offset:             20,
		Limit:              2,
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.Jobs)
	assert.Equal(t, "TWN", result.Jobs[0].CountryCode)
}

func TestSearchValidation(t *testing.T) {
	tests := []struct {
		name    string
		request SearchRequest
		want    string
	}{
		{name: "negative offset", request: SearchRequest{Offset: -1}, want: "offset must be at least 0"},
		{name: "negative limit", request: SearchRequest{Limit: -1}, want: "limit must be between 1 and 100"},
		{name: "limit too large", request: SearchRequest{Limit: 101}, want: "limit must be between 1 and 100"},
		{name: "unknown sort", request: SearchRequest{Sort: "invalid"}, want: `invalid sort "invalid"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, err := NewClient("https://www.amazon.jobs")
			require.NoError(t, err)
			_, err = client.Search(t.Context(), test.request)
			assert.ErrorContains(t, err, test.want)
		})
	}
}

func TestJobDetail(t *testing.T) {
	server := NewMockServer()
	t.Cleanup(server.Close)
	client, err := NewClient(server.URL, WithClient(server.Client()))
	require.NoError(t, err)

	job, err := client.JobDetail(t.Context(), "3164253")
	require.NoError(t, err)
	assert.Equal(t, "Software Dev Engineer, eero", job.Title)
	assert.Equal(t, "/en/jobs/3164253/software-dev-engineer-eero", job.JobPath)
	assert.NotEmpty(t, job.Description)
	assert.NotEmpty(t, job.BasicQualifications)
	assert.NotEmpty(t, job.PreferredQualifications)
}

func TestJobDetailNotFound(t *testing.T) {
	server := NewMockServer()
	t.Cleanup(server.Close)
	client, err := NewClient(server.URL, WithClient(server.Client()))
	require.NoError(t, err)

	_, err = client.JobDetail(t.Context(), "9999999999")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrJobNotFound))
}

func TestJobDetailRejectsInvalidID(t *testing.T) {
	client, err := NewClient("https://www.amazon.jobs")
	require.NoError(t, err)

	tests := []struct {
		name  string
		jobID string
	}{
		{name: "empty", jobID: ""},
		{name: "letters", jobID: "abc"},
		{name: "slash", jobID: "12/34"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := client.JobDetail(t.Context(), test.jobID)
			assert.ErrorContains(t, err, "invalid job id")
		})
	}
}

func TestSearchReportsUpstreamSoftError(t *testing.T) {
	server := NewMockServer()
	t.Cleanup(server.Close)
	client, err := NewClient(server.URL, WithClient(server.Client()))
	require.NoError(t, err)

	_, err = client.Search(t.Context(), SearchRequest{Query: "soft-error"})
	assert.ErrorContains(t, err, "upstream rejected request: result limit cannot be greater than 100")
}

func TestJobURL(t *testing.T) {
	assert.Equal(t, "https://www.amazon.jobs/en/jobs/3164253/software-dev-engineer-eero", JobURL("/en/jobs/3164253/software-dev-engineer-eero"))
	assert.Empty(t, JobURL(""))
	assert.Empty(t, JobURL("en/jobs/3164253/software-dev-engineer-eero"))
}
