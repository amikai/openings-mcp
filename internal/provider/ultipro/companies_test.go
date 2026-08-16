package ultipro

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompaniesLoaded(t *testing.T) {
	require.NotEmpty(t, Companies)
	for _, c := range Companies {
		assert.NotEmpty(t, c.Name)
		assert.True(t, hostRE.MatchString(c.Host), "host %q", c.Host)
		assert.True(t, boardIDRE.MatchString(c.BoardID), "board_id %q", c.BoardID)
		assert.NotEmpty(t, c.CompanyCode)
		got, ok := CompaniesByCode[strings.ToLower(c.CompanyCode)]
		require.True(t, ok, "code index missing %q", c.CompanyCode)
		assert.Equal(t, c, got)
	}
	// Sorted by name.
	for i := 1; i < len(Companies); i++ {
		assert.LessOrEqual(t, Companies[i-1].Name, Companies[i].Name)
	}
}

func TestLoadCompaniesRejectsBadHost(t *testing.T) {
	_, err := loadCompanies([]byte(`- company: "X"
  host: "not-ultipro.example.com"
  company_code: "X1"
  board_id: "18180d88-ced0-4361-bd09-d5eef66dab24"
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "recruiting<N>.ultipro.com")
}

func TestLoadCompaniesRejectsBadBoardID(t *testing.T) {
	_, err := loadCompanies([]byte(`- company: "X"
  host: "recruiting.ultipro.com"
  company_code: "X1"
  board_id: "not-a-guid"
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "board_id")
}

func TestLoadCompaniesRejectsDuplicateCode(t *testing.T) {
	_, err := loadCompanies([]byte(`- company: "X"
  host: "recruiting.ultipro.com"
  company_code: "X1"
  board_id: "18180d88-ced0-4361-bd09-d5eef66dab24"
- company: "Y"
  host: "recruiting.ultipro.com"
  company_code: "x1"
  board_id: "26bbbd4e-7790-a471-7482-a40a1f3f8f25"
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate company_code")
}

func TestCareersURLAndBaseURL(t *testing.T) {
	c := Company{
		Name:        "TechnoServe",
		Host:        "recruiting.ultipro.com",
		CompanyCode: "TEC1006TESER",
		BoardID:     "18180d88-ced0-4361-bd09-d5eef66dab24",
	}
	assert.Equal(t, "https://recruiting.ultipro.com/TEC1006TESER/JobBoard/18180d88-ced0-4361-bd09-d5eef66dab24/", c.CareersURL())
	assert.Equal(t, "https://recruiting.ultipro.com/TEC1006TESER/JobBoard/18180d88-ced0-4361-bd09-d5eef66dab24", c.BaseURL())
}
