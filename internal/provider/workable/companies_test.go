package workable

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A duplicate account must resolve to the roster's first row, or search and
// detail would report different names for one account.
func TestBuildAccountIndexFirstRowWins(t *testing.T) {
	cs := []RosterCompany{
		{Name: "Acme Inc (old listing)", Account: "acme"},
		{Name: "Acme Inc (new listing)", Account: "acme"},
	}
	idx := buildAccountIndex(cs)
	assert.Equal(t, "Acme Inc (old listing)", idx["acme"].Name)
}

func TestCompaniesByAccountMatchesFirstCompanyRow(t *testing.T) {
	seen := map[string]RosterCompany{}
	for _, c := range Companies {
		slug := strings.ToLower(c.Account)
		if _, ok := seen[slug]; !ok {
			seen[slug] = c
		}
	}
	require.NotEmpty(t, seen)
	for slug, want := range seen {
		assert.Equalf(t, want.Name, CompaniesByAccount[slug].Name, "account %q", slug)
	}
}

// TestNoDuplicateAccounts guards companies.yaml itself: a duplicate account
// (same company added twice under different display names) is a data bug,
// not something buildAccountIndex should silently paper over.
func TestNoDuplicateAccounts(t *testing.T) {
	seen := map[string]string{}
	for _, c := range Companies {
		slug := strings.ToLower(c.Account)
		if prev, ok := seen[slug]; ok {
			t.Errorf("account %q appears twice: %q and %q", slug, prev, c.Name)
			continue
		}
		seen[slug] = c.Name
	}
}
