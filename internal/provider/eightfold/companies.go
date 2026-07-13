package eightfold

import (
	_ "embed"
	"fmt"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
)

//go:embed companies.yaml
var companiesYAML []byte

// RosterCompany is a confirmed organization hosting a public Eightfold
// career site, drawn from a curated list
// (internal/provider/eightfold/companies.yaml). Every entry was verified
// live against the PCSX search API — HTTP 200 with a nonzero position
// count. Tenant and Domain are the two values every PCSX request needs:
// Tenant picks the <tenant>.eightfold.ai host, and Domain must match the
// tenant's registered company domain exactly, or the server answers with
// its HTML shell instead of JSON.
type RosterCompany struct {
	Name   string `yaml:"company" json:"company"`
	Tenant string `yaml:"tenant" json:"tenant"`
	Domain string `yaml:"domain" json:"domain"`
}

// CareersURL returns the company's human-facing career site.
func (c RosterCompany) CareersURL() string {
	return fmt.Sprintf("https://%s.eightfold.ai/careers", c.Tenant)
}

// Companies holds every confirmed Eightfold company, sorted by company
// name.
var Companies = mustLoadCompanies()

// CompaniesByTenant looks up a confirmed company by tenant slug. Keys are
// lowercased, so callers must lowercase their input before indexing.
var CompaniesByTenant = buildTenantIndex(Companies)

// mustLoadCompanies parses the embedded companies.yaml. A parse failure is
// a build-time bug in a file this package owns, not a runtime condition to
// recover from.
func mustLoadCompanies() []RosterCompany {
	var cs []RosterCompany
	if err := yaml.Unmarshal(companiesYAML, &cs); err != nil {
		panic(fmt.Sprintf("eightfold: parse companies.yaml: %v", err))
	}
	slices.SortFunc(cs, func(a, b RosterCompany) int { return strings.Compare(a.Name, b.Name) })
	return cs
}

// buildTenantIndex keeps the first row on a duplicate tenant slug, so a
// data bug in companies.yaml degrades to "one entry ignored" rather than
// an unpredictable pick between rows.
func buildTenantIndex(cs []RosterCompany) map[string]RosterCompany {
	m := make(map[string]RosterCompany, len(cs))
	for _, c := range cs {
		slug := strings.ToLower(c.Tenant)
		if _, ok := m[slug]; ok {
			continue
		}
		m[slug] = c
	}
	return m
}
