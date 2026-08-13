package dayforce

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearch(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewBoardClient(srv.URL, srv.Client())
	require.NoError(t, err)

	res, err := client.Search(t.Context(), SearchRequest{
		ClientNamespace: "pca",
		JobBoardCode:    "CANDIDATEPORTAL",
		CultureCode:     "en-US",
	})
	require.NoError(t, err)

	assert.Equal(t, 352, res.MaxCount)
	assert.Equal(t, 0, res.Offset)
	assert.Equal(t, 25, res.Count)
	require.Len(t, res.JobPostings, 25)

	first := res.JobPostings[0]
	assert.Equal(t, 62333, first.JobPostingId)
	assert.Equal(t, "pca", first.ClientNamespace.Or(""))
	assert.Equal(t, 1, first.JobBoardId.Or(0))
	assert.Equal(t, "General Laborer- Wallula", first.JobTitle)
	assert.False(t, first.HasVirtualLocation)
	require.NotEmpty(t, first.PostingLocations)
	assert.Equal(t, "lat:46.0960439;lng:-118.911071", first.PostingLocations[0].Coordinates)
}

// TestSearchNullLocation proves a posting location with isoCountryCode,
// stateCode, and cityName all null at once (a continent-level location,
// e.g. "Europe") decodes without error — the crash this fixture was
// captured to reproduce.
func TestSearchNullLocation(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewBoardClient(srv.URL, srv.Client())
	require.NoError(t, err)

	res, err := client.Search(t.Context(), SearchRequest{
		ClientNamespace: "mymilacron",
		JobBoardCode:    "CANDIDATEPORTAL",
		CultureCode:     "en-US",
	})
	require.NoError(t, err)

	var found *SearchPosting
	for i := range res.JobPostings {
		if res.JobPostings[i].JobPostingId == MockNullLocationJobID {
			found = &res.JobPostings[i]
			break
		}
	}
	require.NotNil(t, found, "expected job %d in the fixture", MockNullLocationJobID)
	require.Len(t, found.PostingLocations, 1)

	loc := found.PostingLocations[0]
	assert.Equal(t, "Europe", loc.FormattedAddress)
	assert.True(t, loc.IsoCountryCode.IsNull())
	assert.True(t, loc.StateCode.IsNull())
	assert.True(t, loc.CityName.IsNull())
}

// TestSearchNullPostingLocations proves a search row whose postingLocations
// array itself is null (not just a field within a location) decodes as a
// nil slice rather than erroring — the gap the earlier fix (widening only
// isoCountryCode) missed.
func TestSearchNullPostingLocations(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewBoardClient(srv.URL, srv.Client())
	require.NoError(t, err)

	res, err := client.Search(t.Context(), SearchRequest{
		ClientNamespace: "mymilacron",
		JobBoardCode:    "CANDIDATEPORTAL",
		CultureCode:     "en-US",
		PaginationStart: NewOptInt(25),
	})
	require.NoError(t, err)

	var found *SearchPosting
	for i := range res.JobPostings {
		if res.JobPostings[i].JobPostingId == MockNullPostingLocationsJobID {
			found = &res.JobPostings[i]
			break
		}
	}
	require.NotNil(t, found, "expected job %d in the fixture", MockNullPostingLocationsJobID)
	assert.Nil(t, found.PostingLocations)
	assert.True(t, found.HasVirtualLocation)
}

// TestSearchFiltered proves searchText is a real server-side filter rather
// than an ignored parameter: the fixture's maxCount narrows from 352 to 115.
func TestSearchFiltered(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewBoardClient(srv.URL, srv.Client())
	require.NoError(t, err)

	res, err := client.Search(t.Context(), SearchRequest{
		ClientNamespace: "pca",
		JobBoardCode:    "CANDIDATEPORTAL",
		CultureCode:     "en-US",
		SearchText:      NewOptString("electrical engineer"),
	})
	require.NoError(t, err)

	assert.Equal(t, 115, res.MaxCount)
}

// TestSearchPage2 proves paginationStart arithmetic: page 2 comes back with
// offset 25 and a job-id set disjoint from page 1, so a regression that
// ignores the parameter (repeating page 1) would fail this test.
func TestSearchPage2(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewBoardClient(srv.URL, srv.Client())
	require.NoError(t, err)

	page1, err := client.Search(t.Context(), SearchRequest{
		ClientNamespace: "pca",
		JobBoardCode:    "CANDIDATEPORTAL",
		CultureCode:     "en-US",
	})
	require.NoError(t, err)

	page2, err := client.Search(t.Context(), SearchRequest{
		ClientNamespace: "pca",
		JobBoardCode:    "CANDIDATEPORTAL",
		CultureCode:     "en-US",
		PaginationStart: NewOptInt(25),
	})
	require.NoError(t, err)

	assert.Equal(t, 352, page2.MaxCount)
	assert.Equal(t, 25, page2.Offset)
	require.Len(t, page2.JobPostings, 25)

	ids1 := make(map[int]bool, len(page1.JobPostings))
	for _, p := range page1.JobPostings {
		ids1[p.JobPostingId] = true
	}
	for _, p := range page2.JobPostings {
		assert.False(t, ids1[p.JobPostingId], "job %d appears on both page 1 and page 2", p.JobPostingId)
	}
}

// TestSearchNonDefaultBoard exercises a tenant whose board is not
// CANDIDATEPORTAL and whose jobBoardId is not 1, so the non-default-board
// path is proven rather than assumed.
func TestSearchNonDefaultBoard(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewBoardClient(srv.URL, srv.Client())
	require.NoError(t, err)

	res, err := client.Search(t.Context(), SearchRequest{
		ClientNamespace: "mydayforce",
		JobBoardCode:    "alljobs",
		CultureCode:     "en-US",
	})
	require.NoError(t, err)

	assert.Equal(t, 139, res.MaxCount)
	require.NotEmpty(t, res.JobPostings)
	for _, p := range res.JobPostings {
		assert.Equal(t, 8, p.JobBoardId.Or(0))
	}
}

// TestSearchUnknownTenant asserts the typed not-found error, not a generic
// decode failure, for a clientNamespace with no matching board.
func TestSearchUnknownTenant(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewBoardClient(srv.URL, srv.Client())
	require.NoError(t, err)

	_, err = client.Search(t.Context(), SearchRequest{
		ClientNamespace: "nosuchtenantxyz",
		JobBoardCode:    "CANDIDATEPORTAL",
		CultureCode:     "en-US",
	})
	require.ErrorIs(t, err, ErrBoardNotFound)
}

// TestSearchWithoutCSRFIsForbidden proves the mock enforces the same
// cookie+header contract as the live API at the transport level: a search
// with neither a csrf cookie nor a header decodes as the typed 403, not a
// generic decode failure or a silently-accepted request. Bypasses
// BoardClient (which always fetches a token first) to reach the generated
// Client directly, so the request genuinely carries neither.
func TestSearchWithoutCSRFIsForbidden(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL, WithClient(srv.Client()))
	require.NoError(t, err)

	res, err := client.SearchJobPostings(t.Context(), &SearchRequest{
		ClientNamespace: "pca",
		JobBoardCode:    "CANDIDATEPORTAL",
		CultureCode:     "en-US",
	}, SearchJobPostingsParams{ClientNamespace: "pca"})
	require.NoError(t, err)

	_, ok := res.(*SearchJobPostingsForbidden)
	assert.True(t, ok, "want *SearchJobPostingsForbidden, got %T", res)
}

// TestSearchWithHeaderButNoCookieIsForbidden proves both legs of the CSRF
// gate are independently required: a correct X-Csrf-Token header alone,
// without the cookie, still decodes as the typed 403. Uses the bare
// generated Client (no jar), so no cookie is ever sent, isolating the
// missing leg to the cookie.
func TestSearchWithHeaderButNoCookieIsForbidden(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL, WithClient(srv.Client()))
	require.NoError(t, err)

	res, err := client.SearchJobPostings(t.Context(), &SearchRequest{
		ClientNamespace: "pca",
		JobBoardCode:    "CANDIDATEPORTAL",
		CultureCode:     "en-US",
	}, SearchJobPostingsParams{ClientNamespace: "pca", XCSRFTOKEN: mockCSRFToken})
	require.NoError(t, err)

	_, ok := res.(*SearchJobPostingsForbidden)
	assert.True(t, ok, "want *SearchJobPostingsForbidden, got %T", res)
}

// TestSearchRefreshesTokenOnForbidden proves BoardClient retries once with
// a refreshed token when its cached token no longer matches.
func TestSearchRefreshesTokenOnForbidden(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewBoardClient(srv.URL, srv.Client())
	require.NoError(t, err)

	req := SearchRequest{ClientNamespace: "pca", JobBoardCode: "CANDIDATEPORTAL", CultureCode: "en-US"}

	_, err = client.Search(t.Context(), req)
	require.NoError(t, err)

	client.token = "stale-token-that-does-not-match-the-cookie"
	_, err = client.Search(t.Context(), req)
	require.NoError(t, err, "the stale-token search should recover with one retry")
}

func TestSearchPersistentForbidden(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	baseClient := srv.Client()
	baseTransport := baseClient.Transport
	baseClient.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/jobposting/search") {
			return &http.Response{
				StatusCode: http.StatusForbidden,
				Body:       io.NopCloser(strings.NewReader("Forbidden")),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}
		return baseTransport.RoundTrip(req)
	})
	client, err := NewBoardClient(srv.URL, baseClient)
	require.NoError(t, err)

	_, err = client.Search(t.Context(), SearchRequest{
		ClientNamespace: "pca",
		JobBoardCode:    "CANDIDATEPORTAL",
		CultureCode:     "en-US",
	})
	require.ErrorIs(t, err, ErrCSRFRequired)
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestJob(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewBoardClient(srv.URL, srv.Client())
	require.NoError(t, err)

	res, err := client.Job(t.Context(), "pca", "en-US", 1, MockJobID)
	require.NoError(t, err)

	assert.Equal(t, MockJobID, res.JobPostingId)
	assert.Equal(t, "Electrical Engineering Co Op/Intern - All Mills", res.JobTitle)
	require.Len(t, res.JobPostingAttributes, 3)

	payType := res.JobPostingAttributes[0]
	assert.Equal(t, "PayType", payType.Name)
	s, ok := payType.Value.GetString()
	require.True(t, ok, "want PayType to decode as string")
	assert.Equal(t, "Salary", s)

	minRate := res.JobPostingAttributes[1]
	assert.Equal(t, "HiringMinRate", minRate.Name)
	f, ok := minRate.Value.GetFloat64()
	require.True(t, ok, "want HiringMinRate to decode as a number")
	assert.Equal(t, float64(25), f)

	require.NotEmpty(t, res.PostingLocations)
	assert.Equal(t, 40.7967244, res.PostingLocations[0].Coordinates.Lat)
}

// TestJobNullCurrencyAndEmptyLocations proves a detail response with
// isoCurrencyRegion: null and a zero-length (not null) postingLocations
// decodes without error — jdemea's real shape for a posting with no location
// at all and hasVirtualLocation: false.
func TestJobNullCurrencyAndEmptyLocations(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewBoardClient(srv.URL, srv.Client())
	require.NoError(t, err)

	res, err := client.Job(t.Context(), "jdemea", "en-US", 1, MockNullCurrencyJobID)
	require.NoError(t, err)

	assert.Equal(t, MockNullCurrencyJobID, res.JobPostingId)
	assert.True(t, res.IsoCurrencyRegion.IsNull())
	assert.False(t, res.HasVirtualLocation)
	assert.Empty(t, res.PostingLocations)
	assert.NotNil(t, res.PostingLocations, "detail postingLocations is documented as never null, only ever empty")
}

// TestJobNullDescriptionHeader proves a detail response with
// jobDescriptionHeader: null decodes without error — ara posting 8's real
// shape when only jobDescription is present.
func TestJobNullDescriptionHeader(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewBoardClient(srv.URL, srv.Client())
	require.NoError(t, err)

	res, err := client.Job(t.Context(), "ara", "en-US", 1, MockNullDescriptionHeaderJobID)
	require.NoError(t, err)

	assert.Equal(t, MockNullDescriptionHeaderJobID, res.JobPostingId)
	assert.True(t, res.JobPostingContent.JobDescriptionHeader.IsNull())
	assert.False(t, res.JobPostingContent.JobDescription.IsNull())
}

// TestJobBoolAttribute proves a jobPostingAttributes entry with a boolean
// value (TravelRequired, type "bool") decodes without error — a shape
// JobPostingAttributeValue (originally string|number only) missed, found
// live during the page-walk for this fix rather than in the initial null
// sweep.
func TestJobBoolAttribute(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewBoardClient(srv.URL, srv.Client())
	require.NoError(t, err)

	res, err := client.Job(t.Context(), "mymilacron", "en-US", 1, MockBoolAttributeJobID)
	require.NoError(t, err)

	var travelRequired *JobPostingAttribute
	for i := range res.JobPostingAttributes {
		if res.JobPostingAttributes[i].Name == "TravelRequired" {
			travelRequired = &res.JobPostingAttributes[i]
			break
		}
	}
	require.NotNil(t, travelRequired, "expected a TravelRequired attribute in the fixture")
	b, ok := travelRequired.Value.GetBool()
	require.True(t, ok, "want TravelRequired to decode as bool")
	assert.True(t, b)
}

func TestJobNotFound(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewBoardClient(srv.URL, srv.Client())
	require.NoError(t, err)

	_, err = client.Job(t.Context(), "pca", "en-US", 1, MockNotFoundJobID)
	require.ErrorIs(t, err, ErrJobNotFound)
}

// TestJobCrossTenantNotFound proves the mock (and hence the client's
// ErrJobNotFound path) honors ns/jobBoardId, not just postingId: a valid
// posting id requested under a different tenant's namespace/board 404s
// rather than returning "pca"'s fixture.
func TestJobCrossTenantNotFound(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewBoardClient(srv.URL, srv.Client())
	require.NoError(t, err)

	_, err = client.Job(t.Context(), "mydayforce", "en-US", 8, MockJobID)
	require.ErrorIs(t, err, ErrJobNotFound)
}

func TestPostingAttributes(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewBoardClient(srv.URL, srv.Client())
	require.NoError(t, err)

	departments, err := client.Departments(t.Context(), "pca", 1, "en-US")
	require.NoError(t, err)
	require.Len(t, departments.PostingAttributesAttributesList, 58)
	assert.Equal(t, 290, departments.PostingAttributesAttributesList[0].AttributeId)
	assert.Equal(t, "4202-TRANSPORTATION", departments.PostingAttributesAttributesList[0].AttributeValue)

	payClasses, err := client.PayClasses(t.Context(), "pca", 1, "en-US")
	require.NoError(t, err)
	assert.Equal(t, []PostingAttribute{{AttributeId: 1, AttributeValue: "Full-time"}}, payClasses.PostingAttributesAttributesList)

	payTypes, err := client.PayTypes(t.Context(), "pca", 1, "en-US")
	require.NoError(t, err)
	assert.Equal(t, []PostingAttribute{
		{AttributeId: 1, AttributeValue: "Hourly"},
		{AttributeId: 2, AttributeValue: "Salary"},
	}, payTypes.PostingAttributesAttributesList)
}

func TestSiteInfo(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewBoardClient(srv.URL, srv.Client())
	require.NoError(t, err)

	info, err := client.SiteInfo(t.Context(), "pca", "CANDIDATEPORTAL", "en-US")
	require.NoError(t, err)

	assert.Equal(t, 1, info.JobBoardId)
	assert.Equal(t, "pca", info.ClientNamespace)
	assert.Equal(t, "candidateportal", info.JobBoardCode)
	assert.Equal(t, "en-US", info.CultureCode)
}

// TestSiteInfoMissingJobBoardID proves parseSiteInfo rejects a site-info
// query whose jobBoardId is absent or zero, rather than returning a board
// whose detail and posting-attribute calls would all address jobBoardId 0.
func TestSiteInfoMissingJobBoardID(t *testing.T) {
	html := `<html><body><script id="__NEXT_DATA__" type="application/json">` +
		`{"props":{"pageProps":{"dehydratedState":{"queries":[{"queryKey":["site-info"],"state":{"data":{"jobBoardId":0}}}]}}}}` +
		`</script></body></html>`
	_, err := parseSiteInfo([]byte(html))
	require.Error(t, err)
	assert.ErrorContains(t, err, "missing jobBoardId")
}

func TestSiteInfoAllowsNullClientID(t *testing.T) {
	html := `<html><body><script id="__NEXT_DATA__" type="application/json">` +
		`{"props":{"pageProps":{"dehydratedState":{"queries":[{"queryKey":["site-info"],"state":{"data":{"jobBoardId":1,"clientId":null}}}]}}}}` +
		`</script></body></html>`
	info, err := parseSiteInfo([]byte(html))
	require.NoError(t, err)
	assert.Equal(t, 1, info.JobBoardId)
}

func TestSiteInfoMissingNextData(t *testing.T) {
	_, err := parseSiteInfo([]byte("<html><body>not a dayforce page</body></html>"))
	require.Error(t, err)
}

// TestSiteInfoMissingSiteInfoQuery proves parseSiteInfo fails rather than
// zero-valuing when __NEXT_DATA__ is present but carries no site-info query
// (e.g. a differently-shaped page).
func TestSiteInfoMissingSiteInfoQuery(t *testing.T) {
	html := `<html><body><script id="__NEXT_DATA__" type="application/json">` +
		`{"props":{"pageProps":{"dehydratedState":{"queries":[{"queryKey":["something-else"],"state":{"data":{}}}]}}}}` +
		`</script></body></html>`
	_, err := parseSiteInfo([]byte(html))
	require.Error(t, err)
}

// TestSiteInfoNonOKStatus proves SiteInfo itself surfaces an error on a
// non-200 page fetch, not just parseSiteInfo on a bad body: an xref with an
// extra path segment doesn't match the mock's site-info route, so the
// default mux 404 exercises siteinfo.go's status check.
func TestSiteInfoNonOKStatus(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewBoardClient(srv.URL, srv.Client())
	require.NoError(t, err)

	_, err = client.SiteInfo(t.Context(), "pca", "no/such/page", "en-US")
	require.Error(t, err)
}
