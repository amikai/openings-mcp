package workable

import (
	_ "embed"
	"fmt"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
)

//go:embed companies.yaml
var companiesYAML []byte

// RosterCompany is a confirmed organization hosting a public Workable careers
// page (internal/provider/workable/companies.yaml). Every entry was verified
// against the live job board API — HTTP 200 with total > 0 and a display name
// matching the account metadata endpoint.
// Account is the subdomain the API takes as its account path parameter, e.g.
// "blueground" for apply.workable.com/api/v3/accounts/blueground/jobs.
type RosterCompany struct {
	Name    string `yaml:"company" json:"company"`
	Account string `yaml:"account" json:"account"`
}

// CareersURL returns the company's human-facing careers page, e.g.
// https://apply.workable.com/blueground/. API calls instead pass Account
// directly as the account parameter.
func (c RosterCompany) CareersURL() string {
	return fmt.Sprintf("https://apply.workable.com/%s/", c.Account)
}

// Companies holds every confirmed Workable company, sorted by company name.
var Companies = mustLoadCompanies()

// CompaniesByAccount looks up a confirmed company by account subdomain. Keys
// are lowercased, so callers must lowercase their input before indexing.
var CompaniesByAccount = buildAccountIndex(Companies)

// mustLoadCompanies parses the embedded companies.yaml. A parse failure is a
// build-time bug in a file this package owns, not a runtime condition to
// recover from.
func mustLoadCompanies() []RosterCompany {
	var cs []RosterCompany
	if err := yaml.Unmarshal(companiesYAML, &cs); err != nil {
		panic(fmt.Sprintf("workable: parse companies.yaml: %v", err))
	}
	slices.SortFunc(cs, func(a, b RosterCompany) int { return strings.Compare(a.Name, b.Name) })
	return cs
}

// buildAccountIndex keeps the first row on a duplicate account, so a data bug
// in companies.yaml degrades to "one entry ignored" rather than an
// unpredictable pick between rows.
func buildAccountIndex(cs []RosterCompany) map[string]RosterCompany {
	m := make(map[string]RosterCompany, len(cs))
	for _, c := range cs {
		slug := strings.ToLower(c.Account)
		if _, ok := m[slug]; ok {
			continue
		}
		m[slug] = c
	}
	return m
}
