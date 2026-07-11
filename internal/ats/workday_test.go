package ats

import (
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/workday"
)

// recordingProxy forwards every request to inner and keeps the bodies, so
// tests can assert how many upstream calls a Search made and what the real
// (second) search request contained.
func recordingProxy(t *testing.T, inner string) (*httptest.Server, *[][]byte) {
	t.Helper()
	var bodies [][]byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodies = append(bodies, body)
		req, err := http.NewRequestWithContext(r.Context(), r.Method, inner+r.URL.Path, strings.NewReader(string(body)))
		if !assert.NoError(t, err, "proxy") {
			return
		}
		req.Header = r.Header.Clone()
		rsp, err := http.DefaultClient.Do(req)
		if !assert.NoError(t, err, "proxy") {
			return
		}
		defer rsp.Body.Close()
		w.Header().Set("Content-Type", rsp.Header.Get("Content-Type"))
		w.WriteHeader(rsp.StatusCode)
		io.Copy(w, rsp.Body)
	}))
	t.Cleanup(srv.Close)
	return srv, &bodies
}

func testWorkdayAdapter(t *testing.T) (*WorkdayAdapter, *[][]byte) {
	t.Helper()
	mock := workday.NewMockServer(workday.MockNvidiaJobsRsp, workday.MockNvidiaJobDetailRsp)
	t.Cleanup(mock.Close)
	proxy, bodies := recordingProxy(t, mock.URL)
	a := NewWorkdayAdapter(&http.Client{Timeout: 5 * time.Second})
	a.baseURL = func(workday.Company) string { return proxy.URL }
	a.siteBaseURL = func(workday.CareersSite) string { return proxy.URL }
	return a, bodies
}

func TestWorkdayRosterDedupesShareClasses(t *testing.T) {
	a := NewWorkdayAdapter(http.DefaultClient)
	seen := map[string]bool{}
	for _, c := range a.Roster() {
		assert.Equal(t, strings.ToLower(c.Slug), c.Slug)
		require.Falsef(t, seen[c.Slug], "duplicate slug %q in roster", c.Slug)
		seen[c.Slug] = true
	}
	// fox and dowjones each occupy two share-class rows in companies.yaml
	// sharing one tenant; the roster must carry each slug once.
	assert.True(t, seen["fox"], "expected fox slug present exactly once")
	assert.True(t, seen["dowjones"], "expected dowjones slug present exactly once")
}

func TestWorkdayRosterBuildsRegistry(t *testing.T) {
	_, err := NewRegistry(NewWorkdayAdapter(http.DefaultClient))
	require.NoError(t, err)
}

func TestWorkdayRosterReturnsIndependentSlice(t *testing.T) {
	a := NewWorkdayAdapter(http.DefaultClient)
	roster := a.Roster()
	require.NotEmpty(t, roster)
	original := roster[0]
	roster[0] = CompanyInfo{Slug: "mutated", Name: "Mutated"}
	assert.Equal(t, original, a.Roster()[0])
}

func TestWorkdaySearchPlainIsOneRequest(t *testing.T) {
	a, bodies := testWorkdayAdapter(t)
	res, err := a.Search(t.Context(), "nvidia", SearchParams{Query: "golang"})
	require.NoError(t, err)
	require.Len(t, *bodies, 1, "plain search should be 1 upstream request")
	assert.Equal(t, 27, res.TotalCount)
	assert.Equal(t, 2, res.TotalPages)
	assert.Len(t, res.Jobs, 20)

	first := res.Jobs[0]
	assert.Equal(t, "Software Golang Kubernetes Engineer", first.Title)
	assert.Equal(t, "/job/Israel-Tel-Aviv/Software-Golang-Kubernetes-Engineer_JR2020442", first.JobID)
}

// Workday sometimes sends "externalPath": "" instead of omitting the
// field; both shapes mean the posting has no fetchable path and must be
// skipped, or Search would hand out a job_id that Detail rejects.
func TestWorkdaySearchSkipsEmptyExternalPath(t *testing.T) {
	rsp := []byte(`{
		"total": 3,
		"jobPostings": [
			{"title": "Kept", "externalPath": "/job/Somewhere/Kept_JR1", "locationsText": "Somewhere", "postedOn": "Posted Today"},
			{"title": "Empty Path", "externalPath": "", "locationsText": "Somewhere", "postedOn": "Posted Today"},
			{"title": "No Path", "locationsText": "Somewhere", "postedOn": "Posted Today"}
		]
	}`)
	mock := workday.NewMockServer(rsp, nil)
	t.Cleanup(mock.Close)
	a := NewWorkdayAdapter(&http.Client{Timeout: 5 * time.Second})
	a.baseURL = func(workday.Company) string { return mock.URL }
	res, err := a.Search(t.Context(), "nvidia", SearchParams{})
	require.NoError(t, err)
	require.Len(t, res.Jobs, 1)
	assert.Equal(t, "/job/Somewhere/Kept_JR1", res.Jobs[0].JobID)
}

func TestWorkdaySearchRejectsHugePage(t *testing.T) {
	a, bodies := testWorkdayAdapter(t)
	_, err := a.Search(t.Context(), "nvidia", SearchParams{Page: math.MaxInt})
	require.ErrorContains(t, err, "smaller page")
	assert.Empty(t, *bodies)
}

func TestWorkdaySearchWithFiltersIsTwoRequests(t *testing.T) {
	a, bodies := testWorkdayAdapter(t)
	_, err := a.Search(t.Context(), "nvidia", SearchParams{
		Filters: map[string][]string{"timeType": {"Full time"}},
	})
	require.NoError(t, err)
	require.Len(t, *bodies, 2, "filtered search should probe then search")

	var real struct {
		AppliedFacets map[string][]string `json:"appliedFacets"`
	}
	require.NoError(t, json.Unmarshal((*bodies)[1], &real))
	require.Len(t, real.AppliedFacets["timeType"], 1)
	assert.Equal(t, "5509c0b5959810ac0029943377d47364", real.AppliedFacets["timeType"][0], "want the Full time GUID")
}

func TestWorkdaySearchLocationResolvesToFacet(t *testing.T) {
	a, bodies := testWorkdayAdapter(t)
	_, err := a.Search(t.Context(), "nvidia", SearchParams{Location: "Tel Aviv"})
	require.NoError(t, err)

	var real struct {
		AppliedFacets map[string][]string `json:"appliedFacets"`
	}
	require.NoError(t, json.Unmarshal((*bodies)[1], &real))
	require.Len(t, real.AppliedFacets["locations"], 1)
	assert.Equal(t, "c7769ee377291036b08490819096b8bf", real.AppliedFacets["locations"][0], `want the "Israel, Tel Aviv" GUID`)
}

func TestWorkdaySearchWhitespaceLocationIsPlainSearch(t *testing.T) {
	a, bodies := testWorkdayAdapter(t)
	res, err := a.Search(t.Context(), "nvidia", SearchParams{Location: "   "})
	require.NoError(t, err)
	require.Len(t, *bodies, 1, "whitespace location should not trigger a facet probe")
	assert.Equal(t, 27, res.TotalCount)
}

func TestWorkdaySearchLocationConflictingFilterErrors(t *testing.T) {
	a, bodies := testWorkdayAdapter(t)
	_, err := a.Search(t.Context(), "nvidia", SearchParams{
		Location: "Tel Aviv",
		Filters:  map[string][]string{"locations": {"Israel, Tel Aviv"}},
	})
	require.ErrorContains(t, err, "locations", "conflicting location facet should be rejected, not OR-ed")
	assert.Len(t, *bodies, 1, "the conflict should fail before the real search request")
}

func TestWorkdayFilterValueNotFoundTeaches(t *testing.T) {
	a, _ := testWorkdayAdapter(t)
	_, err := a.Search(t.Context(), "nvidia", SearchParams{
		Filters: map[string][]string{"timeType": {"Part time"}},
	})
	require.ErrorContains(t, err, "Full time", "error should list available values")
}

func TestWorkdayFilterKeyNotFoundTeaches(t *testing.T) {
	a, _ := testWorkdayAdapter(t)
	_, err := a.Search(t.Context(), "nvidia", SearchParams{
		Filters: map[string][]string{"bogus": {"x"}},
	})
	require.ErrorContains(t, err, "jobFamilyGroup", "error should list valid keys")
}

func TestWorkdayFilters(t *testing.T) {
	a, _ := testWorkdayAdapter(t)
	fs, err := a.Filters(t.Context(), "nvidia")
	require.NoError(t, err)
	require.NotEmptyf(t, fs["jobFamilyGroup"], "FilterSet missing expected dimensions: %v", fs)
	require.NotEmptyf(t, fs["timeType"], "FilterSet missing expected dimensions: %v", fs)
	assert.Contains(t, fs["jobFamilyGroup"], "Engineering")
}

func TestWorkdayDetail(t *testing.T) {
	a, _ := testWorkdayAdapter(t)
	d, err := a.Detail(t.Context(), "nvidia", "/job/Israel-Tel-Aviv/Software-Golang-Kubernetes-Engineer_JR2020442")
	require.NoError(t, err)
	assert.Equal(t, "Senior Software Golang Kubernetes Engineer", d.Title)
	assert.NotContains(t, d.Description, "<p>", "Description should be converted from HTML")
	assert.Contains(t, d.Description, "NVIDIA Networking", "Description should carry the fixture text")
}

func TestWorkdayDetailRejectsMalformedJobID(t *testing.T) {
	a, _ := testWorkdayAdapter(t)
	_, err := a.Detail(t.Context(), "nvidia", "garbage")
	assert.Error(t, err, "want error for malformed job_id")
}

func TestWorkdayParseCareersURL(t *testing.T) {
	a := NewWorkdayAdapter(http.DefaultClient)

	// A roster tenant folds back to its roster key, keeping display names.
	slug, ok := a.ParseCareersURL(mustParseURL(t, "https://nvidia.wd5.myworkdayjobs.com/en-US/NVIDIAExternalCareerSite"))
	require.True(t, ok)
	assert.Equal(t, "nvidia", slug)

	// An unknown tenant gets the canonical URL as a self-describing slug.
	slug, ok = a.ParseCareersURL(mustParseURL(t, "https://stripe.wd5.myworkdayjobs.com/en-US/Stripe_Careers/job/SF/Eng_1"))
	require.True(t, ok)
	assert.Equal(t, "https://stripe.wd5.myworkdayjobs.com/Stripe_Careers", slug)

	_, ok = a.ParseCareersURL(mustParseURL(t, "https://jobs.lever.co/acme"))
	assert.False(t, ok)
}

func TestWorkdayURLSlugSearchAndDetail(t *testing.T) {
	a, _ := testWorkdayAdapter(t)
	urlSlug := "https://stripe.wd5.myworkdayjobs.com/Stripe_Careers"

	res, err := a.Search(t.Context(), urlSlug, SearchParams{})
	require.NoError(t, err)
	assert.Equal(t, 27, res.TotalCount)
	require.NotEmpty(t, res.Jobs)

	d, err := a.Detail(t.Context(), urlSlug, res.Jobs[0].JobID)
	require.NoError(t, err)
	assert.Equal(t, "stripe", d.Company, "URL-resolved company name should be the tenant")
	assert.NotEmpty(t, d.Description)
}

func TestWorkdayUnknownSlugTeaches(t *testing.T) {
	a := NewWorkdayAdapter(http.DefaultClient)
	_, err := a.Search(t.Context(), "not-a-tenant", SearchParams{})
	require.ErrorContains(t, err, "careers URL", "error should teach the URL alternative")
}

// TestFlattenFacetsNullFacetParameter guards a present-but-explicitly-null
// facetParameter on a leaf's group ancestor: it must not overwrite the
// param inherited from a real parent, and a leaf with a null descriptor
// must not produce a garbage empty-label entry.
func TestFlattenFacetsNullFacetParameter(t *testing.T) {
	nodes := []workday.FacetNode{
		{
			FacetParameter: workday.NewOptNilString("jobFamilyGroup"),
			Values: []workday.FacetNode{
				{
					// A group node with an explicitly null facetParameter
					// must not blank out "jobFamilyGroup" for its children.
					FacetParameter: workday.OptNilString{Set: true, Null: true},
					Values: []workday.FacetNode{
						{
							Descriptor: workday.NewOptNilString("Engineering"),
							ID:         workday.NewOptString("guid-1"),
						},
						{
							// Null descriptor: must be dropped, not emitted
							// with an empty label.
							Descriptor: workday.OptNilString{Set: true, Null: true},
							ID:         workday.NewOptString("guid-2"),
						},
					},
				},
			},
		},
	}
	flat := flattenFacets(nodes)
	require.Len(t, flat, 1)
	assert.Equal(t, "jobFamilyGroup", flat[0].param)
	assert.Equal(t, "Engineering", flat[0].label)
	assert.Equal(t, "guid-1", flat[0].id)
}
