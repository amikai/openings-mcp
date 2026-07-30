package oracle

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	oracleprovider "github.com/amikai/openings-mcp/internal/provider/oracle"
)

func TestParseFilters(t *testing.T) {
	tests := []struct {
		name    string
		values  []string
		want    map[oracleprovider.Facet][]string
		wantErr string
	}{
		{
			name:   "repeat same facet",
			values: []string{"location=1", "location=2", "workplace-type=ORA_REMOTE"},
			want: map[oracleprovider.Facet][]string{
				oracleprovider.FacetLocation:      {"1", "2"},
				oracleprovider.FacetWorkplaceType: {"ORA_REMOTE"},
			},
		},
		{name: "trim whitespace", values: []string{" category = 42 "}, want: map[oracleprovider.Facet][]string{oracleprovider.FacetCategory: {"42"}}},
		{name: "missing equals", values: []string{"location"}, wantErr: "must be name=id"},
		{name: "empty value", values: []string{"location="}, wantErr: "must be name=id"},
		{name: "unknown facet", values: []string{"salary=high"}, wantErr: "unknown facet"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFilters(tt.values)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRunCompanies(t *testing.T) {
	var out bytes.Buffer

	err := runCompanies("json", commandEnv{out: &out})
	require.NoError(t, err)

	var companies []oracleprovider.Company
	require.NoError(t, json.Unmarshal(out.Bytes(), &companies))
	assert.Equal(t, oracleprovider.Companies, companies)
}

func TestResolveCompanyByName(t *testing.T) {
	require.NotEmpty(t, oracleprovider.Companies)
	company := oracleprovider.Companies[0]

	site, name, err := resolveCompany(t.Context(), strings.ToLower(company.Name), nil)
	require.NoError(t, err)

	assert.Equal(t, company.Name, name)
	assert.Equal(t, company.Site, site.Site)
	assert.Equal(t, company.SiteNumber, site.SiteNumber)
	assert.Equal(t, company.CareersURL(), site.CareersURL)
}

func TestResolveCompanyByCareersURL(t *testing.T) {
	server := oracleprovider.NewMockServer()
	defer server.Close()

	site, name, err := resolveCompany(t.Context(), careersURL(server.URL), server.Client())
	require.NoError(t, err)

	assert.Equal(t, "Mayo-US", name)
	assert.Equal(t, "Mayo-US", site.Site)
	assert.Equal(t, "CX_1", site.SiteNumber)
	assert.Equal(t, server.URL, site.APIBaseURL)
}

func TestRunSearchEndToEnd(t *testing.T) {
	server := oracleprovider.NewMockServer()
	defer server.Close()
	var out bytes.Buffer

	err := runSearch(t.Context(), searchFlags{
		commonFlags: commonFlags{
			company: careersURL(server.URL),
			timeout: time.Second,
			format:  "json",
		},
		limit: 3,
	}, commandEnv{httpClient: server.Client(), out: &out})
	require.NoError(t, err)

	var result oracleprovider.SearchResult
	require.NoError(t, json.Unmarshal(out.Bytes(), &result))
	assert.Equal(t, 1330, result.Total)
	require.Len(t, result.Jobs, 3)
	assert.Equal(t, "361564", result.Jobs[0].ID)
	assert.Equal(t, "Cardiologist - Echo / Imaging", result.Jobs[0].Title)
}

func TestRunFacetsEndToEnd(t *testing.T) {
	server := oracleprovider.NewMockServer()
	defer server.Close()
	var out bytes.Buffer

	err := runFacets(t.Context(), facetsFlags{
		commonFlags: commonFlags{
			company: careersURL(server.URL),
			timeout: time.Second,
			format:  "json",
		},
	}, commandEnv{httpClient: server.Client(), out: &out})
	require.NoError(t, err)

	var facets map[oracleprovider.Facet][]oracleprovider.FacetOption
	require.NoError(t, json.Unmarshal(out.Bytes(), &facets))
	assert.Equal(t, "7", facets[oracleprovider.FacetPostingDate][0].ID)
	assert.Equal(t, "300000034547579", facets[oracleprovider.FacetWorkLocation][0].ID)
	assert.Equal(t, "1", facets[oracleprovider.FacetOrganization][0].ID)
}

func TestRunDetailEndToEnd(t *testing.T) {
	server := oracleprovider.NewMockServer()
	defer server.Close()
	var out bytes.Buffer

	err := runDetail(t.Context(), detailFlags{
		commonFlags: commonFlags{
			company: careersURL(server.URL),
			timeout: time.Second,
			format:  "text",
		},
		jobID: "386920",
	}, commandEnv{httpClient: server.Client(), out: &out})
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Senior Analyst - ATS")
	assert.Contains(t, out.String(), "Access Technologies and Systems")
	assert.Contains(t, out.String(), "/job/386920")
}

func TestRunDetailNotFound(t *testing.T) {
	server := oracleprovider.NewMockServer()
	defer server.Close()
	var out bytes.Buffer

	err := runDetail(t.Context(), detailFlags{
		commonFlags: commonFlags{
			company: careersURL(server.URL),
			timeout: time.Second,
			format:  "json",
		},
		jobID: "999999999999",
	}, commandEnv{httpClient: server.Client(), out: &out})
	require.Error(t, err)
	assert.True(t, errors.Is(err, oracleprovider.ErrJobNotFound))
}

func TestCommandValidation(t *testing.T) {
	env := commandEnv{out: &bytes.Buffer{}}

	err := runSearch(t.Context(), searchFlags{
		commonFlags: commonFlags{timeout: time.Second},
		limit:       20,
	}, env)
	assert.ErrorContains(t, err, "--company is required")

	err = runDetail(t.Context(), detailFlags{
		commonFlags: commonFlags{company: "Mayo Clinic", timeout: time.Second},
	}, env)
	assert.ErrorContains(t, err, "--id is required")

	err = runSearch(t.Context(), searchFlags{
		commonFlags: commonFlags{company: "Mayo Clinic", timeout: 0},
		limit:       20,
	}, env)
	assert.ErrorContains(t, err, "--timeout must be greater than zero")

	err = runSearch(t.Context(), searchFlags{
		commonFlags: commonFlags{company: "Mayo Clinic", timeout: time.Second},
		limit:       101,
	}, env)
	assert.ErrorContains(t, err, "--limit must be between 1 and 100")
}

func careersURL(baseURL string) string {
	return baseURL + "/hcmUI/CandidateExperience/en/sites/Mayo-US/jobs"
}
