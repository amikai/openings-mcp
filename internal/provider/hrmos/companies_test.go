package hrmos

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A duplicate slug must resolve to the roster's first row, or search and
// detail would report different names for one slug.
func TestBuildSlugIndexFirstRowWins(t *testing.T) {
	cs := []RosterCompany{
		{Name: "Acme Inc (old listing)", Slug: "acme"},
		{Name: "Acme Inc (new listing)", Slug: "acme"},
	}
	idx := buildSlugIndex(cs)
	assert.Equal(t, "Acme Inc (old listing)", idx["acme"].Name)
}

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
		assert.NotEmpty(t, c.Slug, "company %q missing slug", c.Name)
		assert.NotEmpty(t, c.Name, "slug %q missing company name", c.Slug)
	}
}

func TestCareersURL(t *testing.T) {
	c := RosterCompany{Name: "株式会社マネーフォワード", Slug: "moneyforward"}
	assert.Equal(t, "https://hrmos.co/pages/moneyforward/jobs", c.CareersURL())
}
