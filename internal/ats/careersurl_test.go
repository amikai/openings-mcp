package ats

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}

func TestParseCareersInput(t *testing.T) {
	tests := []struct {
		in       string
		ok       bool
		wantHost string
	}{
		{in: "https://jobs.lever.co/acme", ok: true, wantHost: "jobs.lever.co"},
		{in: "jobs.lever.co/acme", ok: true, wantHost: "jobs.lever.co"}, // scheme-less
		{in: "  https://jobs.ashbyhq.com/acme  ", ok: true, wantHost: "jobs.ashbyhq.com"},
		{in: "nvidia", ok: false},          // plain name
		{in: "NVIDIA Corp", ok: false},     // display name
		{in: "acme.io", ok: false},         // dot but no path
		{in: "ftp://x.co/acme", ok: false}, // non-http scheme
		{in: "", ok: false},
	}
	for _, tt := range tests {
		u, ok := parseCareersInput(tt.in)
		require.Equalf(t, tt.ok, ok, "parseCareersInput(%q)", tt.in)
		if ok {
			assert.Equal(t, tt.wantHost, u.Hostname())
		}
	}
}

func TestSlugAdaptersParseCareersURL(t *testing.T) {
	gh, err := NewGreenhouseAdapter("https://example.invalid", http.DefaultClient)
	require.NoError(t, err)
	lv, err := NewLeverAdapter("https://example.invalid", http.DefaultClient)
	require.NoError(t, err)
	ab, err := NewAshbyAdapter("https://example.invalid", http.DefaultClient)
	require.NoError(t, err)

	tests := []struct {
		name    string
		adapter interface {
			ParseCareersURL(*url.URL) (string, bool)
		}
		raw  string
		slug string
		ok   bool
	}{
		{name: "greenhouse job-boards", adapter: gh, raw: "https://job-boards.greenhouse.io/acme", slug: "acme", ok: true},
		{name: "greenhouse boards legacy", adapter: gh, raw: "https://boards.greenhouse.io/acme/jobs/123", slug: "acme", ok: true},
		{name: "greenhouse eu", adapter: gh, raw: "https://job-boards.eu.greenhouse.io/acme", slug: "acme", ok: true},
		{name: "greenhouse wrong host", adapter: gh, raw: "https://jobs.lever.co/acme", ok: false},
		{name: "greenhouse empty path", adapter: gh, raw: "https://job-boards.greenhouse.io/", ok: false},
		{name: "lever", adapter: lv, raw: "https://jobs.lever.co/acme", slug: "acme", ok: true},
		{name: "lever eu", adapter: lv, raw: "https://jobs.eu.lever.co/acme", slug: "acme", ok: true},
		{name: "lever deep link", adapter: lv, raw: "https://jobs.lever.co/acme/00000000-0000", slug: "acme", ok: true},
		{name: "lever wrong host", adapter: lv, raw: "https://job-boards.greenhouse.io/acme", ok: false},
		{name: "ashby", adapter: ab, raw: "https://jobs.ashbyhq.com/acme", slug: "acme", ok: true},
		{name: "ashby url-encoded org", adapter: ab, raw: "https://jobs.ashbyhq.com/Acme%20Inc", slug: "Acme Inc", ok: true},
		{name: "ashby wrong host", adapter: ab, raw: "https://jobs.lever.co/acme", ok: false},
		// Empty path segments are rejected; a double slash is not a real
		// careers URL shape.
		{name: "lever double slash rejected", adapter: lv, raw: "https://jobs.lever.co//acme", ok: false},
		{name: "ashby double slash rejected", adapter: ab, raw: "https://jobs.ashbyhq.com//acme", ok: false},
		{name: "greenhouse double slash rejected", adapter: gh, raw: "https://job-boards.greenhouse.io//acme", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slug, ok := tt.adapter.ParseCareersURL(mustParseURL(t, tt.raw))
			require.Equal(t, tt.ok, ok)
			if ok {
				assert.Equal(t, tt.slug, slug)
			}
		})
	}
}
