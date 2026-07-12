package smartrecruiters

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A duplicate companyIdentifier must resolve to the roster's first row, or
// search and detail would report different names for one identifier.
func TestBuildIdentifierIndexFirstRowWins(t *testing.T) {
	cs := []RosterCompany{
		{Name: "Acme Inc (old listing)", CompanyIdentifier: "Acme"},
		{Name: "Acme Inc (new listing)", CompanyIdentifier: "Acme"},
	}
	idx := buildIdentifierIndex(cs)
	assert.Equal(t, "Acme Inc (old listing)", idx["acme"].Name)
}

func TestCompaniesByIdentifierMatchesFirstCompanyRow(t *testing.T) {
	seen := map[string]RosterCompany{}
	for _, c := range Companies {
		slug := strings.ToLower(c.CompanyIdentifier)
		if _, ok := seen[slug]; !ok {
			seen[slug] = c
		}
	}
	require.NotEmpty(t, seen)
	for slug, want := range seen {
		assert.Equalf(t, want.Name, CompaniesByIdentifier[slug].Name, "identifier %q", slug)
	}
}

// TestNoDuplicateIdentifiers guards companies.yaml itself: the roster was
// hand-assembled from a live discovery feed, so a duplicate companyIdentifier
// (same company harvested twice under different display names) is a data
// bug, not something buildIdentifierIndex should silently paper over.
func TestNoDuplicateIdentifiers(t *testing.T) {
	seen := map[string]string{}
	for _, c := range Companies {
		slug := strings.ToLower(c.CompanyIdentifier)
		if prev, ok := seen[slug]; ok {
			t.Errorf("duplicate company_identifier %q: %q and %q", c.CompanyIdentifier, prev, c.Name)
			continue
		}
		seen[slug] = c.Name
	}
}

func TestCareersURL(t *testing.T) {
	c := RosterCompany{Name: "Equinox", CompanyIdentifier: "Equinox"}
	assert.Equal(t, "https://jobs.smartrecruiters.com/Equinox", c.CareersURL())
}
