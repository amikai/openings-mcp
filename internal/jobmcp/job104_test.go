package jobmcp

import (
	"testing"

	"github.com/amikai/job-mcp/internal/provider/job104"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterJob104(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)

	client, err := job104.NewClient("https://www.104.com.tw")
	require.NoError(t, err)
	RegisterJob104(server, client)

	assertTools(t, server, "104_search_jobs", "104_get_job_detail")
}

func TestJob104ToRequest(t *testing.T) {
	in := job104SearchInput{
		Keyword: "golang",
		Area:    "Taipei",
		JobType: "Part-time",
		Sort:    "Newest",
		Remote:  "Full",
		Edu:     []string{"University", "Master"},
		Shift:   []string{"Day", "Holiday"},
		Page:    2,
	}
	got, err := job104ToRequest(in)
	require.NoError(t, err)

	want := job104.SearchJobsParams{
		Keyword:    job104.NewOptString("golang"),
		Area:       job104.NewOptSearchJobsArea(job104.AreaIDs["Taipei"]),
		Ro:         job104.NewOptSearchJobsRo(job104.SearchJobsRo2),
		Order:      job104.NewOptSearchJobsOrder(job104.SearchJobsOrder2),
		RemoteWork: job104.NewOptSearchJobsRemoteWork(job104.SearchJobsRemoteWork1),
		Page:       job104.NewOptInt(2),
		Edu:        []job104.SearchJobsEduItem{job104.SearchJobsEduItem4, job104.SearchJobsEduItem5},
		S9:         []job104.SearchJobsS9Item{job104.SearchJobsS9Item1, job104.SearchJobsS9Item8},
	}
	assert.Equal(t, want, got)
}

func TestJob104ToRequestEmpty(t *testing.T) {
	got, err := job104ToRequest(job104SearchInput{})
	require.NoError(t, err)
	assert.Equal(t, job104.SearchJobsParams{}, got)
}

func TestJob104ToRequestInvalidLabels(t *testing.T) {
	cases := []struct {
		name string
		in   job104SearchInput
		want string
	}{
		{"area", job104SearchInput{Area: "Mars"}, `invalid area "Mars"`},
		{"job_type", job104SearchInput{JobType: "full"}, `invalid job_type "full"`},
		{"sort", job104SearchInput{Sort: "newest"}, `invalid sort "newest"`},
		{"remote", job104SearchInput{Remote: "hybrid"}, `invalid remote "hybrid"`},
		{"edu", job104SearchInput{Edu: []string{"University", "PhD"}}, `invalid edu "PhD"`},
		{"shift", job104SearchInput{Shift: []string{"Midnight"}}, `invalid shift "Midnight"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := job104ToRequest(tc.in)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}
