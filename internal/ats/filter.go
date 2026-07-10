package ats

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"
)

// dumpJob is the filter engine's intermediate shape for one job from a
// full-dump provider (lever, ashby): the unified summary plus the
// searchable text and structured fields filtering needs. Adapters build
// these; the engine never touches provider types.
type dumpJob struct {
	summary     JobSummary
	sortKey     time.Time         // posting time, for deterministic newest-first ordering
	orgUnit     string            // query tier 2: team/department text (tier 1 is summary.Title)
	description string            // query tier 3: full JD plain text
	locations   string            // every location string joined, for fuzzy matching
	fields      map[string]string // structured dimensions, e.g. "team" -> "Platform"
	isRemote    bool
}

// searchDump filters, ranks, and pages a full board dump. The upstream has
// no usable server-side search, so this layer IS the search — lossless,
// since the dump is complete. Ordering is deterministic (rank, then posted
// desc, then id) because stateless pagination depends on it.
func searchDump(jobs []dumpJob, p SearchParams) (*SearchResult, error) {
	if err := validateFilterKeys(jobs, p.Filters); err != nil {
		return nil, err
	}
	// Loop-invariant normalizations, hoisted out of the per-job matchers.
	words := strings.Fields(strings.ToLower(p.Query))
	loc := strings.ToLower(strings.TrimSpace(p.Location))

	// matched carries pointers plus a precomputed rank: sorting fat dumpJob
	// values (they hold full JD text) and re-ranking inside the comparator
	// would dominate the search cost on large boards.
	type scoredJob struct {
		job  *dumpJob
		rank int
	}
	matched := make([]scoredJob, 0, len(jobs))
	for i := range jobs {
		j := &jobs[i]
		// Cheapest predicates first: map lookups and a small Contains
		// before matchQuery builds the full-JD search blob.
		if !matchFilters(j, p.Filters) || !matchLocation(j, loc) || !matchQuery(j, words) {
			continue
		}
		matched = append(matched, scoredJob{job: j, rank: queryRank(j, words)})
	}
	slices.SortFunc(matched, func(a, b scoredJob) int {
		return cmp.Or(
			cmp.Compare(a.rank, b.rank),
			b.job.sortKey.Compare(a.job.sortKey), // newest first
			strings.Compare(a.job.summary.JobID, b.job.summary.JobID),
		)
	})

	page := clampPage(p.Page)
	total := len(matched)
	start := min((page-1)*PageSize, total)
	end := min(start+PageSize, total)
	out := make([]JobSummary, 0, end-start)
	for _, m := range matched[start:end] {
		out = append(out, m.job.summary)
	}
	return &SearchResult{Jobs: out, TotalCount: total, Page: page, TotalPages: totalPages(total)}, nil
}

// matchQuery requires every query word somewhere in the job's text.
// Ranking (title hits first) happens separately in queryRank.
func matchQuery(j *dumpJob, words []string) bool {
	if len(words) == 0 {
		return true
	}
	blob := strings.ToLower(j.summary.Title + " " + j.orgUnit + " " + j.description)
	return containsAllWords(blob, words)
}

// queryRank orders matches: 0 when the title alone satisfies the whole
// query, 1 otherwise. A title hit is a far stronger signal than a JD-body
// mention.
func queryRank(j *dumpJob, words []string) int {
	if len(words) == 0 || containsAllWords(strings.ToLower(j.summary.Title), words) {
		return 0
	}
	return 1
}

func containsAllWords(text string, words []string) bool {
	for _, w := range words {
		if !strings.Contains(text, w) {
			return false
		}
	}
	return true
}

// matchLocation takes the already-lowercased, trimmed location.
func matchLocation(j *dumpJob, loc string) bool {
	if loc == "" {
		return true
	}
	if loc == "remote" {
		return j.isRemote || strings.Contains(strings.ToLower(j.locations), "remote")
	}
	return strings.Contains(strings.ToLower(j.locations), loc)
}

func matchFilters(j *dumpJob, filters map[string][]string) bool {
	for key, values := range filters {
		actual := j.fields[key]
		if actual == "" {
			return false
		}
		if !slices.ContainsFunc(values, func(v string) bool { return strings.EqualFold(actual, v) }) {
			return false
		}
	}
	return true
}

// validateFilterKeys rejects unknown dimensions up front with a teaching
// error, instead of silently matching nothing.
func validateFilterKeys(jobs []dumpJob, filters map[string][]string) error {
	if len(filters) == 0 {
		return nil
	}
	valid := make(map[string]bool)
	for i := range jobs {
		for k := range jobs[i].fields {
			valid[k] = true
		}
	}
	for key := range filters {
		if !valid[key] {
			return errUnknownFilterKey(key, valid)
		}
	}
	return nil
}

// errUnknownFilterKey is the one teaching error both adapter families
// return for an unknown filter dimension — part of keeping the families
// indistinguishable to the LLM.
func errUnknownFilterKey(key string, valid map[string]bool) error {
	keys := slices.Sorted(maps.Keys(valid))
	return fmt.Errorf("unknown filter key %q; valid keys: %s", key, strings.Join(keys, ", "))
}

// distinctFilters enumerates a dump's structured dimensions — the
// full-dump family's implementation of get_filters.
func distinctFilters(jobs []dumpJob) FilterSet {
	seen := make(map[string]map[string]bool)
	for i := range jobs {
		for k, v := range jobs[i].fields {
			if v == "" {
				continue
			}
			if seen[k] == nil {
				seen[k] = make(map[string]bool)
			}
			seen[k][v] = true
		}
	}
	return toFilterSet(seen)
}

// toFilterSet flattens dimension→value sets into the sorted FilterSet both
// adapter families return.
func toFilterSet(seen map[string]map[string]bool) FilterSet {
	fs := make(FilterSet, len(seen))
	for k, values := range seen {
		fs[k] = slices.Sorted(maps.Keys(values))
	}
	return fs
}
