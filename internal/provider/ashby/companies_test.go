package ashby

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompanies(t *testing.T) {
	assert.NotEmpty(t, Companies)

	seen := make(map[string]string, len(Companies))
	for _, c := range Companies {
		assert.NotEmpty(t, c.Name)
		assert.NotEmpty(t, c.Board)
		if prev, dup := seen[c.Board]; dup {
			t.Errorf("duplicate board %q used by %q and %q (CompaniesByBoard silently drops one)", c.Board, prev, c.Name)
		}
		seen[c.Board] = c.Name
	}

	assert.Equal(t, "https://jobs.ashbyhq.com/openai", CompaniesByBoard["openai"].BoardURL())
}
