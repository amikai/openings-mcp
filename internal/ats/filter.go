package ats

import (
	"fmt"
	"sort"
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
	title       string            // query tier 1
	orgUnit     string            // query tier 2: team/department text
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
	matched := make([]dumpJob, 0, len(jobs))
	for _, j := range jobs {
		if matchQuery(j, p.Query) && matchLocation(j, p.Location) && matchFilters(j, p.Filters) {
			matched = append(matched, j)
		}
	}
	sort.Slice(matched, func(i, k int) bool {
		a, b := matched[i], matched[k]
		ra, rb := queryRank(a, p.Query), queryRank(b, p.Query)
		if ra != rb {
			return ra < rb
		}
		if !a.sortKey.Equal(b.sortKey) {
			return a.sortKey.After(b.sortKey)
		}
		return a.summary.JobID < b.summary.JobID
	})

	page := max(p.Page, 1)
	total := len(matched)
	totalPages := (total + PageSize - 1) / PageSize
	start := min((page-1)*PageSize, total)
	end := min(start+PageSize, total)
	out := make([]JobSummary, 0, end-start)
	for _, j := range matched[start:end] {
		out = append(out, j.summary)
	}
	return &SearchResult{Jobs: out, TotalCount: total, Page: page, TotalPages: totalPages}, nil
}

// matchQuery requires every query word somewhere in the job's text.
// Ranking (title hits first) happens separately in queryRank.
func matchQuery(j dumpJob, query string) bool {
	words := strings.Fields(strings.ToLower(query))
	if len(words) == 0 {
		return true
	}
	blob := strings.ToLower(j.title + " " + j.orgUnit + " " + j.description)
	for _, w := range words {
		if !strings.Contains(blob, w) {
			return false
		}
	}
	return true
}

// queryRank orders matches: 0 when the title alone satisfies the whole
// query, 1 otherwise. A title hit is a far stronger signal than a JD-body
// mention.
func queryRank(j dumpJob, query string) int {
	words := strings.Fields(strings.ToLower(query))
	if len(words) == 0 {
		return 0
	}
	title := strings.ToLower(j.title)
	for _, w := range words {
		if !strings.Contains(title, w) {
			return 1
		}
	}
	return 0
}

func matchLocation(j dumpJob, location string) bool {
	loc := strings.ToLower(strings.TrimSpace(location))
	if loc == "" {
		return true
	}
	if loc == "remote" {
		return j.isRemote || strings.Contains(strings.ToLower(j.locations), "remote")
	}
	return strings.Contains(strings.ToLower(j.locations), loc)
}

func matchFilters(j dumpJob, filters map[string][]string) bool {
	for key, values := range filters {
		actual := j.fields[key]
		if actual == "" {
			return false
		}
		hit := false
		for _, v := range values {
			if strings.EqualFold(actual, v) {
				hit = true
				break
			}
		}
		if !hit {
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
	for _, j := range jobs {
		for k := range j.fields {
			valid[k] = true
		}
	}
	for key := range filters {
		if !valid[key] {
			keys := make([]string, 0, len(valid))
			for k := range valid {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			return fmt.Errorf("unknown filter key %q; valid keys: %s", key, strings.Join(keys, ", "))
		}
	}
	return nil
}

// distinctFilters enumerates a dump's structured dimensions — the
// full-dump family's implementation of get_filters.
func distinctFilters(jobs []dumpJob) FilterSet {
	seen := make(map[string]map[string]bool)
	for _, j := range jobs {
		for k, v := range j.fields {
			if v == "" {
				continue
			}
			if seen[k] == nil {
				seen[k] = make(map[string]bool)
			}
			seen[k][v] = true
		}
	}
	fs := make(FilterSet, len(seen))
	for k, values := range seen {
		list := make([]string, 0, len(values))
		for v := range values {
			list = append(list, v)
		}
		sort.Strings(list)
		fs[k] = list
	}
	return fs
}
