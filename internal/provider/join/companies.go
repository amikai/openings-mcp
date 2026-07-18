package join

import (
	_ "embed"
	"fmt"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
)

//go:embed companies.yaml
var companiesYAML []byte

// RosterCompany is a confirmed organization hosting a public join.com
// career page, drawn from a curated list
// (internal/provider/join/companies.yaml). Every entry was verified
// live: the GraphQL search endpoint returns HTTP 200 with at least one
// job for Slug/CompanyID, or — for a company that happens to have zero
// open jobs right now — the /companies/{slug} page resolves to the same
// CompanyID independently (see cmd/verify-companies).
// CompanyID is the numeric id the GraphQL API takes as
// PublicJobsQueryInput.companyId; there is no way to derive it from Slug
// at request time (see API.md), so it's resolved once here.
type RosterCompany struct {
	Name      string `yaml:"company" json:"company"`
	Slug      string `yaml:"slug" json:"slug"`
	CompanyID int    `yaml:"company_id" json:"company_id"`
}

// CareersURL returns the company's human-facing career page, e.g.
// https://join.com/companies/routinelabs.
func (c RosterCompany) CareersURL() string {
	return fmt.Sprintf("https://join.com/companies/%s", c.Slug)
}

// Companies holds every confirmed join.com company, sorted by company name.
var Companies = mustLoadCompanies()

// CompaniesBySlug looks up a confirmed company by slug. Keys are
// lowercased, so callers must lowercase their input before indexing.
var CompaniesBySlug = buildSlugIndex(Companies)

// mustLoadCompanies parses the embedded companies.yaml. A parse failure is
// a build-time bug in a file this package owns, not a runtime condition to
// recover from.
func mustLoadCompanies() []RosterCompany {
	var cs []RosterCompany
	if err := yaml.Unmarshal(companiesYAML, &cs); err != nil {
		panic(fmt.Sprintf("join: parse companies.yaml: %v", err))
	}
	slices.SortFunc(cs, func(a, b RosterCompany) int { return strings.Compare(a.Name, b.Name) })
	return cs
}

// buildSlugIndex keeps the first row on a duplicate slug, so a data bug in
// companies.yaml degrades to "one entry ignored" rather than an
// unpredictable pick between rows.
func buildSlugIndex(cs []RosterCompany) map[string]RosterCompany {
	m := make(map[string]RosterCompany, len(cs))
	for _, c := range cs {
		slug := strings.ToLower(c.Slug)
		if _, ok := m[slug]; ok {
			continue
		}
		m[slug] = c
	}
	return m
}
