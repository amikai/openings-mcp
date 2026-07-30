package bamboohr

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

// Company is a confirmed organization with a public BambooHR hosted careers
// site. Slug is the site's subdomain and the provider's tenant key. Every
// curated entry was verified against /careers/list - HTTP 200 with a
// non-empty result array - and its careers page's og:site_name checked
// against the expected company name; internal/cli/verifycompanies re-verifies the
// roster.
type Company struct {
	Name string `yaml:"company" json:"company"`
	Slug string `yaml:"slug" json:"slug"`
}

// careersURLTpl formats a BambooHR careers page URL (e.g. "https://curtinmaritime.bamboohr.com/careers").
const careersURLTpl = "https://%s.bamboohr.com/careers"

// CareersURL returns the company's human-facing BambooHR careers page.
func (c Company) CareersURL() string {
	return fmt.Sprintf(careersURLTpl, c.Slug)
}

// Companies holds every confirmed BambooHR careers site, sorted by company
// name.
var Companies = mustLoadCompanies()

// CompaniesBySlug looks up a confirmed company by lowercase subdomain slug.
var CompaniesBySlug = buildSlugIndex(Companies)

func mustLoadCompanies() []Company {
	cs, err := loadCompanies(companiesYAML)
	if err != nil {
		panic(fmt.Sprintf("bamboohr: load companies.yaml: %v", err))
	}
	return cs
}

func loadCompanies(data []byte) ([]Company, error) {
	var cs []Company
	if err := yaml.Unmarshal(data, &cs); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	slugs := make(map[string]string, len(cs))
	names := make(map[string]bool, len(cs))
	for _, c := range cs {
		if err := validateCompany(c); err != nil {
			return nil, err
		}
		if prev, ok := slugs[c.Slug]; ok {
			return nil, fmt.Errorf("duplicate slug %q for %q and %q", c.Slug, prev, c.Name)
		}
		if names[strings.ToLower(c.Name)] {
			return nil, fmt.Errorf("duplicate company name %q", c.Name)
		}
		slugs[c.Slug] = c.Name
		names[strings.ToLower(c.Name)] = true
	}

	slices.SortFunc(cs, func(a, b Company) int { return strings.Compare(a.Name, b.Name) })
	return cs, nil
}

func validateCompany(c Company) error {
	switch {
	case strings.TrimSpace(c.Name) == "":
		return errors.New("company name is required")
	case c.Slug == "":
		return fmt.Errorf("company %q: slug is required", c.Name)
	case c.Slug != strings.ToLower(c.Slug):
		return fmt.Errorf("company %q: slug %q must be lowercase", c.Name, c.Slug)
	}

	u, err := url.Parse("https://" + c.Slug + ".bamboohr.com")
	if err != nil || u.Hostname() != c.Slug+".bamboohr.com" || u.Port() != "" {
		return fmt.Errorf("company %q: invalid slug %q", c.Name, c.Slug)
	}
	return nil
}

func buildSlugIndex(cs []Company) map[string]Company {
	m := make(map[string]Company, len(cs))
	for _, c := range cs {
		m[c.Slug] = c
	}
	return m
}
