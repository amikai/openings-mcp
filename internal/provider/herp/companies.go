package herp

import (
	_ "embed"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
)

// DefaultBaseURL is the public website base URL for HERP Career.
const DefaultBaseURL = "https://herp.careers"

//go:embed companies.yaml
var companiesYAML []byte

// slugRE matches a HERP Career company slug, the single path segment in
// herp.careers/careers/companies/{slug}.
var slugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// Company is a confirmed organization listed on HERP Career. Slug is the
// path segment in herp.careers/careers/companies/{slug} and the provider's
// tenant key. Name is the company's own legal name, exactly as HERP Career
// renders it. Every curated entry was verified against
// /careers/api/v1/companies/{slug}: HTTP 200, a matching companyName, and a
// non-empty jobs array.
type Company struct {
	Name string `yaml:"company" json:"company"`
	Slug string `yaml:"slug" json:"slug"`
}

// CareersURL returns the company's human-facing HERP Career board.
func (c Company) CareersURL() string {
	return "https://herp.careers/careers/companies/" + c.Slug
}

// Companies holds every confirmed HERP Career company, sorted by company name.
var Companies = mustLoadCompanies()

// CompaniesBySlug looks up a confirmed company by lowercase slug.
var CompaniesBySlug = buildSlugIndex(Companies)

func mustLoadCompanies() []Company {
	cs, err := loadCompanies(companiesYAML)
	if err != nil {
		panic(fmt.Sprintf("herp: load companies.yaml: %v", err))
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
	case !slugRE.MatchString(c.Slug):
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
