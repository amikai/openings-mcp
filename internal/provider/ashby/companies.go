package ashby

import (
	_ "embed"
	"fmt"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
)

//go:embed companies.yaml
var companiesYAML []byte

// Company is a confirmed organization hosting a public Ashby job board,
// drawn from a curated list (internal/provider/ashby/companies.yaml). Every
// entry was verified against the live posting API — HTTP 200 with a
// non-empty jobs array — and its board page title checked against the
// expected company name; testdata/verify_companies.sh re-verifies the
// roster. It's keyed by board slug (e.g. "openai"), the same identifier the
// API takes as its jobBoardName path parameter.
type Company struct {
	Name  string `yaml:"company" json:"company"`
	Board string `yaml:"board" json:"board"`
}

// BoardURL returns the company's human-facing job board page, e.g.
// https://jobs.ashbyhq.com/openai. API calls instead pass Board directly as
// the jobBoardName parameter.
func (c Company) BoardURL() string {
	return fmt.Sprintf("https://jobs.ashbyhq.com/%s", c.Board)
}

// Companies holds every confirmed Ashby board, sorted by company name.
var Companies = mustLoadCompanies()

// CompaniesByBoard looks up a confirmed company by board slug. Keys are
// lowercased, so callers must lowercase their input before indexing.
var CompaniesByBoard = buildBoardIndex(Companies)

// mustLoadCompanies parses the embedded companies.yaml. A parse failure is a
// build-time bug in a file this package owns, not a runtime condition to
// recover from.
func mustLoadCompanies() []Company {
	var cs []Company
	if err := yaml.Unmarshal(companiesYAML, &cs); err != nil {
		panic(fmt.Sprintf("ashby: parse companies.yaml: %v", err))
	}
	slices.SortFunc(cs, func(a, b Company) int { return strings.Compare(a.Name, b.Name) })
	return cs
}

func buildBoardIndex(cs []Company) map[string]Company {
	m := make(map[string]Company, len(cs))
	for _, c := range cs {
		m[strings.ToLower(c.Board)] = c
	}
	return m
}
