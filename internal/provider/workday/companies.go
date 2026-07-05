package workday

import (
	_ "embed"
	"fmt"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
)

//go:embed companies.yaml
var companiesYAML []byte

// Company is a confirmed Workday CXS tenant for a public company, drawn from
// a curated S&P 500 list (internal/provider/workday/companies.yaml). It's
// keyed by tenant slug (e.g. "3m", "att") rather than display name — tenant
// slugs are unique*, lowercase, and punctuation-free, unlike display names
// such as "AT&T" or "Workday, Inc.".
//
// *Two harmless exceptions share a tenant across two rows with an identical
// instance/site (Fox Corporation's two share classes under "fox", News
// Corp's two share classes under "dowjones") — both rows resolve to the same
// BaseURL either way, so which one a lookup returns doesn't matter.
type Company struct {
	Name     string `yaml:"company" json:"company"`
	Tenant   string `yaml:"tenant" json:"tenant"`
	Instance string `yaml:"instance" json:"instance"`
	Site     string `yaml:"site" json:"site"`
}

// BaseURL builds this company's Workday CXS base URL, e.g.
// https://3m.wd1.myworkdayjobs.com/wday/cxs/3m/Search — the same
// {tenant}.{instance}.myworkdayjobs.com/wday/cxs/{tenant}/{site} shape
// documented on PublicSiteURL in path.go.
func (c Company) BaseURL() string {
	return fmt.Sprintf("https://%s.%s.myworkdayjobs.com/wday/cxs/%s/%s", c.Tenant, c.Instance, c.Tenant, c.Site)
}

var (
	companies         = mustLoadCompanies()
	companiesByTenant = buildTenantIndex(companies)
)

// mustLoadCompanies parses the embedded companies.yaml. A parse failure is a
// build-time bug in a file this package owns, not a runtime condition to
// recover from.
func mustLoadCompanies() []Company {
	var cs []Company
	if err := yaml.Unmarshal(companiesYAML, &cs); err != nil {
		panic(fmt.Sprintf("workday: parse companies.yaml: %v", err))
	}
	sort.Slice(cs, func(i, j int) bool { return cs[i].Name < cs[j].Name })
	return cs
}

func buildTenantIndex(cs []Company) map[string]Company {
	m := make(map[string]Company, len(cs))
	for _, c := range cs {
		m[strings.ToLower(c.Tenant)] = c
	}
	return m
}

// Companies returns every confirmed Workday tenant, sorted by company name.
func Companies() []Company {
	return companies
}

// CompanyByTenant looks up a confirmed tenant by slug, case-insensitively.
func CompanyByTenant(tenant string) (Company, bool) {
	c, ok := companiesByTenant[strings.ToLower(tenant)]
	return c, ok
}
