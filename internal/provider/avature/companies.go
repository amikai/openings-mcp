package avature

import (
	_ "embed"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
)

//go:embed companies.yaml
var companiesYAML []byte

// Company is a confirmed organization with a public Avature career portal.
// URL is the portal base without a locale segment
// (e.g. "https://koch.avature.net/careers").
type Company struct {
	Name string `yaml:"company" json:"company"`
	URL  string `yaml:"url" json:"url"`
}

// Slug returns the portal's roster key: the base URL without its scheme
// (e.g. "koch.avature.net/careers"). Host plus portal name stays unique
// even when one tenant hosts several portals.
func (c Company) Slug() string {
	return strings.TrimPrefix(c.URL, "https://")
}

// CareersURL returns the company's human-facing job search page.
func (c Company) CareersURL() string {
	return c.URL + "/SearchJobs"
}

// Companies holds every confirmed Avature career portal, sorted by company
// name.
var Companies = mustLoadCompanies()

// CompaniesBySlug looks up a confirmed company by lowercase [Company.Slug].
var CompaniesBySlug = buildSlugIndex(Companies)

func mustLoadCompanies() []Company {
	cs, err := loadCompanies(companiesYAML)
	if err != nil {
		panic(fmt.Sprintf("avature: load companies.yaml: %v", err))
	}
	return cs
}

// localeSegmentRE matches an Avature locale path segment such as "en_US".
// Roster URLs must omit it — the portal 302s to its default locale itself.
var localeSegmentRE = regexp.MustCompile(`^[a-z]{2}_[A-Z]{2}$`)

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
		slug := strings.ToLower(c.Slug())
		if prev, ok := slugs[slug]; ok {
			return nil, fmt.Errorf("duplicate portal %q for %q and %q", c.Slug(), prev, c.Name)
		}
		slugs[slug] = c.Name
		if names[strings.ToLower(c.Name)] {
			return nil, fmt.Errorf("duplicate company name %q", c.Name)
		}
		names[strings.ToLower(c.Name)] = true
	}

	if !slices.IsSortedFunc(cs, func(a, b Company) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	}) {
		return nil, fmt.Errorf("companies must be sorted by company name")
	}
	return cs, nil
}

func validateCompany(c Company) error {
	if c.Name == "" {
		return fmt.Errorf("company with url %q has empty name", c.URL)
	}
	u, err := url.Parse(c.URL)
	if err != nil {
		return fmt.Errorf("company %q: parse url: %w", c.Name, err)
	}
	if u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("company %q: url %q must be https://<host>/<portal>", c.Name, c.URL)
	}
	if u.RawQuery != "" || u.Fragment != "" || strings.HasSuffix(u.Path, "/") {
		return fmt.Errorf("company %q: url %q must be a bare portal base", c.Name, c.URL)
	}
	segs := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	if len(segs) != 1 || segs[0] == "" {
		return fmt.Errorf("company %q: url %q must have exactly one path segment (the portal name)", c.Name, c.URL)
	}
	if localeSegmentRE.MatchString(segs[0]) {
		return fmt.Errorf("company %q: url %q must omit the locale segment", c.Name, c.URL)
	}
	return nil
}

func buildSlugIndex(cs []Company) map[string]Company {
	m := make(map[string]Company, len(cs))
	for _, c := range cs {
		m[strings.ToLower(c.Slug())] = c
	}
	return m
}
