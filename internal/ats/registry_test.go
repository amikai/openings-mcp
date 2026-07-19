package ats

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAdapter satisfies Adapter with a canned roster; search methods are
// never reached by registry tests.
type fakeAdapter struct {
	name   string
	host   string // careers host this fake claims, e.g. "jobs.fake-lever.example"
	roster []CompanyInfo
}

func (f *fakeAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	if f.host == "" || u.Hostname() != f.host {
		return "", false
	}
	slug, _, _ := strings.Cut(strings.Trim(u.Path, "/"), "/")
	return slug, slug != ""
}

func (f *fakeAdapter) Name() string          { return f.name }
func (f *fakeAdapter) Roster() []CompanyInfo { return f.roster }
func (f *fakeAdapter) Search(context.Context, string, SearchParams) (*SearchResult, error) {
	return nil, nil
}
func (f *fakeAdapter) Filters(context.Context, string) (FilterSet, error) { return nil, nil }
func (f *fakeAdapter) Detail(context.Context, string, string) (*JobDetail, error) {
	return nil, nil
}

func testRegistry(t *testing.T) *Registry {
	t.Helper()
	r, err := NewRegistry(
		&fakeAdapter{name: "workday", host: "jobs.fake-workday.example", roster: []CompanyInfo{
			{Slug: "nvidia", Name: "NVIDIA Corp"},
			{Slug: "workday", Name: "Workday, Inc."},
		}},
		&fakeAdapter{name: "lever", host: "jobs.fake-lever.example", roster: []CompanyInfo{
			{Slug: "palantir", Name: "Palantir Technologies"},
			{Slug: "nvidia-jp", Name: "NVIDIA Corp"},
		}},
	)
	require.NoError(t, err)
	return r
}

func TestResolveBySlug(t *testing.T) {
	r := testRegistry(t)
	rs, err := r.Resolve("palantir")
	require.NoError(t, err)
	require.Len(t, rs, 1)
	assert.Equal(t, "lever", rs[0].Adapter.Name())
	assert.Equal(t, "palantir", rs[0].Slug)
}

func TestResolveByDisplayName(t *testing.T) {
	r := testRegistry(t)
	// Case, punctuation, and spaces must not matter.
	for _, input := range []string{"Workday, Inc.", "workday inc"} {
		rs, err := r.Resolve(input)
		require.NoErrorf(t, err, "Resolve(%q)", input)
		require.Lenf(t, rs, 1, "Resolve(%q)", input)
		assert.Equal(t, "workday", rs[0].Slug)
	}
}

func TestResolveMultiMatch(t *testing.T) {
	r := testRegistry(t)
	// "NVIDIA Corp" is workday's name+slug key and lever's name key; both
	// entries come back, in adapter registration order.
	rs, err := r.Resolve("NVIDIA Corp")
	require.NoError(t, err)
	require.Len(t, rs, 2)
	assert.Equal(t, "workday", rs[0].Adapter.Name())
	assert.Equal(t, "nvidia", rs[0].Slug)
	assert.Equal(t, "lever", rs[1].Adapter.Name())
	assert.Equal(t, "nvidia-jp", rs[1].Slug)
}

func TestResolveSlugKeyStaysSpecific(t *testing.T) {
	r := testRegistry(t)
	// The regional slug hits only its own entry, not the shared name key.
	rs, err := r.Resolve("nvidia-jp")
	require.NoError(t, err)
	require.Len(t, rs, 1)
	assert.Equal(t, "lever", rs[0].Adapter.Name())
}

func TestResolveUnknownTeaches(t *testing.T) {
	r := testRegistry(t)
	_, err := r.Resolve("palantir tech")
	require.ErrorContains(t, err, "palantir", "suggestions should contain the input")
	assert.ErrorContains(t, err, "4 companies", "error should state supported count")
}

func TestResolveEmpty(t *testing.T) {
	r := testRegistry(t)
	_, err := r.Resolve("  ")
	assert.Error(t, err, "want error for empty company")
}

func TestNewRegistryAllowsCrossAdapterCollision(t *testing.T) {
	r, err := NewRegistry(
		&fakeAdapter{name: "workday", roster: []CompanyInfo{{Slug: "acme", Name: "Acme (Workday)"}}},
		&fakeAdapter{name: "lever", roster: []CompanyInfo{{Slug: "acme", Name: "Acme (Lever)"}}},
	)
	require.NoError(t, err, "cross-adapter slug collision must not fail startup")
	rs, err := r.Resolve("acme")
	require.NoError(t, err)
	assert.Len(t, rs, 2)
}

func TestNewRegistryRejectsDuplicateSlugWithinAdapter(t *testing.T) {
	_, err := NewRegistry(
		&fakeAdapter{name: "workday", roster: []CompanyInfo{
			{Slug: "acme", Name: "Acme"},
			{Slug: "Acme", Name: "Acme Holdings"},
		}},
	)
	assert.Error(t, err, "want error for duplicate slug within one adapter")
}

func TestResolveCareersURL(t *testing.T) {
	r := testRegistry(t)
	rs, err := r.Resolve("https://jobs.fake-lever.example/somestartup")
	require.NoError(t, err)
	require.Len(t, rs, 1)
	assert.Equal(t, "lever", rs[0].Adapter.Name())
	assert.Equal(t, "somestartup", rs[0].Slug)
}

func TestResolveCareersURLSchemeless(t *testing.T) {
	r := testRegistry(t)
	rs, err := r.Resolve("jobs.fake-workday.example/acme")
	require.NoError(t, err)
	require.Len(t, rs, 1)
	assert.Equal(t, "workday", rs[0].Adapter.Name())
	assert.Equal(t, "acme", rs[0].Slug)
}

func TestResolveUnrecognizedCareersURLTeaches(t *testing.T) {
	r := testRegistry(t)
	_, err := r.Resolve("https://careers.example.com/acme")
	require.ErrorContains(t, err, "careers URL", "URL misses should get the URL error, not name suggestions")
	assert.NotContains(t, err.Error(), "closest matches", "no levenshtein suggestions for URLs")
}
