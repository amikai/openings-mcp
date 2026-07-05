package workday

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req JobsRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		wantReq := JobsRequest{
			AppliedFacets: AppliedFacets{"jobFamilyGroup": {"cat-eng-id"}},
			Limit:         20,
			Offset:        0,
			SearchText:    "engineer",
		}
		assert.Equal(t, wantReq, req)

		serveMockJSON(mockJobsRsp)(w, r)
	})

	mux.HandleFunc("/job/US-CA-Remote/Backend-Engineer_REQ001", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		serveMockJSON(mockJobDetailRsp)(w, r)
	})

	return httptest.NewServer(mux)
}

func TestSearchJobs(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	resp, err := client.SearchJobs(context.Background(), &JobsRequest{
		AppliedFacets: AppliedFacets{"jobFamilyGroup": {"cat-eng-id"}},
		Limit:         20,
		Offset:        0,
		SearchText:    "engineer",
	})
	require.NoError(t, err)

	assert.Equal(t, 2, resp.Total)
	require.Len(t, resp.JobPostings, 2)

	first := resp.JobPostings[0]
	assert.Equal(t, "Backend Engineer", first.Title.Value)
	assert.Equal(t, "/job/US-CA-Remote/Backend-Engineer_REQ001", first.ExternalPath.Value)
	assert.Equal(t, []string{"REQ001"}, first.BulletFields)

	// Second posting is a sparse entry (no title/externalPath) — mirrors
	// Workday occasionally omitting them; only BulletFields must survive.
	second := resp.JobPostings[1]
	assert.False(t, second.Title.Set)
	assert.False(t, second.ExternalPath.Set)
	assert.Equal(t, []string{"REQ002"}, second.BulletFields)

	require.Len(t, resp.Facets, 2)

	jobCategory := resp.Facets[0]
	assert.Equal(t, "jobFamilyGroup", jobCategory.FacetParameter.Value)
	assert.Equal(t, "Job Category", jobCategory.Descriptor.Value)
	require.Len(t, jobCategory.Values, 2)
	assert.Equal(t, "Engineering", jobCategory.Values[0].Descriptor.Value)
	assert.Equal(t, "cat-eng-id", jobCategory.Values[0].ID.Value)
	assert.Equal(t, 1, jobCategory.Values[0].Count.Value)

	locationGroup := resp.Facets[1]
	assert.Equal(t, "locationMainGroup", locationGroup.FacetParameter.Value)
	assert.False(t, locationGroup.Descriptor.Set) // top-level group with no UI label, mirrors NVIDIA's locationMainGroup
	require.Len(t, locationGroup.Values, 1)
	nested := locationGroup.Values[0]
	assert.Equal(t, "locationHierarchy1", nested.FacetParameter.Value)
	require.Len(t, nested.Values, 1)
	assert.Equal(t, "United States", nested.Values[0].Descriptor.Value)
}

func TestGetJobDetail(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	detail, err := client.GetJobDetail(context.Background(), GetJobDetailParams{
		Location:  "US-CA-Remote",
		TitleSlug: "Backend-Engineer_REQ001",
	})
	require.NoError(t, err)

	info := detail.JobPostingInfo
	assert.Equal(t, "Backend Engineer", info.Title)
	assert.Equal(t, "US, CA, Remote", info.Location.Value)
	assert.Equal(t, "Full time", info.TimeType.Value)
	assert.Equal(t, "REQ001", info.JobReqId.Value)
	assert.Contains(t, info.JobDescription, "Build things")
}
