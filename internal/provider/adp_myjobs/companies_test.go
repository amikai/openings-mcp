package adp_myjobs

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompaniesSeed(t *testing.T) {
	require.GreaterOrEqual(t, len(Companies), 5)
	seen := map[string]bool{}
	for _, c := range Companies {
		assert.Equal(t, strings.ToLower(c.Slug), c.Slug)
		require.False(t, seen[c.Slug], "dup slug %s", c.Slug)
		seen[c.Slug] = true
		assert.NotEmpty(t, c.Name)
		assert.Contains(t, c.CareersURL(), c.Slug)
	}
	_, ok := CompaniesBySlug["guitarcenterexternal"]
	assert.True(t, ok)
}
