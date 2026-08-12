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
	require.Equal(t, 3, page.Count)
	require.Len(t, page.JobRequisitions, 3)
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

func TestGeoBoxFilterMatchesUpstreamShape(t *testing.T) {
	box := NewGeoBox(-118.36, 34.13, -118.28, 34.07)
	// west/south first, then east/north, with the "undefined" tokens upstream's
	// parser requires. Diverging from this string is answered with HTTP 500.
	assert.Equal(t,
		"geo.intersects(workLocations.geoLocation, geography'POLYGON((undefined, "+
			"-118.36 34.07, undefined, -118.28 34.13))')",
		box.Filter(),
	)
}

func TestGeoBoxNormalizesCornersAndPads(t *testing.T) {
	box := NewGeoBox(10, 20, -10, -20)
	assert.Equal(t, GeoBox{West: -10, South: -20, East: 10, North: 20}, box)

	// A box around one point has zero area, which upstream rejects; Pad is how
	// callers make it sendable, and must not switch to exponent notation.
	point := NewGeoBox(-74.890955, 40.177719, -74.890955, 40.177719).Pad(0.0001)
	assert.Contains(t, point.Filter(), "-74.891055 40.177619")
	assert.NotContains(t, point.Filter(), "e-")
}

func TestListSearchAndGeoBoxServerSide(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	c := NewClient(Config{CareerSiteBase: srv.URL, ListingBase: srv.URL})

	orem := NewGeoBox(MockOremLon, MockOremLat, MockOremLon, MockOremLat).Pad(0.01)
	page, err := c.ListJobRequisitions(context.Background(), MockSlug, ListParams{
		GeoBox: &orem,
		Top:    10,
	})
	require.NoError(t, err)
	require.Equal(t, 1, page.Count)
	assert.Contains(t, page.JobRequisitions[0].Title(), "Teacher")

	// $search and the box are both applied upstream, in the same request.
	page, err = c.ListJobRequisitions(context.Background(), MockSlug, ListParams{
		Search: "engineer",
		GeoBox: &orem,
		Top:    10,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, page.Count)
}

func TestListSearchAndGeoBoxEncodeBothQueryOptions(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	recorder := &recordingRoundTripper{}
	c := NewClient(Config{
		HTTPClient:     &http.Client{Transport: recorder},
		CareerSiteBase: srv.URL,
		ListingBase:    srv.URL,
	})

	box := NewGeoBox(-118.36, 34.07, -118.28, 34.13)
	_, err := c.ListJobRequisitions(context.Background(), MockSlug, ListParams{
		Search: "teacher",
		GeoBox: &box,
		Top:    10,
	})
	require.NoError(t, err)
	require.Len(t, recorder.requests, 2)
	query := recorder.requests[1].URL.Query()
	assert.Equal(t, "teacher", query.Get("$search"))
	assert.Equal(t, box.Filter(), query.Get("$filter"))
}

func TestListZeroAreaGeoBoxRejectedUpstream(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	c := NewClient(Config{CareerSiteBase: srv.URL, ListingBase: srv.URL})

	unpadded := NewGeoBox(MockOremLon, MockOremLat, MockOremLon, MockOremLat)
	_, err := c.ListJobRequisitions(context.Background(), MockSlug, ListParams{
		GeoBox: &unpadded,
		Top:    10,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 500")
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
