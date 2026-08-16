package oracle

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSiteDocument(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(mockCareersPageResponse)))
	require.NoError(t, err)
	pageURL, err := url.Parse("https://fa-euwp-saasfaprod1.fa.ocs.oraclecloud.com/hcmUI/CandidateExperience/en/sites/Mayo-US/jobs")
	require.NoError(t, err)

	got, err := parseSiteDocument(doc, pageURL)
	require.NoError(t, err)
	assert.Equal(t, Site{
		CareersURL: "https://fa-euwp-saasfaprod1.fa.ocs.oraclecloud.com/hcmUI/CandidateExperience/en/sites/Mayo-US/jobs",
		APIBaseURL: "https://fa-euwp-saasfaprod1.fa.ocs.oraclecloud.com:443",
		Site:       "Mayo-US",
		SiteNumber: "CX_1",
		Language:   "en",
	}, got)
	assert.Equal(
		t,
		"https://fa-euwp-saasfaprod1.fa.ocs.oraclecloud.com/hcmUI/CandidateExperience/en/sites/Mayo-US/job/386920",
		got.JobURL("386920"),
	)
}

func TestParseSiteDocumentLegacyTheme(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(
		`<html><head><base href="/hcmUI/CandidateExperience/en/sites/CX_1001"/></head></html>`,
	))
	require.NoError(t, err)
	pageURL, err := url.Parse("https://elzw.fa.em8.oraclecloud.com/hcmUI/CandidateExperience/en/sites/CX_1001/jobs")
	require.NoError(t, err)

	got, err := parseSiteDocument(doc, pageURL)
	require.NoError(t, err)
	assert.Equal(t, Site{
		CareersURL: "https://elzw.fa.em8.oraclecloud.com/hcmUI/CandidateExperience/en/sites/CX_1001/jobs",
		APIBaseURL: "https://elzw.fa.em8.oraclecloud.com",
		Site:       "CX_1001",
		SiteNumber: "CX_1001",
		Language:   "en",
	}, got)
}

func TestParseSiteDocumentRejectsUnrecognizedPage(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(
		`<html><head><base href="/careers"/></head></html>`,
	))
	require.NoError(t, err)
	pageURL, err := url.Parse("https://example.com/careers")
	require.NoError(t, err)

	_, err = parseSiteDocument(doc, pageURL)
	assert.ErrorContains(t, err, "unrecognized candidate experience path")
}

func TestSearchFinder(t *testing.T) {
	client := &SiteClient{site: Site{SiteNumber: "CX_1"}}
	got, err := client.searchFinder(SearchRequest{
		Keyword: `analyst "data"`,
		Limit:   25,
		Offset:  50,
		Facets:  []Facet{FacetTitle, FacetLocation, FacetTitle},
		Filters: map[Facet][]string{
			FacetLocation:      {"300000006426003"},
			FacetWorkplaceType: {"ORA_ON_SITE", "ORA_REMOTE"},
		},
	})
	require.NoError(t, err)
	assert.Equal(
		t,
		`findReqs;siteNumber=CX_1,facetsList=TITLES;LOCATIONS,limit=25,offset=50,sortBy=POSTING_DATES_DESC,keyword="analyst \"data\"",selectedLocationsFacet=300000006426003,selectedWorkplaceTypesFacet=ORA_ON_SITE;ORA_REMOTE`,
		got,
	)
}

func TestSearchFinderRejectsSyntaxInFacetValue(t *testing.T) {
	client := &SiteClient{site: Site{SiteNumber: "CX_1"}}
	_, err := client.searchFinder(SearchRequest{
		Limit:   20,
		Filters: map[Facet][]string{FacetLocation: {"1,siteNumber=CX_2"}},
	})
	assert.ErrorContains(t, err, "contains finder syntax")
}

func TestDiscoverClientEndToEnd(t *testing.T) {
	server := NewMockServer()
	defer server.Close()

	client, err := DiscoverClient(
		t.Context(),
		server.URL+"/hcmUI/CandidateExperience/en/sites/Mayo-US/jobs",
		server.Client(),
	)
	require.NoError(t, err)
	assert.Equal(t, "CX_1", client.Site().SiteNumber)
	assert.Equal(t, server.URL, client.Site().APIBaseURL)

	search, err := client.Search(t.Context(), SearchRequest{Limit: 3})
	require.NoError(t, err)
	assert.Equal(t, 1330, search.Total)
	require.Len(t, search.Jobs, 3)
	assert.Equal(t, "361564", search.Jobs[0].ID)
	assert.Equal(t, "Cardiologist - Echo / Imaging", search.Jobs[0].Title)

	facets, err := client.Search(t.Context(), SearchRequest{Limit: 1, Facets: AllFacets()})
	require.NoError(t, err)
	require.NotEmpty(t, facets.Facets[FacetPostingDate])
	assert.Equal(t, "7", facets.Facets[FacetPostingDate][0].ID)
	assert.Equal(t, "300000034547579", facets.Facets[FacetWorkLocation][0].ID)
	assert.Equal(t, "1", facets.Facets[FacetOrganization][0].ID)

	detail, err := client.Detail(t.Context(), "386920")
	require.NoError(t, err)
	assert.Equal(t, "Senior Analyst - ATS", detail.Title)
	assert.Contains(t, detail.DescriptionHTML, "Access Technologies and Systems")
	assert.Equal(
		t,
		server.URL+"/hcmUI/CandidateExperience/en/sites/Mayo-US/job/386920",
		detail.URL,
	)

	_, err = client.Detail(t.Context(), "999999999999")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrJobNotFound))
}

func TestSiteClientRequestsJSONMediaType(t *testing.T) {
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		assert.Equal(t, "application/json", request.Header.Get("Accept"))
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(
				`{"items":[{"TotalJobsCount":0,"Limit":1,"Offset":0,"requisitionList":[],"titlesFacet":[],"locationsFacet":[],"categoriesFacet":[],"postingDatesFacet":[],"workLocationsFacet":[],"organizationsFacet":[],"workplaceTypesFacet":[]}]}`,
			)),
		}, nil
	})
	client, err := NewSiteClient(
		Site{APIBaseURL: "https://example.com", SiteNumber: "CX_1"},
		&http.Client{Transport: transport},
	)
	require.NoError(t, err)

	_, err = client.Search(t.Context(), SearchRequest{Limit: 1})
	require.NoError(t, err)
}

func TestSearchValidation(t *testing.T) {
	client := &SiteClient{}
	for _, limit := range []int{-1, 0, 101} {
		_, err := client.Search(t.Context(), SearchRequest{Limit: limit})
		assert.ErrorContainsf(t, err, "limit must be between 1 and 100", "limit=%d", limit)
	}
	_, err := client.Search(t.Context(), SearchRequest{Limit: 20, Offset: -1})
	assert.ErrorContains(t, err, "offset must be >= 0")
}

func TestParseFacet(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Facet
		wantErr string
	}{
		{name: "location", input: "location", want: FacetLocation},
		{name: "case and whitespace", input: " WORKPLACE-TYPE ", want: FacetWorkplaceType},
		{name: "unknown", input: "salary", wantErr: "unknown facet"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFacet(tt.input)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
