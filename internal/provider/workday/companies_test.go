package workday

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Duplicate share-class rows (fox, dowjones) must resolve to the same row the
// roster keeps — the first one — or search and detail report different names
// for one tenant.
func TestBuildTenantIndexFirstRowWins(t *testing.T) {
	cs := []Company{
		{Name: "Fox Corporation (Class A)", Tenant: "fox", Instance: "wd1", Site: "Domestic"},
		{Name: "Fox Corporation (Class B)", Tenant: "fox", Instance: "wd1", Site: "Domestic"},
	}
	idx := buildTenantIndex(cs)
	assert.Equal(t, "Fox Corporation (Class A)", idx["fox"].Name)
}

func TestCompaniesByTenantMatchesFirstCompanyRow(t *testing.T) {
	seen := make(map[string]Company)
	for _, c := range Companies {
		slug := strings.ToLower(c.Tenant)
		if _, ok := seen[slug]; !ok {
			seen[slug] = c
		}
	}
	require.NotEmpty(t, seen)
	for slug, want := range seen {
		assert.Equalf(t, want.Name, CompaniesByTenant[slug].Name, "tenant %q", slug)
	}
}
