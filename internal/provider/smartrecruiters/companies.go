package smartrecruiters

import (
	_ "embed"
	"fmt"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
)

//go:embed companies.yaml
var companiesYAML []byte

// RosterCompany is a confirmed organization hosting a public SmartRecruiters
// career site, drawn from a curated list
// (internal/provider/smartrecruiters/companies.yaml). Every entry was
// discovered live off the public cross-company feed
// (jobs.smartrecruiters.com/sr-jobs/search) and verified against the
// per-company Posting API — HTTP 200 with totalFound > 0 and a matching
// company name. Named RosterCompany, not Company, because the generated
// Company schema type already names the postings' {identifier, name}
// object.
// CompanyIdentifier is the identifier the API takes as its
// companyIdentifier path parameter, e.g. "Equinox" for
// api.smartrecruiters.com/v1/companies/Equinox/postings.
type RosterCompany struct {
	Name              string `yaml:"company" json:"company"`
	CompanyIdentifier string `yaml:"company_identifier" json:"company_identifier"`
}

// CareersURL returns the company's human-facing career site, e.g.
// https://jobs.smartrecruiters.com/Equinox. API calls instead pass
// CompanyIdentifier directly as the companyIdentifier parameter.
func (c RosterCompany) CareersURL() string {
	return fmt.Sprintf("https://jobs.smartrecruiters.com/%s", c.CompanyIdentifier)
}

// Companies holds every confirmed SmartRecruiters company, sorted by
// company name.
var Companies = mustLoadCompanies()

// CompaniesByIdentifier looks up a confirmed company by companyIdentifier.
// Keys are lowercased, so callers must lowercase their input before
// indexing.
var CompaniesByIdentifier = buildIdentifierIndex(Companies)

// mustLoadCompanies parses the embedded companies.yaml. A parse failure is
// a build-time bug in a file this package owns, not a runtime condition to
// recover from.
func mustLoadCompanies() []RosterCompany {
	var cs []RosterCompany
	if err := yaml.Unmarshal(companiesYAML, &cs); err != nil {
		panic(fmt.Sprintf("smartrecruiters: parse companies.yaml: %v", err))
	}
	slices.SortFunc(cs, func(a, b RosterCompany) int { return strings.Compare(a.Name, b.Name) })
	return cs
}

// buildIdentifierIndex keeps the first row on a duplicate companyIdentifier,
// so a data bug in companies.yaml degrades to "one entry ignored" rather
// than an unpredictable pick between rows.
func buildIdentifierIndex(cs []RosterCompany) map[string]RosterCompany {
	m := make(map[string]RosterCompany, len(cs))
	for _, c := range cs {
		slug := strings.ToLower(c.CompanyIdentifier)
		if _, ok := m[slug]; ok {
			continue
		}
		m[slug] = c
	}
	return m
}
