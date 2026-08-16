package mokahr

import (
	_ "embed"
	"fmt"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
)

//go:embed companies.yaml
var _companiesYAML []byte

// Company is a confirmed organization hosting a public MokaHR careers site
// (internal/provider/mokahr/companies.yaml). Every entry was verified against
// the live job API — a successful envelope with at least one open posting —
// and its careers-page title checked against the expected company;
// cmd/verify-companies re-verifies the roster.
//
// A tenant can own several careers sites with different job sets, and the one
// a bare /social-recruitment/<org> URL redirects to is not always the largest,
// so an entry names the site as well as the tenant.
type Company struct {
	Name string `yaml:"company" json:"company"`
	Org  string `yaml:"org"     json:"org"`
	Site string `yaml:"site"    json:"site"`
}

// Slug is the company's key across this package and the ATS registry. The
// site id is part of it because a tenant alone does not identify a job set.
func (c Company) Slug() string { return c.Org + "/" + c.Site }

// CareersURL returns the company's human-facing job-listing page.
func (c Company) CareersURL() string { return CareersURL(c.Org, c.Site) }

// Companies holds every confirmed MokaHR careers site, sorted by company name.
var Companies = mustLoadCompanies()

// CompaniesBySlug looks up a confirmed company by "<org>/<site>" slug. Keys
// are lowercased, so callers must lowercase their input before indexing.
var CompaniesBySlug = buildSlugIndex(Companies)

// mustLoadCompanies parses the embedded companies.yaml. A parse failure is a
// build-time bug in a file this package owns, not a runtime condition to
// recover from.
func mustLoadCompanies() []Company {
	var cs []Company
	if err := yaml.Unmarshal(_companiesYAML, &cs); err != nil {
		panic(fmt.Sprintf("mokahr: parse companies.yaml: %v", err))
	}
	for _, c := range cs {
		if c.Name == "" || c.Org == "" || c.Site == "" {
			panic(fmt.Sprintf("mokahr: companies.yaml entry %+v is missing company, org, or site", c))
		}
	}
	slices.SortFunc(cs, func(a, b Company) int { return strings.Compare(a.Name, b.Name) })
	return cs
}

func buildSlugIndex(cs []Company) map[string]Company {
	m := make(map[string]Company, len(cs))
	for _, c := range cs {
		m[strings.ToLower(c.Slug())] = c
	}
	return m
}
