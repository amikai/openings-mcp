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
//
// The same normalized slug or display name can belong to entries from
// different adapters — ATS-local identifiers were never a shared
// namespace. bySlug and byName therefore map to every entry sharing a key,
// and [Registry.Resolve] disambiguates when more than one survives.
type Registry struct {
	adapters []Adapter                  // registration order; polled for careers-URL input
	bySlug   map[string][]registryEntry // key: normalize(slug)
	byName   map[string][]registryEntry // key: normalize(display name)
	slugs    []slugEntry                // sorted by slug, for suggestions; one entry per distinct roster slug
	// entryCount is the number of roster rows across every adapter — the
	// figure the miss error advertises. It is not len(bySlug) or
	// len(byName): those count distinct normalized keys, which undercounts
	// once cross-adapter collisions share a key.
	entryCount int
}

// careersHostPatternsByAdapter maps each known adapter name to the
// careers-page URL shape it recognizes, so the "unrecognized careers URL"
// error only advertises hosts the registry actually has an adapter for.
var careersHostPatternsByAdapter = map[string]string{
	"workday":         "<tenant>.<wd*>.myworkdayjobs.com/<site>",
	"adp_myjobs":      "myjobs.adp.com/<slug>",
	"adp_wfn":         "workforcenow.adp.com/mascsr/.../recruitment.html?cid=<guid>",
	"avature":         "<tenant>.avature.net/<portal> (custom-domain portals via roster only)",
	"bamboohr":        "<company>.bamboohr.com/careers",
	"greenhouse":      "job-boards.greenhouse.io/<board>",
	"hrmos":           "hrmos.co/pages/<slug>",
	"lever":           "jobs.lever.co/<org>",
	"ashby":           "jobs.ashbyhq.com/<org>",
	"teamtailor":      "<company>[.na|.au].teamtailor.com/jobs",
	"recruitee":       "<company>.recruitee.com",
	"herp":            "herp.careers/careers/companies/<company> (also herp.careers/v1/<company>)",
	"engage":          "en-gage.net/<company>",
	"eightfold":       "<tenant>.eightfold.ai/careers (roster tenants only)",
	"successfactors":  "jobs.<company>.com/search (roster tenants only)",
	"smartrecruiters": "jobs.smartrecruiters.com/<company>",
	"workable":        "apply.workable.com/<company>",
	"rippling":        "ats.rippling.com/<board>/jobs",
	"icims":           "careers-<slug>.icims.com/jobs/search",
	"oracle":          "<fusion>.oraclecloud.com/hcmUI/CandidateExperience/<lang>/sites/<site>/jobs",
	"join":            "join.com/companies/<company> (roster companies only)",
	"mokahr":          "app.mokahr.com/<social-recruitment|campus[_-]apply|campus[_-]recruitment>/<org>/<siteId>",
	"ultipro":         "recruiting<N>.ultipro.com/<companyCode>/JobBoard/<boardId>",
	"dayforce":        "jobs.dayforcehcm.com/[<culture>/]<namespace>/<jobBoardCode>",
}

// CompanyCandidate is one human-readable choice in a [CompanyResolution].
// CareersURL is empty when the owning adapter has no public URL for this
// entry; Provider is carried for logs and debug tooling and is never
// rendered in MCP-facing text.
type CompanyCandidate struct {
	Name       string
	CareersURL string
	Provider   string
}

// CompanyResolution is the successful result of resolving a company input.
// It may contain one exact match or multiple choices that the caller must
// disambiguate with the user.
type CompanyResolution struct {
	entries []registryEntry
}

// IsAmbiguous reports whether the input matched more than one company.
func (r CompanyResolution) IsAmbiguous() bool {
	return len(r.entries) > 1
}

// Single returns the adapter and provider-local slug when exactly one company
// matched.
func (r CompanyResolution) Single() (Adapter, string, bool) {
	if len(r.entries) != 1 {
		return nil, "", false
	}
	entry := r.entries[0]
	return entry.adapter, entry.slug, true
}

// Candidates returns the human-readable choices in registration order.
func (r CompanyResolution) Candidates() []CompanyCandidate {
	candidates := make([]CompanyCandidate, 0, len(r.entries))
	for _, entry := range r.entries {
		candidate := CompanyCandidate{
			Name:     entry.name,
			Provider: entry.adapter.Name(),
		}
		if url, ok := entry.adapter.CareersURL(entry.slug); ok {
			candidate.CareersURL = url
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

// Select returns the adapter and provider-local slug for a candidate index.
func (r CompanyResolution) Select(index int) (Adapter, string, bool) {
	if index < 0 || index >= len(r.entries) {
		return nil, "", false
	}
	entry := r.entries[index]
	return entry.adapter, entry.slug, true
}

// NewRegistry unions the adapters' rosters. Cross-adapter slug and name
// collisions are expected — ATS-local identifiers are only unique within
// their own ATS — so they are indexed for [Registry.Resolve] to
// disambiguate at lookup time, not rejected here.
//
// A collision within one adapter's own roster stays a fatal curation bug:
// two rows sharing a slug would render the same careers URL, so a retry
// could never disambiguate them, and two rows sharing a name in a roster
// this repo owns outright has no upstream cause.
//
// An entry whose adapter cannot render a careers URL for it (CareersURL
// returns ok=false) is required to have a normalized name that collides
// with no other entry's normalized name or slug: that name is the only
// retry a caller can offer for it, so the name must resolve uniquely.
func NewRegistry(adapters ...Adapter) (*Registry, error) {
	r := &Registry{
		adapters: adapters,
		bySlug:   make(map[string][]registryEntry),
		byName:   make(map[string][]registryEntry),
	}
	seenSlugs := make(map[string]bool)
	var allEntries []registryEntry
	for _, a := range adapters {
		localSlugs := make(map[string]string) // normalized -> original slug, this adapter's roster only
		localNames := make(map[string]string) // normalized -> original name, this adapter's roster only
		for _, c := range a.Roster() {
			e := registryEntry{adapter: a, slug: c.Slug, name: c.Name}

			slugKey := normalize(c.Slug)
			if prev, ok := localSlugs[slugKey]; ok {
				return nil, fmt.Errorf("ats: company slug %q from %s collides with %q in the same roster", c.Slug, a.Name(), prev)
			}
			localSlugs[slugKey] = c.Slug

			nameKey := normalize(c.Name)
			if prev, ok := localNames[nameKey]; ok {
				return nil, fmt.Errorf("ats: company name %q from %s collides with %q in the same roster", c.Name, a.Name(), prev)
			}
			localNames[nameKey] = c.Name

			r.bySlug[slugKey] = append(r.bySlug[slugKey], e)
			r.byName[nameKey] = append(r.byName[nameKey], e)
			allEntries = append(allEntries, e)
			r.entryCount++
			if !seenSlugs[c.Slug] {
				seenSlugs[c.Slug] = true
				r.slugs = append(r.slugs, slugEntry{slug: c.Slug, norm: slugKey})
			}
		}
	}

	for _, e := range allEntries {
		if _, ok := e.adapter.CareersURL(e.slug); ok {
			continue
		}
		nameKey := normalize(e.name)
		for _, other := range r.byName[nameKey] {
			if other != e {
				return nil, fmt.Errorf("ats: company %q from %s has no careers URL and its name collides with %q from %s; its name-only retry could not resolve uniquely",
					e.name, e.adapter.Name(), other.name, other.adapter.Name())
			}
		}
		for _, other := range r.bySlug[nameKey] {
			if other != e {
				return nil, fmt.Errorf("ats: company %q from %s has no careers URL and its name collides with slug %q from %s; its name-only retry could not resolve uniquely",
					e.name, e.adapter.Name(), other.slug, other.adapter.Name())
			}
		}
	}

	slices.SortFunc(r.slugs, func(a, b slugEntry) int { return strings.Compare(a.slug, b.slug) })
	return r, nil
}

// Resolve maps a user-supplied company string to a [CompanyResolution]. The
// input can be a roster slug, a display name, or a careers URL. A selected
// slug is not always a roster key: a careers URL for a company outside the
// roster resolves to whatever slug the owning adapter minted via
// [Adapter.ParseCareersURL] (Workday mints the canonical careers URL).
//
// A careers URL is authoritative and answers immediately. Anything else —
// including a URL shape no adapter recognizes — falls through to the
// roster union of slug and name hits. One or more hits are a successful
// resolution; callers decide how to handle multiple choices. Zero returns
// a teaching error with either the closest roster slugs or the supported
// careers-page hosts when the input was URL-shaped.
func (r *Registry) Resolve(company string) (CompanyResolution, error) {
	key := normalize(company)
	if key == "" {
		return CompanyResolution{}, errors.New("company is required")
	}

	urlShaped := false
	if u, ok := parseCareersInput(company); ok {
		urlShaped = true
		for _, a := range r.adapters {
			if slug, ok := a.ParseCareersURL(u); ok {
				return CompanyResolution{
					entries: []registryEntry{{adapter: a, slug: slug}},
				}, nil
			}
		}
	}

	candidates := r.candidates(key)
	if len(candidates) == 0 {
		if urlShaped {
			return CompanyResolution{}, fmt.Errorf("unrecognized careers URL %q; supported careers-page hosts: %s", company, strings.Join(r.careersHostPatterns(), ", "))
		}
		return CompanyResolution{}, fmt.Errorf("unknown company %q; closest matches: %s. %d companies are supported — pass one of the suggested slugs",
			company, strings.Join(r.suggest(key, 3), ", "), r.entryCount)
	}
	return CompanyResolution{entries: candidates}, nil
}

// candidates returns the deduplicated union of bySlug[key] and byName[key],
// with identity (adapter, slug) — an entry whose slug and name both
// normalize to key (common when the two already match, e.g. "bunq") must
// collapse to one candidate rather than becoming ambiguous with itself.
func (r *Registry) candidates(key string) []registryEntry {
	slugHits, nameHits := r.bySlug[key], r.byName[key]
	if len(slugHits) == 0 && len(nameHits) == 0 {
		return nil
	}
	seen := make(map[registryEntry]bool, len(slugHits)+len(nameHits))
	out := make([]registryEntry, 0, len(slugHits)+len(nameHits))
	for _, e := range slugHits {
		if !seen[e] {
			seen[e] = true
			out = append(out, e)
		}
	}
	for _, e := range nameHits {
		if !seen[e] {
			seen[e] = true
			out = append(out, e)
		}
	}
	return out
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
// direction) beat everything, then edit distance breaks ties. r.slugs
// already carries at most one entry per distinct roster slug, so results
// need no further deduplication.
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
