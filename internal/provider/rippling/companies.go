package rippling

import (
	_ "embed"
	"fmt"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
)

//go:embed companies.yaml
var companiesYAML []byte

// Company is a confirmed organization hosting a public Rippling job board,
// drawn from a curated list (internal/provider/rippling/companies.yaml).
// Every entry was verified against the live Job Board API — HTTP 200 with a
// non-empty jobs array and a matching companyName on a sampled job detail;
// cmd/verify-companies re-verifies the roster.
// BoardSlug is the identifier the API takes as its board_slug path
// parameter, e.g. "pythian" for
// api.rippling.com/platform/api/ats/v1/board/pythian/jobs.
type Company struct {
	Name      string `yaml:"company" json:"company"`
	BoardSlug string `yaml:"board_slug" json:"board_slug"`
}

// BoardURL returns the company's human-facing job board page, e.g.
// https://ats.rippling.com/pythian/jobs. API calls instead pass BoardSlug
// directly as the board_slug parameter.
func (c Company) BoardURL() string {
	return fmt.Sprintf("https://ats.rippling.com/%s/jobs", c.BoardSlug)
}

// Companies holds every confirmed Rippling board, sorted by company name.
var Companies = mustLoadCompanies()

// CompaniesByBoardSlug looks up a confirmed company by board slug. Keys are
// lowercased, so callers must lowercase their input before indexing.
var CompaniesByBoardSlug = buildBoardSlugIndex(Companies)

// mustLoadCompanies parses the embedded companies.yaml. A parse failure is a
// build-time bug in a file this package owns, not a runtime condition to
// recover from.
func mustLoadCompanies() []Company {
	var cs []Company
	if err := yaml.Unmarshal(companiesYAML, &cs); err != nil {
		panic(fmt.Sprintf("rippling: parse companies.yaml: %v", err))
	}
	slices.SortFunc(cs, func(a, b Company) int { return strings.Compare(a.Name, b.Name) })
	return cs
}

func buildBoardSlugIndex(cs []Company) map[string]Company {
	m := make(map[string]Company, len(cs))
	for _, c := range cs {
		m[strings.ToLower(c.BoardSlug)] = c
	}
	return m
}
