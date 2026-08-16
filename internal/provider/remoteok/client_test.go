package remoteok

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var wantJob = Job{
	ID:          "1134996",
	Slug:        "remote-patient-access-scheduler-centralized-scheduling-ft-8-30a-5p-orchestrate-consulting-group-1134996",
	Epoch:       NewOptInt64(1784311668),
	Date:        NewOptString("2026-07-17T18:07:48+00:00"),
	Company:     NewOptString("Orchestrate Consulting Group"),
	CompanyLogo: NewOptString(""),
	Position:    NewOptString("Patient Access Scheduler Centralized Scheduling FT 8 30A 5P"),
	Tags: []string{
		"virtual assistant", "marketing", "exec", "content writing",
		"social media", "speech", "education", "technical",
		"customer support", "video", "medical", "design", "ads",
		"digital nomad", "consulting",
	},
	Description: NewOptString("<br/><br/>Please mention the word **SOOTHINGLY** and tag RNDkuMTU5LjQuODc= when applying to show you read the job post completely (#RNDkuMTU5LjQuODc=). This is a beta feature to avoid spam applicants. Companies can search these words to find applicants that read this and see they're human."),
	Location:    NewOptString("Los Angeles, "),
	ApplyURL:    NewOptString("https://remoteOK.com/remote-jobs/remote-patient-access-scheduler-centralized-scheduling-ft-8-30a-5p-orchestrate-consulting-group-1134996"),
	SalaryMin:   NewOptInt(0),
	SalaryMax:   NewOptInt(0),
	Logo:        NewOptString(""),
	URL:         NewOptString("https://remoteOK.com/remote-jobs/remote-patient-access-scheduler-centralized-scheduling-ft-8-30a-5p-orchestrate-consulting-group-1134996"),
}

func TestGetJobs(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	c, err := NewClient(srv.URL, WithClient(srv.Client()))
	require.NoError(t, err)

	feed, err := c.GetJobs(t.Context(), GetJobsParams{})
	require.NoError(t, err)
	require.Len(t, feed, 7)

	legal, ok := feed[0].GetLegalNotice()
	require.True(t, ok, "element 0 must be the legal notice")
	assert.Equal(t, int64(1784395677), legal.LastUpdated)
	assert.True(t, strings.HasPrefix(legal.Legal, "API Terms of Service:"))

	job, ok := feed[1].GetJob()
	require.True(t, ok, "element 1 must be a job")
	assert.Equal(t, wantJob, job)

	last, ok := feed[6].GetJob()
	require.True(t, ok)
	assert.Equal(t, "1134871", last.ID)
	assert.Equal(t, NewOptBool(true), last.Original)
}

// TestGetJobsTags pins the query encoding: multiple tags are comma-joined
// into a single tags parameter, the format the real feed ANDs.
func TestGetJobsTags(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, []string{"golang,react"}, r.URL.Query()["tags"])
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockJobsTagsRsp)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := NewClient(srv.URL, WithClient(srv.Client()))
	require.NoError(t, err)

	feed, err := c.GetJobs(t.Context(), GetJobsParams{Tags: []string{"golang", "react"}})
	require.NoError(t, err)
	require.Len(t, feed, 6)

	_, ok := feed[0].GetLegalNotice()
	require.True(t, ok, "element 0 must be the legal notice")
	for _, el := range feed[1:] {
		job, ok := el.GetJob()
		require.True(t, ok, "every element after the notice must be a job")
		assert.Contains(t, job.Tags, "golang")
	}
}

func TestGetJobsUnknownTag(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	c, err := NewClient(srv.URL, WithClient(srv.Client()))
	require.NoError(t, err)

	feed, err := c.GetJobs(t.Context(), GetJobsParams{Tags: []string{MockUnknownTag}})
	require.NoError(t, err)
	require.Len(t, feed, 1)
	assert.True(t, feed[0].IsLegalNotice())
}
