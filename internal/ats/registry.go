package ats

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"
)

// registryEntry binds one resolved company to the adapter that serves it.
type registryEntry struct {
	adapter Adapter
	slug    string
	name    string
}

// slugEntry pairs a roster slug with its precomputed normalized form, so
// the suggestion path doesn't re-normalize the whole roster on every miss.
type slugEntry struct {
	slug string
	norm string
}

// Registry is the read-only union of every adapter's roster, built once at
// startup. It owns name resolution; adapters never see unresolved input.
type Registry struct {
	adapters []Adapter                // registration order; polled for careers-URL input
	bySlug   map[string]registryEntry // key: normalize(slug)
	byName   map[string]registryEntry // key: normalize(display name)
	slugs    []slugEntry              // sorted by slug, for suggestions
}

// NewRegistry unions the adapters' rosters. A slug or normalized display
// name colliding across entries is a curation bug — fail startup loudly
// rather than silently shadowing one company with another.
func NewRegistry(adapters ...Adapter) (*Registry, error) {
	r := &Registry{
		adapters: adapters,
		bySlug:   make(map[string]registryEntry),
		byName:   make(map[string]registryEntry),
	}
	for _, a := range adapters {
		for _, c := range a.Roster() {
			e := registryEntry{adapter: a, slug: c.Slug, name: c.Name}
			slugKey := normalize(c.Slug)
			if prev, ok := r.bySlug[slugKey]; ok {
				return nil, fmt.Errorf("ats: company slug %q from %s collides with %q from %s",
					c.Slug, a.Name(), prev.slug, prev.adapter.Name())
			}
			r.bySlug[slugKey] = e
			nameKey := normalize(c.Name)
			if prev, ok := r.byName[nameKey]; ok {
				return nil, fmt.Errorf("ats: company name %q from %s collides with %q from %s",
					c.Name, a.Name(), prev.name, prev.adapter.Name())
			}
			r.byName[nameKey] = e
			r.slugs = append(r.slugs, slugEntry{slug: c.Slug, norm: slugKey})
		}
	}
	slices.SortFunc(r.slugs, func(a, b slugEntry) int { return strings.Compare(a.slug, b.slug) })
	return r, nil
}

// Resolve maps a user-supplied company string to (adapter, slug). The input can
// be a roster slug, a display name, or a careers URL. The returned slug is not
// always a roster key: a careers URL for a company outside the roster resolves
// to whatever slug the owning adapter minted via [Adapter.ParseCareersURL]
// (Workday mints the canonical careers URL). Misses return a teaching error
// carrying the closest slugs, so one retry from the LLM almost always lands.
func (r *Registry) Resolve(company string) (Adapter, string, error) {
	key := normalize(company)
	if key == "" {
		return nil, "", errors.New("company is required")
	}
	// 1. match slug
	if e, ok := r.bySlug[key]; ok {
		return e.adapter, e.slug, nil
	}
	// 2. match company name
	if e, ok := r.byName[key]; ok {
		return e.adapter, e.slug, nil
	}
	// 3. fallback to careers URL to match url slug
	if u, ok := parseCareersInput(company); ok {
		for _, a := range r.adapters {
			if slug, ok := a.ParseCareersURL(u); ok {
				return a, slug, nil
			}
		}
		return nil, "", fmt.Errorf("unrecognized careers URL %q; supported careers-page hosts: %s", company, strings.Join(r.careersHostPatterns(), ", "))
	}
	return nil, "", fmt.Errorf("unknown company %q; closest matches: %s. %d companies are supported — pass one of the suggested slugs",
		company, strings.Join(r.suggest(key, 3), ", "), len(r.bySlug))
}

// careersHostPatternsByAdapter maps each known adapter name to the
// careers-page URL shape it recognizes, so the "unrecognized careers URL"
// error only advertises hosts the registry actually has an adapter for.
var careersHostPatternsByAdapter = map[string]string{
	"workday":    "<tenant>.<wd*>.myworkdayjobs.com/<site>",
	"greenhouse": "job-boards.greenhouse.io/<board>",
	"lever":      "jobs.lever.co/<org>",
	"ashby":      "jobs.ashbyhq.com/<org>",
	"teamtailor": "<company>[.na|.au].teamtailor.com/jobs",
	"recruitee":  "<company>.recruitee.com",
}

// careersHostPatterns lists the careers-page URL shapes for r's registered
// adapters, in registration order.
func (r *Registry) careersHostPatterns() []string {
	patterns := make([]string, 0, len(r.adapters))
	for _, a := range r.adapters {
		if p, ok := careersHostPatternsByAdapter[a.Name()]; ok {
			patterns = append(patterns, p)
		}
	}
	return patterns
}

// normalize folds case and strips everything but letters and digits, so
// "Workday, Inc." and "workday inc" collide on purpose.
func normalize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// suggest ranks slugs for a missed lookup: substring hits (either
// direction) beat everything, then edit distance breaks ties.
func (r *Registry) suggest(key string, n int) []string {
	type scored struct {
		slug string
		dist int
	}
	ranked := make([]scored, 0, len(r.slugs))
	for _, s := range r.slugs {
		// Substring hits win outright; levenshtein only runs when needed.
		var dist int
		if !strings.Contains(s.norm, key) && !strings.Contains(key, s.norm) {
			dist = levenshtein(key, s.norm)
		}
		ranked = append(ranked, scored{slug: s.slug, dist: dist})
	}
	slices.SortFunc(ranked, func(a, b scored) int {
		return cmp.Or(cmp.Compare(a.dist, b.dist), strings.Compare(a.slug, b.slug))
	})
	if len(ranked) > n {
		ranked = ranked[:n]
	}
	out := make([]string, 0, len(ranked))
	for _, s := range ranked {
		out = append(out, s.slug)
	}
	return out
}

// levenshtein is the classic two-row edit distance; rosters are a few
// hundred short strings, so no need for anything fancier.
func levenshtein(a, b string) int {
	ar, br := []rune(a), []rune(b)
	prev := make([]int, len(br)+1)
	curr := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		curr[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			curr[j] = min(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(br)]
}
