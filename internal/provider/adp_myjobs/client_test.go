package adp_myjobs

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCareerSiteAndList(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)

	c := NewClient(Config{
		CareerSiteBase: srv.URL,
		ListingBase:    srv.URL,
		PageSize:       5,
	})
	cs, err := c.GetCareerSite(context.Background(), MockSlug)
	require.NoError(t, err)
	assert.Equal(t, "Guitar Center", cs.ClientName)
	assert.Equal(t, "TEST_MYJOBS_TOKEN", cs.MyJobsToken)

	page, err := c.ListJobRequisitions(context.Background(), MockSlug, ListParams{Top: 5})
	require.NoError(t, err)
	require.Equal(t, 2, page.Count)
	require.Len(t, page.JobRequisitions, 2)
}

func TestListSearchServerSide(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	c := NewClient(Config{CareerSiteBase: srv.URL, ListingBase: srv.URL})

	page, err := c.ListJobRequisitions(context.Background(), MockSlug, ListParams{Search: "teacher", Top: 10})
	require.NoError(t, err)
	require.Equal(t, 1, page.Count)
	require.Len(t, page.JobRequisitions, 1)
	assert.Contains(t, page.JobRequisitions[0].Title(), "Teacher")
}

func TestListSearchAndFilterServerSide(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	c := NewClient(Config{CareerSiteBase: srv.URL, ListingBase: srv.URL})

	page, err := c.ListJobRequisitions(context.Background(), MockSlug, ListParams{
		Search: "teacher",
		Filter: "requisitionLocations/itemID eq loc-2",
		Top:    10,
	})
	require.NoError(t, err)
	require.Equal(t, 1, page.Count)
	assert.Contains(t, page.JobRequisitions[0].Title(), "Teacher")
}

func TestListSearchAndFilterEncodeBothQueryOptions(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	recorder := &recordingRoundTripper{}
	c := NewClient(Config{
		HTTPClient:     &http.Client{Transport: recorder},
		CareerSiteBase: srv.URL,
		ListingBase:    srv.URL,
	})

	_, err := c.ListJobRequisitions(context.Background(), MockSlug, ListParams{
		Search: "teacher",
		Filter: "requisitionLocations/itemID eq loc-2",
		Top:    10,
	})
	require.NoError(t, err)
	require.Len(t, recorder.requests, 2)
	query := recorder.requests[1].URL.Query()
	assert.Equal(t, "teacher", query.Get("$search"))
	assert.Equal(t, "requisitionLocations/itemID eq loc-2", query.Get("$filter"))
}

func TestGetJobRequisition(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	c := NewClient(Config{CareerSiteBase: srv.URL, ListingBase: srv.URL})

	j, err := c.GetJobRequisition(context.Background(), MockSlug, "1002")
	require.NoError(t, err)
	assert.Equal(t, "1002", j.ReqIDString())
	assert.Contains(t, j.Title(), "Teacher")
	assert.NotEmpty(t, j.JobDescription)
}

func TestUnknownCareerSite(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	c := NewClient(Config{CareerSiteBase: srv.URL, ListingBase: srv.URL})
	_, err := c.GetCareerSite(context.Background(), MockUnknownSlug)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 400")
}

type recordingRoundTripper struct {
	requests []*http.Request
}

func (r *recordingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	r.requests = append(r.requests, req)
	return http.DefaultTransport.RoundTrip(req)
}
