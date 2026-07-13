package eightfold

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A duplicate tenant slug must resolve to the roster's first row, or search
// and detail would report different names for one tenant.
func TestBuildTenantIndexFirstRowWins(t *testing.T) {
	cs := []RosterCompany{
		{Name: "Acme Inc (old listing)", Tenant: "acme"},
		{Name: "Acme Inc (new listing)", Tenant: "acme"},
	}
	idx := buildTenantIndex(cs)
	assert.Equal(t, "Acme Inc (old listing)", idx["acme"].Name)
}

func TestCompaniesByTenantMatchesFirstCompanyRow(t *testing.T) {
	seen := map[string]RosterCompany{}
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

// TestNoDuplicateTenants guards companies.yaml itself: a duplicate tenant
// slug (same company listed twice) is a data bug, not something
// buildTenantIndex should silently paper over.
func TestNoDuplicateTenants(t *testing.T) {
	seen := map[string]string{}
	for _, c := range Companies {
		slug := strings.ToLower(c.Tenant)
		if prev, ok := seen[slug]; ok {
			t.Errorf("duplicate tenant %q: %q and %q", c.Tenant, prev, c.Name)
			continue
		}
		seen[slug] = c.Name
	}
}

func TestCareersURL(t *testing.T) {
	c := RosterCompany{Name: "Morgan Stanley", Tenant: "morganstanley"}
	assert.Equal(t, "https://morganstanley.eightfold.ai/careers", c.CareersURL())
}
