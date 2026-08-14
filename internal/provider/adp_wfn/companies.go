package adp_wfn

import (
	_ "embed"
	"fmt"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
)

//go:embed companies.yaml
var companiesYAML []byte

// Company is a confirmed Workforce Now career center
// (internal/provider/adp_wfn/companies.yaml).
//
// Slug is curated here rather than taken from ADP. The API knows a tenant
// only by CID, a GUID, and ADP's own ClientID is opaque for a good share of
// tenants ("1euxj", "xip39"); either would land in the registry's
// did-you-mean pool, which is built from slugs and never from names, and make
// these companies unreachable to anyone who mistypes.
type Company struct {
	Name string `yaml:"company"   json:"company"`
	Slug string `yaml:"slug"      json:"slug"`
	// CID is the tenant GUID, and the only key the API accepts.
	CID string `yaml:"cid"       json:"cid"`
	// CCID is the career-center id. It is never sent to the API — a wrong
	// value there returns an empty board instead of an error, and omitting it
	// returned the right board on every tenant tested — but it appears in the
	// URLs tenants publish, so it is kept for rendering human-facing links.
	CCID string `yaml:"cc_id"     json:"cc_id"`
	// ClientID is ADP's readable tenant slug, kept as the input the legacy
	// careers-URL resolver takes. Not used to address the API.
	ClientID string `yaml:"client_id" json:"client_id"`
	// Locale is the one locale this tenant's postings exist under. Required:
	// naming another locale the tenant recognizes returns an empty board
	// rather than an error, so it can never be defaulted safely.
	Locale string `yaml:"locale"    json:"locale"`
}

// CareersURL returns the company's public job-listing page.
func (c Company) CareersURL() string { return BoardURL(c.CID, c.CCID, c.Locale) }

// Companies holds every confirmed career center, sorted by company name.
var Companies = mustLoadCompanies()

// CompaniesBySlug looks up a confirmed company by slug. Keys are lowercased,
// so callers must lowercase their input before indexing.
var CompaniesBySlug = buildSlugIndex(Companies)

// CompaniesByCID looks up a confirmed company by tenant GUID, which is what a
// careers URL yields. Keys are lowercased.
var CompaniesByCID = buildCIDIndex(Companies)

// mustLoadCompanies parses the embedded companies.yaml. A parse failure is a
// build-time bug in a file this package owns, not a runtime condition to
// recover from.
func mustLoadCompanies() []Company {
	var cs []Company
	if err := yaml.Unmarshal(companiesYAML, &cs); err != nil {
		panic(fmt.Sprintf("adp_wfn: parse companies.yaml: %v", err))
	}
	for _, c := range cs {
		if c.Name == "" || c.Slug == "" || c.CID == "" || c.Locale == "" {
			panic(fmt.Sprintf("adp_wfn: companies.yaml entry %+v is missing company, slug, cid, or locale", c))
		}
	}
	slices.SortFunc(cs, func(a, b Company) int { return strings.Compare(a.Name, b.Name) })
	return cs
}

func buildSlugIndex(cs []Company) map[string]Company {
	m := make(map[string]Company, len(cs))
	for _, c := range cs {
		m[strings.ToLower(c.Slug)] = c
	}
	return m
}

func buildCIDIndex(cs []Company) map[string]Company {
	m := make(map[string]Company, len(cs))
	for _, c := range cs {
		m[strings.ToLower(c.CID)] = c
	}
	return m
}
