package adp_myjobs

import (
	_ "embed"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
)

//go:embed companies.yaml
var companiesYAML []byte

// Company is a confirmed organization with a public ADP MyJobs career site.
// Slug is the myjobs.adp.com path segment and the provider's tenant key.
type Company struct {
	Name string `yaml:"company" json:"company"`
	Slug string `yaml:"slug" json:"slug"`
}

// CareersURL returns the company's human-facing MyJobs careers page.
func (c Company) CareersURL() string {
	return fmt.Sprintf("https://myjobs.adp.com/%s/cx", c.Slug)
}

// Companies holds every confirmed MyJobs site, sorted by company name.
var Companies = mustLoadCompanies()

// CompaniesBySlug looks up a confirmed company by lowercase slug.
var CompaniesBySlug = buildSlugIndex(Companies)

func mustLoadCompanies() []Company {
	cs, err := loadCompanies(companiesYAML)
	if err != nil {
		panic(fmt.Sprintf("adp_myjobs: load companies.yaml: %v", err))
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
	for i := range cs {
		c := &cs[i]
		c.Slug = strings.ToLower(strings.TrimSpace(c.Slug))
		c.Name = strings.TrimSpace(c.Name)
		if err := validateCompany(*c); err != nil {
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
	case c.Name == "":
		return errors.New("company name is required")
	case c.Slug == "":
		return errors.New("slug is required")
	case strings.ContainsAny(c.Slug, "/ \t"):
		return fmt.Errorf("slug %q must be a single path segment", c.Slug)
	default:
		return nil
	}
}

func buildSlugIndex(cs []Company) map[string]Company {
	m := make(map[string]Company, len(cs))
	for _, c := range cs {
		m[c.Slug] = c
	}
	return m
}
