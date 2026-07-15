package icims

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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

	// "US" matches Austin and Lorton options; encoded searches keep both
	// cities in option order (Austin jobs, then Lorton jobs).
	got, err := c.Search(t.Context(), &SearchRequest{Location: "US"})
	require.NoError(t, err)
	require.Len(t, got.Jobs, 3)
	ids := []string{got.Jobs[0].ID, got.Jobs[1].ID, got.Jobs[2].ID}
	assert.Equal(t, []string{"1977", "1922", "1925"}, ids)
}

func TestSearchLocationMultiMatchUsesEncodedOptions(t *testing.T) {
	// Some tenants expose a country token in option labels while job cards
	// only show city/state. Filtering unfiltered cards by the original "US"
	// text would therefore drop both valid jobs; encoded option searches must
	// remain the source of truth.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		location := r.URL.Query().Get("searchLocation")
		jobs := []string{
			jobCardHTML("1", "Austin role", "Austin, TX"),
			jobCardHTML("2", "Lorton role", "Lorton, VA"),
		}
		switch location {
		case "1-1-Austin":
			jobs = jobs[:1]
		case "1-2-Lorton":
			jobs = jobs[1:]
		}
		writeSearchHTML(w, []string{
			`<option value="1-1-Austin">TX Austin US</option>`,
			`<option value="1-2-Lorton">VA Lorton US</option>`,
		}, jobs, 1)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, srv.Client())
	got, err := c.Search(t.Context(), &SearchRequest{Location: "US"})
	require.NoError(t, err)
	require.Len(t, got.Jobs, 2)
	assert.Equal(t, []string{"1", "2"}, []string{got.Jobs[0].ID, got.Jobs[1].ID})
}

func TestSearchLocationTooBroadStopsAfterProbe(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		options := make([]string, 11)
		for i := range options {
			options[i] = fmt.Sprintf(`<option value="1-%d-City">ST City %d US</option>`, i, i)
		}
		writeSearchHTML(w, options, nil, 1)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, srv.Client())
	_, err := c.Search(t.Context(), &SearchRequest{Location: "US"})
	require.ErrorIs(t, err, ErrLocationTooBroad)
	assert.Equal(t, int32(1), requests.Load())
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
	assert.Equal(t, []string{"1977", "1922", "1925"}, []string{jobs[0].ID, jobs[1].ID, jobs[2].ID})
}

func TestSearchAllForLocationsCapsRequests(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		writeSearchHTML(w,
			[]string{`<option value="1-1-Austin">TX Austin US</option>`},
			[]string{jobCardHTML("1", "Job 1", "Austin, TX")},
			99,
		)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	_, _, err := c.SearchAllForLocations(t.Context(), "", []string{"1-1-Austin"})
	require.ErrorIs(t, err, ErrLocationRequestLimit)
	assert.Equal(t, int32(1), requests.Load())
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

func jobCardHTML(id, title, location string) string {
	return fmt.Sprintf(`<li class="iCIMS_JobCardItem">
  <span class="sr-only field-label">Location</span><span>%s</span>
  <a class="iCIMS_Anchor" href="/jobs/%s/job-title/job"><h3>%s</h3></a>
</li>`, location, id, title)
}

func writeSearchHTML(w http.ResponseWriter, options, jobs []string, totalPages int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<!DOCTYPE html><html><body>
<form id="searchForm"><input name="searchKeyword"/>
<select name="searchLocation">%s</select></form>
<div class="iCIMS_PagingBatch">Page 1 of %d</div>
<ul class="iCIMS_JobsTable">%s</ul>
</body></html>`, strings.Join(options, ""), totalPages, strings.Join(jobs, ""))
}
