package successfactors

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearch(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.Search(t.Context(), &SearchRequest{Query: "engineer"})
	require.NoError(t, err)

	assert.Equal(t, 633, got.TotalCount)
	assert.Len(t, got.Jobs, 25)
	assert.Equal(t, Job{ID: "1414343333", Title: "Developer Associate", Location: "Bangalore, IN, 560066"}, got.Jobs[0])
}

func TestSearchFiltered(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.Search(t.Context(), &SearchRequest{
		Query: "engineer",
		Filters: map[string]string{
			"department": "Software-Design and Development",
			"country":    "DE",
		},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, got.Jobs)
}

func TestSearchNoResults(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.Search(t.Context(), &SearchRequest{Query: "zzzznonexistentkeyword12345"})
	require.NoError(t, err)
	assert.Empty(t, got.Jobs)
	assert.Equal(t, 0, got.TotalCount)
}

func TestJobDetail(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.JobDetail(t.Context(), "1414343333")
	require.NoError(t, err)

	assert.Equal(t, "1414343333", got.ID)
	assert.Equal(t, "Developer Associate", got.Title)
	assert.Equal(t, "Bangalore, IN, 560066", got.Location)
	assert.Equal(t, "SAP", got.Employer)
	assert.NotEmpty(t, got.DescriptionHTML)
}

func TestJobDetailNotFound(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	_, err := c.JobDetail(t.Context(), "999999999")
	require.ErrorIs(t, err, ErrJobNotFound)
}

// TestJobDetailOperationalFailureIsNotErrJobNotFound proves a 5xx (or any
// non-parse failure) is a distinct error from ErrJobNotFound, so callers
// checking errors.Is don't mistake an outage for an expired job ID.
func TestJobDetailOperationalFailureIsNotErrJobNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	_, err := c.JobDetail(t.Context(), "1")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrJobNotFound)
}

func TestJobDetailUnrecognized200IsNotErrJobNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><title>Maintenance</title><body>Try again later</body></html>`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	_, err := c.JobDetail(t.Context(), "1")
	require.ErrorContains(t, err, "unrecognized detail page")
	assert.NotErrorIs(t, err, ErrJobNotFound)
}

func TestFacetValues(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.FacetValues(t.Context(), &SearchRequest{Query: "engineer"})
	require.NoError(t, err)

	require.Contains(t, got.Facets, "country")
	require.Contains(t, got.Facets, "department")
	assert.Contains(t, got.Facets["country"], FacetOption{Name: "DE", Translated: "Germany", Count: 192})
	assert.Contains(t, got.Facets["department"], FacetOption{Name: "Software-Design and Development", Translated: "", Count: 208})
}
