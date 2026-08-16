package dayforce

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
		assert.NotEmpty(t, c.Namespace)
		assert.NotEmpty(t, c.JobBoardCode)
		assert.Positive(t, c.JobBoardID)
		if c.CultureCode == "" {
			assert.Equal(t, _defaultCultureCode, c.Culture())
		} else {
			assert.Equal(t, c.CultureCode, c.Culture())
		}
		got, ok := CompaniesBySlug[strings.ToLower(c.Slug())]
		require.True(t, ok, "slug index missing %q", c.Slug())
		assert.Equal(t, c, got)
	}
	// Sorted by name.
	for i := 1; i < len(Companies); i++ {
		assert.LessOrEqual(t, Companies[i-1].Name, Companies[i].Name)
	}
}

func TestLoadCompaniesRejectsMissingNamespace(t *testing.T) {
	_, err := loadCompanies([]byte(`- company: "X"
  namespace: ""
  job_board_code: "CANDIDATEPORTAL"
  job_board_id: 1
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "namespace is required")
}

func TestLoadCompaniesRejectsMissingJobBoardCode(t *testing.T) {
	_, err := loadCompanies([]byte(`- company: "X"
  namespace: "x"
  job_board_code: ""
  job_board_id: 1
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "job_board_code is required")
}

func TestLoadCompaniesRejectsBadJobBoardID(t *testing.T) {
	_, err := loadCompanies([]byte(`- company: "X"
  namespace: "x"
  job_board_code: "CANDIDATEPORTAL"
  job_board_id: 0
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "job_board_id must be > 0")
}

func TestLoadCompaniesRejectsDuplicateSlug(t *testing.T) {
	_, err := loadCompanies([]byte(`- company: "X"
  namespace: "dup"
  job_board_code: "CANDIDATEPORTAL"
  job_board_id: 1
- company: "Y"
  namespace: "DUP"
  job_board_code: "CANDIDATEPORTAL"
  job_board_id: 1
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate slug")
}

func TestLoadCompaniesRejectsDuplicateName(t *testing.T) {
	_, err := loadCompanies([]byte(`- company: "X"
  namespace: "a"
  job_board_code: "CANDIDATEPORTAL"
  job_board_id: 1
- company: "x"
  namespace: "b"
  job_board_code: "CANDIDATEPORTAL"
  job_board_id: 1
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate company name")
}

func TestLoadCompaniesRejectsDuplicateNameAcrossPunctuation(t *testing.T) {
	_, err := loadCompanies([]byte(`- company: "Foo Inc."
  namespace: "a"
  job_board_code: "CANDIDATEPORTAL"
  job_board_id: 1
- company: "foo inc"
  namespace: "b"
  job_board_code: "CANDIDATEPORTAL"
  job_board_id: 1
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate company name")
}

func TestSlugSuffixesNonDefaultBoard(t *testing.T) {
	c := Company{Name: "X", Namespace: "mydayforce", JobBoardCode: "ALLJOBS", JobBoardID: 8}
	assert.Equal(t, "mydayforce-alljobs", c.Slug())

	c.JobBoardCode = "CANDIDATEPORTAL"
	assert.Equal(t, "mydayforce", c.Slug())
}

func TestCultureDefault(t *testing.T) {
	c := Company{Name: "X", Namespace: "x", JobBoardCode: "CANDIDATEPORTAL", JobBoardID: 1}
	assert.Equal(t, "en-US", c.Culture())

	c.CultureCode = "fr-CA"
	assert.Equal(t, "fr-CA", c.Culture())
}

func TestRosterPinsNonDefaultCulture(t *testing.T) {
	want := map[string]string{
		"ausredcross":  "en-AU",
		"cht":          "en-NZ",
		"cmhr":         "en-CA",
		"costa":        "en-GB",
		"eldoradogold": "fr-CA",
	}
	for slug, culture := range want {
		c, ok := CompaniesBySlug[slug]
		require.True(t, ok, "missing roster slug %q", slug)
		assert.Equal(t, culture, c.CultureCode)
		assert.Equal(t, culture, c.Culture())
	}
}
