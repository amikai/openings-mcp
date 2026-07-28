package engage

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNoDuplicateSlugs guards companies.yaml itself: a duplicate slug (same
// company added twice) is a data bug, not something buildSlugIndex should
// silently paper over.
func TestNoDuplicateSlugs(t *testing.T) {
	seen := map[string]string{}
	for _, c := range Companies {
		slug := strings.ToLower(c.Slug)
		if prev, ok := seen[slug]; ok {
			t.Errorf("duplicate slug %q: %q and %q", c.Slug, prev, c.Name)
			continue
		}
		seen[slug] = c.Name
	}
}

func TestCompaniesLoaded(t *testing.T) {
	require.NotEmpty(t, Companies)
	for _, c := range Companies {
		assert.NotEmpty(t, c.Name, "company missing name")
		assert.NotEmpty(t, c.Slug, "company %q missing slug", c.Name)
	}
}

func TestBuildSlugIndex(t *testing.T) {
	for _, c := range Companies {
		got, ok := CompaniesBySlug[c.Slug]
		require.True(t, ok, "slug %q missing from index", c.Slug)
		assert.Equal(t, c.Name, got.Name)
	}
}

func TestCareersURL(t *testing.T) {
	c := Company{Name: "Example", Slug: "example_jobs"}
	assert.Equal(t, "https://en-gage.net/example_jobs/", c.CareersURL())
}

func TestLoadCompaniesRejectsDuplicateSlug(t *testing.T) {
	data := []byte(`
- company: "Acme Inc"
  slug: "acme"
- company: "Acme Inc Two"
  slug: "acme"
`)
	_, err := loadCompanies(data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate slug")
}

func TestLoadCompaniesRejectsDuplicateNameCaseInsensitive(t *testing.T) {
	data := []byte(`
- company: "Acme Inc"
  slug: "acme"
- company: "ACME INC"
  slug: "acme-two"
`)
	_, err := loadCompanies(data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate company name")
}

func TestLoadCompaniesRejectsInvalidSlug(t *testing.T) {
	data := []byte(`
- company: "Acme Inc"
  slug: "Acme_Inc"
`)
	_, err := loadCompanies(data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid slug")
}
