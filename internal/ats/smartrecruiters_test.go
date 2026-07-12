package ats

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/smartrecruiters"
)

func TestSmartRecruitersRosterMirrorsProviderRoster(t *testing.T) {
	a, err := NewSmartRecruitersAdapter("https://api.smartrecruiters.com", http.DefaultClient)
	require.NoError(t, err)
	roster := a.Roster()
	require.Len(t, roster, len(smartrecruiters.Companies))
	seen := map[string]bool{}
	for _, c := range roster {
		assert.Equal(t, strings.ToLower(c.Slug), c.Slug, "slug %q must be lowercase", c.Slug)
		require.Falsef(t, seen[c.Slug], "duplicate slug %q in roster", c.Slug)
		seen[c.Slug] = true
	}
	assert.True(t, seen["equinox"], "expected equinox in roster")
}

func TestSmartRecruitersRosterBuildsRegistry(t *testing.T) {
	a, err := NewSmartRecruitersAdapter("https://api.smartrecruiters.com", http.DefaultClient)
	require.NoError(t, err)
	_, err = NewRegistry(a)
	require.NoError(t, err)
}

func TestSmartRecruitersParseCareersURL(t *testing.T) {
	a, err := NewSmartRecruitersAdapter("https://api.smartrecruiters.com", http.DefaultClient)
	require.NoError(t, err)
	tests := []struct {
		name string
		url  string
		slug string
		ok   bool
	}{
		{"roster company", "https://jobs.smartrecruiters.com/Equinox", "equinox", true},
		{"posting page", "https://jobs.smartrecruiters.com/Equinox/744000137225639-female-locker-room-associate-houston", "equinox", true},
		{"non-roster company", "https://jobs.smartrecruiters.com/SomeUnknownCo", "someunknownco", true},
		{"host only", "https://jobs.smartrecruiters.com/", "", false},
		{"other ats", "https://jobs.lever.co/acme", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.url)
			require.NoError(t, err)
			slug, ok := a.ParseCareersURL(u)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.slug, slug)
		})
	}
}
