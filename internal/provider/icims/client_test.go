package icims

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

	got, err := c.Search(t.Context(), &SearchRequest{})
	require.NoError(t, err)

	assert.Equal(t, 1, got.TotalPages)
	assert.Len(t, got.Jobs, 3)
	assert.Equal(t, 3, got.PageSize)
	assert.Equal(t, "1977", got.Jobs[0].ID)
	assert.Equal(t, "Senior Product Manager", got.Jobs[0].Title)
	assert.Contains(t, got.Jobs[0].Location, "Austin")
	assert.Equal(t, "senior-product-manager", got.Jobs[0].Slug)
}

func TestSearchFiltered(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.Search(t.Context(), &SearchRequest{Keyword: "Product"})
	require.NoError(t, err)
	assert.NotEmpty(t, got.Jobs)
	assert.LessOrEqual(t, len(got.Jobs), 3)
}

func TestSearchLocationFreeText(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	// Free text must resolve to the encoded option value; mock only returns
	// the Austin fixture when searchLocation contains "Austin".
	got, err := c.Search(t.Context(), &SearchRequest{Location: "Austin"})
	require.NoError(t, err)
	require.Len(t, got.Jobs, 2)
	for _, j := range got.Jobs {
		assert.Contains(t, j.Location, "Austin")
		assert.NotContains(t, j.Location, "Lorton")
	}
	assert.Equal(t, []string{"1977", "1922"}, []string{got.Jobs[0].ID, got.Jobs[1].ID})
}

func TestSearchLocationEncodedValue(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.Search(t.Context(), &SearchRequest{Location: "12781-12827-Austin"})
	require.NoError(t, err)
	require.Len(t, got.Jobs, 2)
	assert.Equal(t, "1977", got.Jobs[0].ID)
}

func TestSearchLocationUnknown(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.Search(t.Context(), &SearchRequest{Location: "Seattle"})
	require.NoError(t, err)
	assert.Empty(t, got.Jobs)
	assert.Equal(t, 0, got.PageSize)
}

func TestSearchLocationMultiMatchFansOut(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	// "US" matches Austin and Lorton options; fan-out must keep both cities.
	got, err := c.Search(t.Context(), &SearchRequest{Location: "US"})
	require.NoError(t, err)
	require.Len(t, got.Jobs, 3)
	ids := []string{got.Jobs[0].ID, got.Jobs[1].ID, got.Jobs[2].ID}
	assert.Equal(t, []string{"1977", "1922", "1925"}, ids)
}

func TestSearchAllForLocations(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	jobs, _, err := c.SearchAllForLocations(t.Context(), "", []string{
		"12781-12827-Austin",
		"12781-12830-Lorton",
	})
	require.NoError(t, err)
	require.Len(t, jobs, 3)
	assert.Equal(t, "1977", jobs[0].ID)
	assert.Equal(t, "1925", jobs[2].ID)
}

func TestSearchNoResults(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.Search(t.Context(), &SearchRequest{Keyword: "zzzznonexistentkeyword12345"})
	require.NoError(t, err)
	assert.Empty(t, got.Jobs)
	assert.Equal(t, 1, got.TotalPages)
}

func TestSearchUnknownCompany(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL+"/unknown", srv.Client())

	_, err := c.Search(t.Context(), &SearchRequest{})
	require.ErrorIs(t, err, ErrCompanyNotFound)
}

func TestJobDetail(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.JobDetail(t.Context(), "1977")
	require.NoError(t, err)

	assert.Equal(t, "1977", got.ID)
	assert.Equal(t, "Senior Product Manager", got.Title)
	assert.Contains(t, got.Location, "Austin")
	assert.NotEmpty(t, got.DescriptionHTML)
	assert.Equal(t, "FULL_TIME", got.EmploymentType)
	assert.NotEmpty(t, got.PostedAtRaw)
}

func TestJobDetailNotFound(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	_, err := c.JobDetail(t.Context(), "999999999")
	require.ErrorIs(t, err, ErrJobNotFound)
}

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
		_, _ = w.Write([]byte(`<html><title>Maintenance</title><body>Try again later</body></html>`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	_, err := c.JobDetail(t.Context(), "1")
	require.ErrorContains(t, err, "unrecognized detail page")
	assert.NotErrorIs(t, err, ErrJobNotFound)
}

func TestJobDetailRejectsNonNumericID(t *testing.T) {
	c := NewClient("https://example.icims.com", http.DefaultClient)
	_, err := c.JobDetail(t.Context(), "abc")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "numeric")
}

func TestJobURL(t *testing.T) {
	assert.Equal(t, "https://careers-buspatrol.icims.com/jobs/1977/job/job", JobURL("careers-buspatrol.icims.com", "1977"))
}
