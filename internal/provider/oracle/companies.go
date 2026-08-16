package oracle

import (
	_ "embed"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
)

//go:embed companies.yaml
var companiesYAML []byte

// Company is a confirmed organization with a public Oracle Recruiting Cloud
// career site. Host is the Fusion API/career hostname (e.g.
// "jpmc.fa.oraclecloud.com"). Site is the public /sites/<site> path segment;
// SiteNumber is the internal finder value from the career-site HTML
// (data-sitenumber), which can differ from Site.
type Company struct {
	Name       string `yaml:"company" json:"company"`
	Host       string `yaml:"host" json:"host"`
	Site       string `yaml:"site" json:"site"`
	SiteNumber string `yaml:"site_number" json:"site_number"`
}

// careersURLTpl formats an Oracle Recruiting Cloud job search page URL.
// Parameters: 1: Host (e.g. "jpmc.fa.oraclecloud.com"), 2: Site (e.g. "CX_1").
// Example: "https://jpmc.fa.oraclecloud.com/hcmUI/CandidateExperience/en/sites/CX_1/jobs"
const careersURLTpl = "https://%s/hcmUI/CandidateExperience/en/sites/%s/jobs"

// apiBaseURLTpl formats an Oracle Fusion API origin URL.
// Parameters: 1: Host (e.g. "jpmc.fa.oraclecloud.com").
// Example: "https://jpmc.fa.oraclecloud.com"
const apiBaseURLTpl = "https://%s"

// CareersURL returns the company's human-facing job search page.
func (c Company) CareersURL() string {
	return fmt.Sprintf(careersURLTpl, c.Host, c.Site)
}

// APIBaseURL returns the Fusion origin used for Candidate Experience REST calls.
func (c Company) APIBaseURL() string {
	return fmt.Sprintf(apiBaseURLTpl, c.Host)
}

// Companies holds every confirmed Oracle career site, sorted by company name.
var Companies = mustLoadCompanies()

// CompaniesByKey looks up a confirmed company by "host/site_number" (lowercase host).
var CompaniesByKey = buildKeyIndex(Companies)

func mustLoadCompanies() []Company {
	cs, err := loadCompanies(companiesYAML)
	if err != nil {
		panic(fmt.Sprintf("oracle: load companies.yaml: %v", err))
	}
	return cs
}

func loadCompanies(data []byte) ([]Company, error) {
	var cs []Company
	if err := yaml.Unmarshal(data, &cs); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	keys := make(map[string]string, len(cs))
	names := make(map[string]bool, len(cs))
	for _, c := range cs {
		if err := validateCompany(c); err != nil {
			return nil, err
		}
		key := companyKey(c.Host, c.SiteNumber)
		if prev, ok := keys[key]; ok {
			return nil, fmt.Errorf("duplicate host/site_number %q for %q and %q", key, prev, c.Name)
		}
		if names[strings.ToLower(c.Name)] {
			return nil, fmt.Errorf("duplicate company name %q", c.Name)
		}
		keys[key] = c.Name
		names[strings.ToLower(c.Name)] = true
	}

	slices.SortFunc(cs, func(a, b Company) int { return strings.Compare(a.Name, b.Name) })
	return cs, nil
}

func validateCompany(c Company) error {
	switch {
	case strings.TrimSpace(c.Name) == "":
		return errors.New("company name is required")
	case c.Host == "":
		return fmt.Errorf("company %q: host is required", c.Name)
	case c.Host != strings.ToLower(c.Host):
		return fmt.Errorf("company %q: host %q must be lowercase", c.Name, c.Host)
	case !strings.HasSuffix(c.Host, ".oraclecloud.com"):
		return fmt.Errorf("company %q: host %q must be an oraclecloud.com Fusion host", c.Name, c.Host)
	case strings.TrimSpace(c.Site) == "":
		return fmt.Errorf("company %q: site is required", c.Name)
	case strings.TrimSpace(c.SiteNumber) == "":
		return fmt.Errorf("company %q: site_number is required", c.Name)
	}

	u, err := url.Parse("https://" + c.Host)
	if err != nil || u.Hostname() != c.Host || u.Port() != "" {
		return fmt.Errorf("company %q: invalid host %q", c.Name, c.Host)
	}
	return nil
}

func companyKey(host, siteNumber string) string {
	return strings.ToLower(host) + "/" + siteNumber
}

func buildKeyIndex(cs []Company) map[string]Company {
	m := make(map[string]Company, len(cs))
	for _, c := range cs {
		m[companyKey(c.Host, c.SiteNumber)] = c
	}
	return m
}
