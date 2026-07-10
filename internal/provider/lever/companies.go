package lever

import (
	_ "embed"
	"fmt"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
)

//go:embed companies.yaml
var companiesYAML []byte

// Company is a confirmed Lever tenant from the curated
// internal/provider/lever/companies.yaml. Site is the slug that namespaces
// the company's postings, e.g. "leverdemo" for jobs.lever.co/leverdemo —
// slugs are unique and lowercase, unlike display names. Every entry was
// verified to return a non-empty postings array from the global instance
// (api.lever.co) at collection time.
type Company struct {
	Name string `yaml:"company" json:"company"`
	Site string `yaml:"site" json:"site"`
}

// Companies holds every confirmed Lever tenant, sorted by company name.
var Companies = mustLoadCompanies()

// CompaniesBySite looks up a confirmed tenant by site slug. Keys are
// lowercase slugs as they appear in companies.yaml.
var CompaniesBySite = buildSiteIndex(Companies)

// mustLoadCompanies parses the embedded companies.yaml. A parse failure is
// a build-time bug in a file this package owns, not a runtime condition to
// recover from.
func mustLoadCompanies() []Company {
	var cs []Company
	if err := yaml.Unmarshal(companiesYAML, &cs); err != nil {
		panic(fmt.Sprintf("lever: parse companies.yaml: %v", err))
	}
	slices.SortFunc(cs, func(a, b Company) int { return strings.Compare(a.Name, b.Name) })
	return cs
}

func buildSiteIndex(cs []Company) map[string]Company {
	m := make(map[string]Company, len(cs))
	for _, c := range cs {
		m[c.Site] = c
	}
	return m
}
