package dayforce

import (
	_ "embed"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/goccy/go-yaml"
)

//go:embed companies.yaml
var companiesYAML []byte

// defaultCultureCode is used when a roster row omits culture_code, matching
// [BoardClient]'s culture pinning: an unsupported culture returns 200 with
// an empty board rather than an error, so it must be pinned explicitly.
const defaultCultureCode = "en-US"

// candidatePortalBoardCode is the default job board code most tenants serve;
// it does not appear in a Slug, since it needs no board disambiguation.
const candidatePortalBoardCode = "CANDIDATEPORTAL"

// Company is a confirmed Dayforce tenant board. A tenant can carry several
// boards under distinct job_board_code/job_board_id pairs — see the
// mydayforce/alljobs board in client_test.go — so both are stored explicitly
// alongside the tenant Namespace rather than assumed to be the default.
type Company struct {
	Name         string `yaml:"company" json:"company"`
	Namespace    string `yaml:"namespace" json:"namespace"`
	JobBoardCode string `yaml:"job_board_code" json:"job_board_code"`
	JobBoardID   int    `yaml:"job_board_id" json:"job_board_id"`
	CultureCode  string `yaml:"culture_code" json:"culture_code,omitempty"`
}

// Culture returns c.CultureCode, defaulting to en-US when unset.
func (c Company) Culture() string {
	if c.CultureCode == "" {
		return defaultCultureCode
	}
	return c.CultureCode
}

// Slug identifies this board within the dayforce roster: the tenant
// namespace, suffixed with the lowercased job board code when the board is
// not the default CANDIDATEPORTAL board, so a multi-board tenant can carry
// more than one board without its slugs colliding.
func (c Company) Slug() string {
	if strings.EqualFold(c.JobBoardCode, candidatePortalBoardCode) {
		return c.Namespace
	}
	return c.Namespace + "-" + strings.ToLower(c.JobBoardCode)
}

// Companies holds every confirmed Dayforce board, sorted by company name.
var Companies = mustLoadCompanies()

// CompaniesBySlug looks up a confirmed company by lowercase [Company.Slug].
var CompaniesBySlug = buildSlugIndex(Companies)

func mustLoadCompanies() []Company {
	cs, err := loadCompanies(companiesYAML)
	if err != nil {
		panic(fmt.Sprintf("dayforce: load companies.yaml: %v", err))
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
		slug := strings.ToLower(c.Slug())
		if prev, ok := slugs[slug]; ok {
			return nil, fmt.Errorf("duplicate slug %q for %q and %q", c.Slug(), prev, c.Name)
		}
		nameKey := normalizeName(c.Name)
		if names[nameKey] {
			return nil, fmt.Errorf("duplicate company name %q", c.Name)
		}
		slugs[slug] = c.Name
		names[nameKey] = true
	}

	slices.SortFunc(cs, func(a, b Company) int { return strings.Compare(a.Name, b.Name) })
	return cs, nil
}

// normalizeName folds case and strips everything but letters and digits, so
// "Foo Inc." and "foo inc" collide as duplicates. This mirrors
// internal/ats.normalize: that registry is what decides whether the MCP
// server can start, so a punctuation-variant duplicate that slips past this
// check would only surface later as a server-startup failure instead of a
// `go test` failure here. It is duplicated rather than imported to avoid a
// dependency from this provider package onto internal/ats.
func normalizeName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func validateCompany(c Company) error {
	switch {
	case strings.TrimSpace(c.Name) == "":
		return errors.New("company name is required")
	case c.Namespace == "":
		return fmt.Errorf("company %q: namespace is required", c.Name)
	case c.JobBoardCode == "":
		return fmt.Errorf("company %q: job_board_code is required", c.Name)
	case c.JobBoardID <= 0:
		return fmt.Errorf("company %q: job_board_id must be > 0", c.Name)
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
