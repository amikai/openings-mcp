package icims

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

// Company is a confirmed organization with a public iCIMS career portal.
// Host is the portal hostname (e.g. "careers-peraton.icims.com").
// Every curated entry was verified against /jobs/search?ss=1&pr=0&in_iframe=1:
// HTTP 200 and at least one iCIMS_JobCardItem.
type Company struct {
	Name string `yaml:"company" json:"company"`
	Host string `yaml:"host" json:"host"`
}

// careersURLTpl formats an iCIMS job search page URL (e.g. "https://careers-peraton.icims.com/jobs/search?ss=1").
const careersURLTpl = "https://%s/jobs/search?ss=1"

// CareersURL returns the company's human-facing job search page.
func (c Company) CareersURL() string {
	return fmt.Sprintf(careersURLTpl, c.Host)
}

// Companies holds every confirmed iCIMS career portal, sorted by company name.
var Companies = mustLoadCompanies()

// CompaniesByHost looks up a confirmed company by lowercase host.
var CompaniesByHost = buildHostIndex(Companies)

func mustLoadCompanies() []Company {
	cs, err := loadCompanies(companiesYAML)
	if err != nil {
		panic(fmt.Sprintf("icims: load companies.yaml: %v", err))
	}
	return cs
}

func loadCompanies(data []byte) ([]Company, error) {
	var cs []Company
	if err := yaml.Unmarshal(data, &cs); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	hosts := make(map[string]string, len(cs))
	names := make(map[string]bool, len(cs))
	for _, c := range cs {
		if err := validateCompany(c); err != nil {
			return nil, err
		}
		if prev, ok := hosts[c.Host]; ok {
			return nil, fmt.Errorf("duplicate host %q for %q and %q", c.Host, prev, c.Name)
		}
		if names[strings.ToLower(c.Name)] {
			return nil, fmt.Errorf("duplicate company name %q", c.Name)
		}
		hosts[c.Host] = c.Name
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
	case !strings.HasSuffix(c.Host, ".icims.com"):
		return fmt.Errorf("company %q: host %q must end with .icims.com", c.Name, c.Host)
	}

	u, err := url.Parse("https://" + c.Host)
	if err != nil || u.Hostname() != c.Host || u.Port() != "" {
		return fmt.Errorf("company %q: invalid host %q", c.Name, c.Host)
	}
	return nil
}

func buildHostIndex(cs []Company) map[string]Company {
	m := make(map[string]Company, len(cs))
	for _, c := range cs {
		m[c.Host] = c
	}
	return m
}
