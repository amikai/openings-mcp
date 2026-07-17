package ultipro

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
var companiesYAML []byte

// hostRE matches the two UltiPro hosts observed in live traffic
// (recruiting.ultipro.com, recruiting2.ultipro.com); an optional trailing
// digit tolerates further numbered hosts UKG may add.
var hostRE = regexp.MustCompile(`^recruiting\d*\.ultipro\.com$`)

// boardIDRE matches a lowercase GUID, the observed board-id shape.
var boardIDRE = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Company is a confirmed UltiPro tenant board. Host varies per tenant
// (recruiting.ultipro.com vs recruiting2.ultipro.com, verified live — see
// companies.yaml), so it is stored explicitly alongside CompanyCode and
// BoardID rather than derived.
type Company struct {
	Name        string `yaml:"company" json:"company"`
	Host        string `yaml:"host" json:"host"`
	CompanyCode string `yaml:"company_code" json:"company_code"`
	BoardID     string `yaml:"board_id" json:"board_id"`
}

// CareersURL returns the company's human-facing job board page.
func (c Company) CareersURL() string {
	return fmt.Sprintf("https://%s/%s/JobBoard/%s/", c.Host, c.CompanyCode, c.BoardID)
}

// BaseURL returns the board's API base URL, for [NewClient].
func (c Company) BaseURL() string {
	return fmt.Sprintf("https://%s/%s/JobBoard/%s", c.Host, c.CompanyCode, c.BoardID)
}

// Companies holds every confirmed UltiPro board, sorted by company name.
var Companies = mustLoadCompanies()

// CompaniesByCode looks up a confirmed company by lowercase company code.
var CompaniesByCode = buildCodeIndex(Companies)

func mustLoadCompanies() []Company {
	cs, err := loadCompanies(companiesYAML)
	if err != nil {
		panic(fmt.Sprintf("ultipro: load companies.yaml: %v", err))
	}
	return cs
}

func loadCompanies(data []byte) ([]Company, error) {
	var cs []Company
	if err := yaml.Unmarshal(data, &cs); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	codes := make(map[string]string, len(cs))
	names := make(map[string]bool, len(cs))
	for _, c := range cs {
		if err := validateCompany(c); err != nil {
			return nil, err
		}
		code := strings.ToLower(c.CompanyCode)
		if prev, ok := codes[code]; ok {
			return nil, fmt.Errorf("duplicate company_code %q for %q and %q", c.CompanyCode, prev, c.Name)
		}
		if names[strings.ToLower(c.Name)] {
			return nil, fmt.Errorf("duplicate company name %q", c.Name)
		}
		codes[code] = c.Name
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
	case !hostRE.MatchString(c.Host):
		return fmt.Errorf("company %q: host %q must match recruiting<N>.ultipro.com", c.Name, c.Host)
	case c.CompanyCode == "":
		return fmt.Errorf("company %q: company_code is required", c.Name)
	case c.BoardID == "":
		return fmt.Errorf("company %q: board_id is required", c.Name)
	case !boardIDRE.MatchString(c.BoardID):
		return fmt.Errorf("company %q: board_id %q must be a lowercase GUID", c.Name, c.BoardID)
	}
	return nil
}

func buildCodeIndex(cs []Company) map[string]Company {
	m := make(map[string]Company, len(cs))
	for _, c := range cs {
		m[strings.ToLower(c.CompanyCode)] = c
	}
	return m
}
