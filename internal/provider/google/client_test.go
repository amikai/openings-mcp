package google

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobs(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.Jobs(t.Context(), &JobsRequest{
		Query:          "software engineer",
		Locations:      []string{"Taiwan"},
		EmploymentType: []string{"FULL_TIME"},
		SortBy:         "date",
		Page:           1,
	})
	require.NoError(t, err)

	assert.Equal(t, &JobsResponse{Jobs: wantJobs}, got)
}

func TestJobDetail(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.JobDetail(t.Context(), "106863362666570438")
	require.NoError(t, err)

	assert.Equal(t, wantDetail, got)
}
