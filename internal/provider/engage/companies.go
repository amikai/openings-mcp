package engage

import (
	_ "embed"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
)

//go:embed companies.yaml
var _companiesYAML []byte

// slugRE matches an engage tenant slug, the single path segment in
// en-gage.net/{slug}/. Slugs use lowercase letters, digits, hyphens and
// underscores, and may start with a digit (e.g. "2918").
var _slugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// Company is a confirmed organization listed on en-gage.net. Slug is the
// path segment in en-gage.net/{slug}/ and the provider's tenant key. Name is
// the company's own name, exactly as en-gage.net renders it. Every curated
// entry was verified live: HTTP 200 on the tenant board, a matching company
// name, and at least one parsed job.
type Company struct {
	Name string `yaml:"company" json:"company"`
	Slug string `yaml:"slug" json:"slug"`
}

// CareersURL returns the company's human-facing engage tenant board.
func (c Company) CareersURL() string {
	return "https://en-gage.net/" + c.Slug + "/"
}

// Companies holds every confirmed engage company, sorted by company name.
var Companies = mustLoadCompanies()

// CompaniesBySlug looks up a confirmed company by slug.
var CompaniesBySlug = buildSlugIndex(Companies)

func mustLoadCompanies() []Company {
	cs, err := loadCompanies(_companiesYAML)
	if err != nil {
		panic(fmt.Sprintf("engage: load companies.yaml: %v", err))
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
	case !_slugRE.MatchString(c.Slug):
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
