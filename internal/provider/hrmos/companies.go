package hrmos

import (
	_ "embed"
	"fmt"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
)

//go:embed companies.yaml
var companiesYAML []byte

// RosterCompany is a confirmed organization hosting a public HRMOS 採用
// tenant, drawn from a curated list (internal/provider/hrmos/companies.yaml).
// Every entry was verified live: /pages/{slug}/jobs returns HTTP 200 with
// jobs present and a matching company name (see internal/cli/verifycompanies).
type RosterCompany struct {
	Name string `yaml:"company" json:"company"`
	Slug string `yaml:"slug" json:"slug"`
}

// careersURLTpl formats an HRMOS jobs page URL (e.g. "https://hrmos.co/pages/moneyforward/jobs").
const careersURLTpl = "https://hrmos.co/pages/%s/jobs"

// CareersURL returns the company's human-facing jobs page, e.g.
// https://hrmos.co/pages/moneyforward/jobs.
func (c RosterCompany) CareersURL() string {
	return fmt.Sprintf(careersURLTpl, c.Slug)
}

// Companies holds every confirmed HRMOS company, sorted by company name.
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
		panic(fmt.Sprintf("hrmos: parse companies.yaml: %v", err))
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
