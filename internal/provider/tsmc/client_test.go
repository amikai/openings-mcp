package tsmc

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobs(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}

	got, err := c.Jobs(t.Context(), &JobsRequest{
		Keyword:         "engineer",
		Locations:       []string{LocTaiwan},
		Categories:      []string{CatRD},
		JobTypes:        []string{JobTypeEngineer},
		EmploymentTypes: []string{EmployRegular},
	})
	require.NoError(t, err)

	want := &JobsResponse{Total: 22, Jobs: wantJobs}
	assert.Equal(t, want, got)
}

func TestJobDetail(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}

	got, err := c.JobDetail(t.Context(), "21826")
	require.NoError(t, err)

	assert.Equal(t, wantDetail, got)
}

func TestJobDetailRejectsEmptyID(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}

	_, err := c.JobDetail(t.Context(), "")
	require.Error(t, err)
}

// The mock serves job 21826's page for every jobId; a canonical-ID mismatch
// must fail instead of relabeling 21826's content as the requested job.
func TestJobDetailRejectsMismatchedCanonicalID(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}

	_, err := c.JobDetail(t.Context(), "999")
	require.ErrorContains(t, err, "21826")
}

func TestJobsURLEncodesKeywordAsOneSegment(t *testing.T) {
	c := NewClient("https://careers.tsmc.com", nil)

	raw, err := c.jobsURL(&JobsRequest{Keyword: "A10/A14"})
	require.NoError(t, err)
	assert.Contains(t, raw, "/SearchJobs/A10%2FA14")

	for _, kw := range []string{".", ".."} {
		_, err := c.jobsURL(&JobsRequest{Keyword: kw})
		assert.Errorf(t, err, "keyword %q must not become a path traversal", kw)
	}
}

func TestJobsURLRejectsOverflowingPage(t *testing.T) {
	c := NewClient("https://careers.tsmc.com", nil)
	_, err := c.jobsURL(&JobsRequest{Page: math.MaxInt})
	require.Error(t, err)
}
