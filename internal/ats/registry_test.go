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
func (f *fakeAdapter) CareersURL(slug string) (string, bool) {
	if f.host == "" {
		return "", false
	}
	return "https://" + f.host + "/" + slug, true
}
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
		}},
	)
	require.NoError(t, err)
	return r
}

func TestResolveBySlug(t *testing.T) {
	r := testRegistry(t)
	resolution, err := r.Resolve("nvidia")
	require.NoError(t, err)
	a, slug, ok := resolution.Single()
	require.True(t, ok)
	assert.Equal(t, "workday", a.Name())
	assert.Equal(t, "nvidia", slug)
}

func TestCompanyResolutionSingleRejectsNonUniqueResults(t *testing.T) {
	_, _, ok := (CompanyResolution{}).Single()
	assert.False(t, ok)

	r, err := NewRegistry(
		&fakeAdapter{name: "herp", roster: []CompanyInfo{{Slug: "nature", Name: "Nature KK"}}},
		&fakeAdapter{name: "workday", roster: []CompanyInfo{{Slug: "nature", Name: "Nature Research"}}},
	)
	require.NoError(t, err)
	resolution, err := r.Resolve("nature")
	require.NoError(t, err)

	_, _, ok = resolution.Single()
	assert.False(t, ok)
}

func TestCompanyResolutionSelectRejectsInvalidIndex(t *testing.T) {
	resolution, err := testRegistry(t).Resolve("nvidia")
	require.NoError(t, err)

	_, _, ok := resolution.Select(-1)
	assert.False(t, ok)
	_, _, ok = resolution.Select(1)
	assert.False(t, ok)
}

func TestResolveByDisplayName(t *testing.T) {
	r := testRegistry(t)
	// Case, punctuation, and spaces must not matter.
	for _, input := range []string{"NVIDIA Corp", "nvidia corp", "Workday, Inc.", "workday inc"} {
		_, err := r.Resolve(input)
		assert.NoErrorf(t, err, "Resolve(%q)", input)
	}
}

func TestResolveUnknownTeaches(t *testing.T) {
	r := testRegistry(t)
	_, err := r.Resolve("palantir tech")
	require.ErrorContains(t, err, "palantir", "suggestions should contain the input")
	assert.ErrorContains(t, err, "3 companies", "error should state supported count")
}

func TestResolveEmpty(t *testing.T) {
	r := testRegistry(t)
	_, err := r.Resolve("  ")
	assert.Error(t, err, "want error for empty company")
}

func TestNewRegistryRejectsDuplicateSlug(t *testing.T) {
	_, err := NewRegistry(
		&fakeAdapter{name: "workday", roster: []CompanyInfo{
			{Slug: "acme", Name: "Acme (First)"},
			{Slug: "acme", Name: "Acme (Second)"},
		}},
	)
	assert.Error(t, err, "want error for duplicate slug within the same adapter's roster")
}

func TestNewRegistryRejectsDuplicateName(t *testing.T) {
	_, err := NewRegistry(
		&fakeAdapter{name: "workday", roster: []CompanyInfo{
			{Slug: "acme-1", Name: "Acme"},
			{Slug: "acme-2", Name: "Acme"},
		}},
	)
	assert.Error(t, err, "want error for duplicate name within the same adapter's roster")
}

func TestNewRegistryAllowsCrossAdapterDuplicateSlug(t *testing.T) {
	_, err := NewRegistry(
		&fakeAdapter{name: "workday", roster: []CompanyInfo{{Slug: "acme", Name: "Acme (Workday)"}}},
		&fakeAdapter{name: "lever", roster: []CompanyInfo{{Slug: "acme", Name: "Acme (Lever)"}}},
	)
	assert.NoError(t, err, "cross-adapter slug collisions are not fatal")
}

func TestNewRegistryAllowsCrossAdapterDuplicateName(t *testing.T) {
	_, err := NewRegistry(
		&fakeAdapter{name: "workday", host: "workday.example", roster: []CompanyInfo{{Slug: "acme-workday", Name: "Acme"}}},
		&fakeAdapter{name: "lever", host: "lever.example", roster: []CompanyInfo{{Slug: "acme-lever", Name: "Acme"}}},
	)
	assert.NoError(t, err, "cross-adapter name collisions are not fatal")
}

func TestResolveCareersURL(t *testing.T) {
	r := testRegistry(t)
	resolution, err := r.Resolve("https://jobs.fake-lever.example/somestartup")
	require.NoError(t, err)
	a, slug, ok := resolution.Single()
	require.True(t, ok)
	assert.Equal(t, "lever", a.Name())
	assert.Equal(t, "somestartup", slug)
}

func TestResolveCareersURLSchemeless(t *testing.T) {
	r := testRegistry(t)
	resolution, err := r.Resolve("jobs.fake-workday.example/acme")
	require.NoError(t, err)
	a, slug, ok := resolution.Single()
	require.True(t, ok)
	assert.Equal(t, "workday", a.Name())
	assert.Equal(t, "acme", slug)
}

func TestResolveUnrecognizedCareersURLTeaches(t *testing.T) {
	r := testRegistry(t)
	_, err := r.Resolve("https://careers.example.com/acme")
	require.ErrorContains(t, err, "careers URL", "URL misses should get the URL error, not name suggestions")
	assert.NotContains(t, err.Error(), "closest matches", "no levenshtein suggestions for URLs")
}

func TestResolveAmbiguousSlug(t *testing.T) {
	r, err := NewRegistry(
		&fakeAdapter{name: "herp", host: "herp.example", roster: []CompanyInfo{{Slug: "nature", Name: "Nature KK"}}},
		&fakeAdapter{name: "workday", host: "workday.example", roster: []CompanyInfo{{Slug: "nature", Name: "Nature Research"}}},
	)
	require.NoError(t, err)

	resolution, err := r.Resolve("nature")
	require.NoError(t, err)
	require.True(t, resolution.IsAmbiguous())
	candidates := resolution.Candidates()
	require.Len(t, candidates, 2)
	var names []string
	for _, c := range candidates {
		names = append(names, c.Name)
		assert.NotEmpty(t, c.CareersURL, "both candidates render a careers URL")
	}
	assert.ElementsMatch(t, []string{"Nature KK", "Nature Research"}, names)

	adapter, slug, ok := resolution.Select(1)
	require.True(t, ok)
	assert.Equal(t, "workday", adapter.Name())
	assert.Equal(t, "nature", slug)
}

func TestResolveAmbiguousName(t *testing.T) {
	r, err := NewRegistry(
		&fakeAdapter{name: "greenhouse", host: "greenhouse.example", roster: []CompanyInfo{{Slug: "beta-gh", Name: "BETA Technologies"}}},
		&fakeAdapter{name: "lever", host: "lever.example", roster: []CompanyInfo{{Slug: "beta-lv", Name: "BETA Technologies"}}},
	)
	require.NoError(t, err)

	resolution, err := r.Resolve("BETA Technologies")
	require.NoError(t, err)
	assert.True(t, resolution.IsAmbiguous())
	assert.Len(t, resolution.Candidates(), 2)
}

// TestResolveAmbiguousSlugVersusName covers the case the options survey
// missed: one entry's slug and a different entry's name normalize to the
// same key. The union must list both rather than the slug index silently
// shadowing the name hit.
func TestResolveAmbiguousSlugVersusName(t *testing.T) {
	r, err := NewRegistry(
		&fakeAdapter{name: "herp", host: "herp.example", roster: []CompanyInfo{{Slug: "fusion", Name: "株式会社FUSION"}}},
		&fakeAdapter{name: "workday", host: "workday.example", roster: []CompanyInfo{{Slug: "fusion-wd", Name: "Fusion"}}},
	)
	require.NoError(t, err)

	resolution, err := r.Resolve("fusion")
	require.NoError(t, err)
	assert.True(t, resolution.IsAmbiguous())
	assert.Len(t, resolution.Candidates(), 2)
}

// TestResolveCareersURLWinsOverAmbiguousToken checks that a careers URL is
// authoritative even when its bare slug alone would be ambiguous.
func TestResolveCareersURLWinsOverAmbiguousToken(t *testing.T) {
	r, err := NewRegistry(
		&fakeAdapter{name: "herp", host: "herp.example", roster: []CompanyInfo{{Slug: "acme", Name: "Acme HERP"}}},
		&fakeAdapter{name: "workday", host: "workday.example", roster: []CompanyInfo{{Slug: "acme", Name: "Acme Workday"}}},
	)
	require.NoError(t, err)

	resolution, err := r.Resolve("https://workday.example/acme")
	require.NoError(t, err)
	a, slug, ok := resolution.Single()
	require.True(t, ok)
	assert.Equal(t, "workday", a.Name())
	assert.Equal(t, "acme", slug)
}

// TestResolveAmbiguousCandidateWithoutCareersURL covers a candidate whose
// adapter has no public careers URL. Its human-readable choice still carries
// the display name, and selecting it returns the original registry entry.
func TestResolveAmbiguousCandidateWithoutCareersURL(t *testing.T) {
	r, err := NewRegistry(
		&fakeAdapter{name: "roster-only", roster: []CompanyInfo{{Slug: "nature", Name: "Foo Corp"}}},
		&fakeAdapter{name: "workday", host: "workday.example", roster: []CompanyInfo{{Slug: "nature", Name: "Nature Research"}}},
	)
	require.NoError(t, err)

	resolution, err := r.Resolve("nature")
	require.NoError(t, err)
	require.True(t, resolution.IsAmbiguous())
	candidates := resolution.Candidates()
	require.Len(t, candidates, 2)
	assert.Equal(t, "Foo Corp", candidates[0].Name)
	assert.Empty(t, candidates[0].CareersURL)

	a, slug, ok := resolution.Select(0)
	require.True(t, ok)
	assert.Equal(t, "roster-only", a.Name())
	assert.Equal(t, "nature", slug)
}

// TestResolveSlugAndNameSameKeyIsOneCandidate ensures an entry whose slug
// and name both normalize to the same key (e.g. "bunq"/"bunq") does not
// become ambiguous with itself: it appears in both bySlug and byName for
// that key, and the union must collapse it to one candidate.
func TestResolveSlugAndNameSameKeyIsOneCandidate(t *testing.T) {
	r, err := NewRegistry(
		&fakeAdapter{name: "workday", host: "workday.example", roster: []CompanyInfo{{Slug: "bunq", Name: "bunq"}}},
	)
	require.NoError(t, err)
	resolution, err := r.Resolve("bunq")
	require.NoError(t, err)
	a, slug, ok := resolution.Single()
	require.True(t, ok)
	assert.Equal(t, "workday", a.Name())
	assert.Equal(t, "bunq", slug)
}

func TestNewRegistryRejectsNoURLEntryNameCollidingWithName(t *testing.T) {
	_, err := NewRegistry(
		&fakeAdapter{name: "roster-only", roster: []CompanyInfo{{Slug: "foo", Name: "Acme"}}},
		&fakeAdapter{name: "workday", host: "workday.example", roster: []CompanyInfo{{Slug: "acme-wd", Name: "Acme"}}},
	)
	assert.Error(t, err, "an ok=false entry's name must not collide with another entry's name")
}

func TestNewRegistryRejectsNoURLEntryNameCollidingWithSlug(t *testing.T) {
	_, err := NewRegistry(
		&fakeAdapter{name: "roster-only", roster: []CompanyInfo{{Slug: "foo", Name: "Fusion"}}},
		&fakeAdapter{name: "workday", host: "workday.example", roster: []CompanyInfo{{Slug: "fusion", Name: "Something Else"}}},
	)
	assert.Error(t, err, "an ok=false entry's name must not collide with another entry's slug")
}

// TestResolveURLShapedRosterSlugFallsThroughToUnion covers Oracle- and
// Avature-style roster slugs that are themselves careers-URL shaped but
// unparseable by their own adapter's ParseCareersURL. The ①→② fallthrough
// must still resolve them through the slug index.
func TestResolveURLShapedRosterSlugFallsThroughToUnion(t *testing.T) {
	r, err := NewRegistry(
		&fakeAdapter{name: "oracle", roster: []CompanyInfo{{Slug: "abc.fa.us2.oraclecloud.com/CX_1", Name: "Acme Health"}}},
	)
	require.NoError(t, err)
	resolution, err := r.Resolve("abc.fa.us2.oraclecloud.com/CX_1")
	require.NoError(t, err)
	a, slug, ok := resolution.Single()
	require.True(t, ok)
	assert.Equal(t, "oracle", a.Name())
	assert.Equal(t, "abc.fa.us2.oraclecloud.com/CX_1", slug)
}

// TestSuggestDedupesCollidingSlug ensures a miss whose nearest slug is a
// cross-adapter collision does not list that slug twice.
func TestSuggestDedupesCollidingSlug(t *testing.T) {
	r, err := NewRegistry(
		&fakeAdapter{name: "herp", roster: []CompanyInfo{{Slug: "nature", Name: "Nature KK"}}},
		&fakeAdapter{name: "workday", roster: []CompanyInfo{{Slug: "nature", Name: "Nature Research"}}},
		&fakeAdapter{name: "lever", roster: []CompanyInfo{{Slug: "other", Name: "Other Co"}}},
	)
	require.NoError(t, err)
	_, err = r.Resolve("natur")
	require.Error(t, err)
	assert.Equal(t, 1, strings.Count(err.Error(), "nature"), "colliding slug must be suggested once, not once per adapter")
}

// TestMissErrorCountsRosterEntriesNotKeys ensures the advertised company
// count is roster rows, not distinct normalized keys — those diverge once
// a cross-adapter collision shares a key.
func TestMissErrorCountsRosterEntriesNotKeys(t *testing.T) {
	r, err := NewRegistry(
		&fakeAdapter{name: "herp", roster: []CompanyInfo{{Slug: "nature", Name: "Nature KK"}}},
		&fakeAdapter{name: "workday", roster: []CompanyInfo{{Slug: "nature", Name: "Nature Research"}}},
	)
	require.NoError(t, err)
	_, err = r.Resolve("zzz-totally-unknown")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "2 companies are supported")
}
