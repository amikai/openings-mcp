package lever

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompanies(t *testing.T) {
	require.NotEmpty(t, Companies)
	assert.True(t, sort.SliceIsSorted(Companies, func(i, j int) bool {
		return Companies[i].Name < Companies[j].Name
	}), "Companies must be sorted by name")

	seen := make(map[string]bool, len(Companies))
	for _, c := range Companies {
		assert.NotEmpty(t, c.Name)
		assert.NotEmpty(t, c.Site)
		assert.Equal(t, strings.ToLower(c.Site), c.Site, "site slugs are lowercase")
		assert.False(t, seen[c.Site], "duplicate site %q", c.Site)
		seen[c.Site] = true
	}

	demo, ok := CompaniesBySite["leverdemo"]
	require.True(t, ok)
	assert.Equal(t, "Lever (demo)", demo.Name)
	assert.Len(t, CompaniesBySite, len(Companies))
}
