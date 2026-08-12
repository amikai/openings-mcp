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

func TestCustomFilterExprShape(t *testing.T) {
	// The slot code is the field name and clauses AND with "&&"; upstream
	// answers any other shape with the whole board.
	assert.Equal(t,
		"FIELD1 eq 'Pennsylvania' && FIELD2 eq 'Full-time'",
		filterExpr([]CustomFilter{
			{Field: "FIELD1", Value: "Pennsylvania"},
			{Field: "FIELD2", Value: "Full-time"},
		}),
	)
}

func TestGetCustomFilters(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	c := NewClient(Config{CareerSiteBase: srv.URL, ListingBase: srv.URL})

	catalog, err := c.GetCustomFilters(context.Background(), MockSlug)
	require.NoError(t, err)
	require.Len(t, catalog.FilterList, 3)
	assert.Equal(t, "FIELD1", catalog.FilterList[0].Category)
	assert.Equal(t, "State", catalog.FilterList[0].CategoryLabel)
	assert.Equal(t, "Pennsylvania", catalog.FilterList[0].FilterList[0].Value)
}

func TestListWithCustomFilters(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	c := NewClient(Config{CareerSiteBase: srv.URL, ListingBase: srv.URL})
	ctx := context.Background()

	page, err := c.ListJobRequisitions(ctx, MockSlug, ListParams{
		CustomFilters: []CustomFilter{{Field: "FIELD1", Value: "Utah"}},
		Top:           10,
	})
	require.NoError(t, err)
	require.Equal(t, 1, page.Count)
	assert.Contains(t, page.JobRequisitions[0].Title(), "Teacher")

	// Clauses AND, so a combination no job satisfies is empty.
	page, err = c.ListJobRequisitions(ctx, MockSlug, ListParams{
		CustomFilters: []CustomFilter{
			{Field: "FIELD1", Value: "Utah"},
			{Field: "FIELD2", Value: "Full-time"},
		},
		Top: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, page.Count)

	// $search and a filter are applied together, in one request.
	page, err = c.ListJobRequisitions(ctx, MockSlug, ListParams{
		Search:        "teacher",
		CustomFilters: []CustomFilter{{Field: "FIELD1", Value: "Utah"}},
		Top:           10,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, page.Count)

	// A value whose case is off matches nothing rather than erroring, which is
	// why the adapter canonicalizes before sending.
	page, err = c.ListJobRequisitions(ctx, MockSlug, ListParams{
		CustomFilters: []CustomFilter{{Field: "FIELD1", Value: "utah"}},
		Top:           10,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, page.Count)
}

func TestListUnconfiguredFieldReturnsWholeBoard(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	c := NewClient(Config{CareerSiteBase: srv.URL, ListingBase: srv.URL})

	// The trap this provider is built around: upstream does not reject a slot
	// code the board never configured, it ignores the filter entirely.
	page, err := c.ListJobRequisitions(context.Background(), MockSlug, ListParams{
		CustomFilters: []CustomFilter{{Field: "FIELD9", Value: "anything"}},
		Top:           10,
	})
	require.NoError(t, err)
	assert.Equal(t, 3, page.Count)
}

func TestListEncodesSpacesAsPercent20(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	recorder := &recordingRoundTripper{}
	c := NewClient(Config{
		HTTPClient:     &http.Client{Transport: recorder},
		CareerSiteBase: srv.URL,
		ListingBase:    srv.URL,
	})

	_, err := c.ListJobRequisitions(context.Background(), MockSlug, ListParams{
		Search:        "teacher",
		CustomFilters: []CustomFilter{{Field: "FIELD1", Value: "Pennsylvania"}},
		Top:           10,
	})
	require.NoError(t, err)
	require.Len(t, recorder.requests, 2)

	// Upstream reads a "+" in $filter as part of the value, not as a space, and
	// answers the whole unfiltered board. Every facet value with a space would
	// silently stop filtering, so the raw query must carry %20.
	raw := recorder.requests[1].URL.RawQuery
	assert.Contains(t, raw, "%24filter=FIELD1%20eq%20%27Pennsylvania%27")
	assert.NotContains(t, raw, "+eq+")

	query := recorder.requests[1].URL.Query()
	assert.Equal(t, "teacher", query.Get("$search"))
	assert.Equal(t, "FIELD1 eq 'Pennsylvania'", query.Get("$filter"))
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
