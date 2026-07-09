package ats

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// dumpJob is the filter engine's provider-neutral shape for one full-dump job.
type dumpJob struct {
	summary     JobSummary
	sortKey     time.Time         // posting time, for deterministic newest-first ordering
	orgUnit     string            // query tier 2: team/department text (tier 1 is summary.Title)
	description string            // query tier 3: full JD plain text
	locations   string            // every location string joined, for fuzzy matching
	fields      map[string]string // structured dimensions, e.g. "team" -> "Platform"
	isRemote    bool
}

// searchViaDump and filtersViaDump provide the shared Search/Filters flow for
// adapters whose upstream exposes only a full-board dump.
func searchViaDump(ctx context.Context, dump func(context.Context, string) ([]dumpJob, error), slug string, p SearchParams) (*SearchResult, error) {
	jobs, err := dump(ctx, slug)
	if err != nil {
		return nil, err
	}
	return searchDump(jobs, p)
}

func filtersViaDump(ctx context.Context, dump func(context.Context, string) ([]dumpJob, error), slug string) (FilterSet, error) {
	jobs, err := dump(ctx, slug)
	if err != nil {
		return nil, err
	}
	return distinctFilters(jobs), nil
}

// searchDump filters and pages a complete board dump. Results are ordered by
// query rank, posted time, and ID so stateless pagination is deterministic.
func searchDump(jobs []dumpJob, p SearchParams) (*SearchResult, error) {
	if err := validateFilterKeys(jobs, p.Filters); err != nil {
		return nil, err
	}
	words := strings.Fields(strings.ToLower(p.Query))
	loc := strings.ToLower(strings.TrimSpace(p.Location))

	// Keep pointers and precomputed ranks so sorting does not copy full job text.
	type scoredJob struct {
		job  *dumpJob
		rank int
	}
	matched := make([]scoredJob, 0, len(jobs))
	for i := range jobs {
		j := &jobs[i]
		// Run cheap filters before building the full-JD search text.
		if !matchFilters(j, p.Filters) || !matchLocation(j, loc) || !matchQuery(j, words) {
			continue
		}
		matched = append(matched, scoredJob{job: j, rank: queryRank(j, words)})
	}
	sort.Slice(matched, func(i, k int) bool {
		a, b := matched[i], matched[k]
		if a.rank != b.rank {
			return a.rank < b.rank
		}
		if !a.job.sortKey.Equal(b.job.sortKey) {
			return a.job.sortKey.After(b.job.sortKey)
		}
		return a.job.summary.JobID < b.job.summary.JobID
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

// matchQuery requires every query word to occur in the job text.
func matchQuery(j *dumpJob, words []string) bool {
	if len(words) == 0 {
		return true
	}
	blob := strings.ToLower(j.summary.Title + " " + j.orgUnit + " " + j.description)
	return containsAllWords(blob, words)
}

// queryRank prefers matches whose title contains every query word.
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

// validateFilterKeys rejects unknown dimensions before matching.
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

// errUnknownFilterKey is shared by both adapter families.
func errUnknownFilterKey(key string, valid map[string]bool) error {
	keys := make([]string, 0, len(valid))
	for k := range valid {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return fmt.Errorf("unknown filter key %q; valid keys: %s", key, strings.Join(keys, ", "))
}

// distinctFilters enumerates the structured dimensions in a full dump.
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

// toFilterSet converts dimension/value sets to the sorted public form.
func toFilterSet(seen map[string]map[string]bool) FilterSet {
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
