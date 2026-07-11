package greenhouse

import (
	_ "embed"
	"fmt"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
)

//go:embed companies.yaml
var companiesYAML []byte

// Company is a confirmed organization hosting a public Greenhouse job
// board, drawn from a curated list (internal/provider/greenhouse/companies.yaml).
// Every entry was verified against the live Job Board API — HTTP 200 with
// a non-empty jobs array; cmd/verify-companies re-verifies the roster.
// BoardToken is the identifier the API takes as its board_token
// path parameter, e.g. "anthropic" for boards-api.greenhouse.io/v1/boards/anthropic/jobs.
type Company struct {
	Name       string `yaml:"company" json:"company"`
	BoardToken string `yaml:"board_token" json:"board_token"`
}

// BoardURL returns the company's human-facing job board page, e.g.
// https://job-boards.greenhouse.io/anthropic. API calls instead pass
// BoardToken directly as the board_token parameter.
func (c Company) BoardURL() string {
	return fmt.Sprintf("https://job-boards.greenhouse.io/%s", c.BoardToken)
}

// Companies holds every confirmed Greenhouse board, sorted by company name.
var Companies = mustLoadCompanies()

// CompaniesByBoardToken looks up a confirmed company by board token. Keys
// are lowercased, so callers must lowercase their input before indexing.
var CompaniesByBoardToken = buildBoardTokenIndex(Companies)

// mustLoadCompanies parses the embedded companies.yaml. A parse failure is a
// build-time bug in a file this package owns, not a runtime condition to
// recover from.
func mustLoadCompanies() []Company {
	var cs []Company
	if err := yaml.Unmarshal(companiesYAML, &cs); err != nil {
		panic(fmt.Sprintf("greenhouse: parse companies.yaml: %v", err))
	}
	slices.SortFunc(cs, func(a, b Company) int { return strings.Compare(a.Name, b.Name) })
	return cs
}

func buildBoardTokenIndex(cs []Company) map[string]Company {
	m := make(map[string]Company, len(cs))
	for _, c := range cs {
		m[strings.ToLower(c.BoardToken)] = c
	}
	return m
}
