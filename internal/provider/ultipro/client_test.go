package ultipro

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mockClient(t *testing.T) *Client {
	t.Helper()
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	return NewClient(srv.URL+"/"+MockCompanyCode+"/JobBoard/"+MockBoardID, srv.Client())
}

func TestSearch(t *testing.T) {
	c := mockClient(t)
	got, err := c.Search(t.Context(), SearchRequest{Top: 20})
	require.NoError(t, err)

	assert.Equal(t, 90, got.TotalCount)
	assert.Len(t, got.Opportunities, 20)
	first := got.Opportunities[0]
	assert.NotEmpty(t, first.ID)
	assert.NotEmpty(t, first.Title)
	assert.NotEmpty(t, first.Locations)
	assert.NotEmpty(t, first.Locations[0].Display())
}

func TestSearchFiltered(t *testing.T) {
	c := mockClient(t)
	got, err := c.Search(t.Context(), SearchRequest{
		Top:     20,
		Filters: []SearchFilter{{FieldName: 5, Values: []string{MockFilteredCategoryID}}},
	})
	require.NoError(t, err)
	require.NotEmpty(t, got.Opportunities)
	for _, o := range got.Opportunities {
		assert.Equal(t, "Finance", o.JobCategoryName)
	}
}

func TestSearchUnknownCompany(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL+"/"+MockUnknownCompanyCode+"/JobBoard/"+MockBoardID, srv.Client())

	_, err := c.Search(t.Context(), SearchRequest{Top: 20})
	require.ErrorIs(t, err, ErrCompanyNotFound)
}

func TestLocations(t *testing.T) {
	c := mockClient(t)
	got, err := c.Locations(t.Context())
	require.NoError(t, err)
	assert.Len(t, got, 44)
	for _, l := range got {
		assert.NotEmpty(t, l.ID)
		assert.NotEmpty(t, l.Label)
	}
}

func TestCategories(t *testing.T) {
	c := mockClient(t)
	got, err := c.Categories(t.Context())
	require.NoError(t, err)
	assert.Contains(t, got, FilterCatalog{ID: MockFilteredCategoryID, Label: "Finance"})
}

func TestDetail(t *testing.T) {
	c := mockClient(t)
	got, err := c.Detail(t.Context(), MockOpportunityID)
	require.NoError(t, err)
	assert.Equal(t, MockOpportunityID, got.ID)
	assert.Equal(t, "Conseiller Senior en Partenariat-BeniBiz", got.Title)
	assert.Contains(t, got.Description, "TechnoServe")
	assert.NotEmpty(t, got.Locations)
}

func TestDetailNotFound(t *testing.T) {
	c := mockClient(t)
	_, err := c.Detail(t.Context(), MockNotFoundOpportunityID)
	require.ErrorIs(t, err, ErrJobNotFound)
}
