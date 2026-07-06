package ats

import (
	"strings"
	"testing"
	"time"
)

func dj(id, title, orgUnit, desc, loc string, posted time.Time, fields map[string]string, remote bool) dumpJob {
	return dumpJob{
		summary:     JobSummary{JobID: id, Title: title, Location: loc, PostedAt: posted.Format("2006-01-02"), URL: "https://example.com/" + id},
		sortKey:     posted,
		orgUnit:     orgUnit,
		description: desc,
		locations:   loc,
		fields:      fields,
		isRemote:    remote,
	}
}

func testJobs() []dumpJob {
	day := func(n int) time.Time { return time.Date(2026, 7, n, 0, 0, 0, 0, time.UTC) }
	return []dumpJob{
		dj("a", "Senior Go Engineer", "Platform", "You will write Go services", "Taipei, Taiwan", day(1), map[string]string{"team": "Platform", "commitment": "Full-time"}, false),
		dj("b", "Frontend Engineer", "Web", "React and TypeScript, some Go tooling", "London, UK", day(3), map[string]string{"team": "Web", "commitment": "Full-time"}, false),
		dj("c", "Data Scientist", "ML", "Python and statistics", "Remote - US", day(2), map[string]string{"team": "ML", "commitment": "Contract"}, true),
	}
}

func TestSearchDumpQueryANDAcrossWords(t *testing.T) {
	res, err := searchDump(testJobs(), SearchParams{Query: "go engineer"})
	if err != nil {
		t.Fatal(err)
	}
	// "go engineer": job a matches both words in title; job b matches
	// "engineer" in title and "go" only in description — still a match
	// (AND is across the whole text), but ranked below the title hit.
	if len(res.Jobs) != 2 {
		t.Fatalf("got %d jobs, want 2", len(res.Jobs))
	}
	if res.Jobs[0].JobID != "a" {
		t.Errorf("title hit should rank first, got %q", res.Jobs[0].JobID)
	}
	if res.TotalCount != 2 {
		t.Errorf("TotalCount = %d, want 2", res.TotalCount)
	}
}

func TestSearchDumpSortNewestFirstIDTiebreak(t *testing.T) {
	res, err := searchDump(testJobs(), SearchParams{})
	if err != nil {
		t.Fatal(err)
	}
	// No query: rank is uniform, so order is posted desc: b(3rd) c(2nd) a(1st).
	want := []string{"b", "c", "a"}
	for i, w := range want {
		if res.Jobs[i].JobID != w {
			t.Fatalf("order = %v..., want %v", res.Jobs[i].JobID, want)
		}
	}
}

func TestSearchDumpLocation(t *testing.T) {
	res, err := searchDump(testJobs(), SearchParams{Location: "taipei"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Jobs) != 1 || res.Jobs[0].JobID != "a" {
		t.Fatalf("location=taipei should match only job a, got %v", res.Jobs)
	}
}

func TestSearchDumpRemoteSpecialCase(t *testing.T) {
	res, err := searchDump(testJobs(), SearchParams{Location: "remote"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Jobs) != 1 || res.Jobs[0].JobID != "c" {
		t.Fatalf("location=remote should match only job c, got %v", res.Jobs)
	}
}

func TestSearchDumpFiltersORWithinKeyANDAcrossKeys(t *testing.T) {
	res, err := searchDump(testJobs(), SearchParams{
		Filters: map[string][]string{"team": {"Platform", "Web"}, "commitment": {"Full-time"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Jobs) != 2 {
		t.Fatalf("got %d jobs, want 2 (a and b)", len(res.Jobs))
	}
}

func TestSearchDumpFilterValueCaseInsensitive(t *testing.T) {
	res, err := searchDump(testJobs(), SearchParams{Filters: map[string][]string{"team": {"platform"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Jobs) != 1 || res.Jobs[0].JobID != "a" {
		t.Fatalf("got %v, want job a", res.Jobs)
	}
}

func TestSearchDumpUnknownFilterKeyTeaches(t *testing.T) {
	_, err := searchDump(testJobs(), SearchParams{Filters: map[string][]string{"bogus": {"x"}}})
	if err == nil {
		t.Fatal("want error for unknown filter key")
	}
	if !strings.Contains(err.Error(), "team") {
		t.Errorf("error should list valid keys, got: %v", err)
	}
}

func TestSearchDumpPagination(t *testing.T) {
	jobs := make([]dumpJob, 0, 45)
	for i := range 45 {
		posted := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Hour)
		jobs = append(jobs, dj(strings.Repeat("z", 3)+string(rune('a'+i%26))+string(rune('0'+i/26)), "Engineer", "", "", "X", posted, nil, false))
	}
	page2, err := searchDump(jobs, SearchParams{Page: 2})
	if err != nil {
		t.Fatal(err)
	}
	if page2.TotalCount != 45 || page2.TotalPages != 3 || page2.Page != 2 || len(page2.Jobs) != PageSize {
		t.Fatalf("page2 = {total %d, pages %d, page %d, len %d}", page2.TotalCount, page2.TotalPages, page2.Page, len(page2.Jobs))
	}
	page3, _ := searchDump(jobs, SearchParams{Page: 3})
	if len(page3.Jobs) != 5 {
		t.Errorf("page3 len = %d, want 5", len(page3.Jobs))
	}
	page9, _ := searchDump(jobs, SearchParams{Page: 9})
	if len(page9.Jobs) != 0 {
		t.Errorf("past-the-end page should be empty, got %d", len(page9.Jobs))
	}
	// Determinism: two identical calls agree item-for-item.
	again, _ := searchDump(jobs, SearchParams{Page: 2})
	for i := range page2.Jobs {
		if page2.Jobs[i].JobID != again.Jobs[i].JobID {
			t.Fatal("pagination is not deterministic")
		}
	}
}

func TestDistinctFilters(t *testing.T) {
	fs := distinctFilters(testJobs())
	if got := fs["team"]; len(got) != 3 || got[0] != "ML" {
		t.Errorf(`fs["team"] = %v, want sorted [ML Platform Web]`, got)
	}
	if got := fs["commitment"]; len(got) != 2 {
		t.Errorf(`fs["commitment"] = %v, want 2 distinct values`, got)
	}
}
