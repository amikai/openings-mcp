package asus

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

func TestParseSearchHTML_Success(t *testing.T) {
	resp, err := parseSearchHTML(bytes.NewReader(mockJobsRsp), "https://recruit.asus.com")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(resp.Jobs) == 0 {
		t.Fatal("expected at least 1 job, got 0")
	}
	if resp.TotalPages <= 0 {
		t.Errorf("expected total pages > 0, got %d", resp.TotalPages)
	}
	if resp.CurrentPage != 1 {
		t.Errorf("expected current page = 1, got %d", resp.CurrentPage)
	}

	first := resp.Jobs[0]
	if first.Title == "" {
		t.Error("expected non-empty title")
	}
	if first.ID == "" {
		t.Error("expected non-empty ID")
	}
	if !strings.HasPrefix(first.DetailURL, "https://recruit.asus.com") {
		t.Errorf("expected DetailURL to start with base URL, got %q", first.DetailURL)
	}
}

func TestParseSearchHTML_FilterOptions(t *testing.T) {
	resp, err := parseSearchHTML(bytes.NewReader(mockJobsRsp), "https://recruit.asus.com")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if got, want := optionValues(resp.Categories), []string{"研究發展", "業務/行銷", "工程技術", "管理/支援"}; !slices.Equal(got, want) {
		t.Errorf("categories = %v, want %v", got, want)
	}
	if got, want := optionValues(resp.Experiences), []string{"0", "1", "2", "3", "4"}; !slices.Equal(got, want) {
		t.Errorf("experiences = %v, want %v", got, want)
	}
	if got, want := optionLabels(resp.Experiences)[0], "2年以下"; got != want {
		t.Errorf("first experience label = %q, want %q", got, want)
	}

	countries := optionValues(resp.Countries)
	// The "請選擇" placeholder carries no value and must not become an option.
	if slices.Contains(countries, "") {
		t.Error("countries contain the valueless placeholder option")
	}
	if !slices.Contains(countries, "TW") {
		t.Errorf("countries missing TW: %v", countries)
	}
	// Slovenia deviates from ISO 3166-1 alpha-2 (SI).
	if !slices.Contains(countries, "SL") {
		t.Error("countries missing the SL deviation for Slovenia")
	}
}

// TestParseSearchHTML_FilterOptionsEnglish pins the reason the options are
// read off the page rather than compiled into the package: the same board
// serves a different set of category filter values to an en-US session.
func TestParseSearchHTML_FilterOptionsEnglish(t *testing.T) {
	resp, err := parseSearchHTML(bytes.NewReader(mockJobsEnRsp), "https://recruit.asus.com")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	want := []string{
		"Research and Development",
		"Marketing / Sales",
		"Technology / Engineering",
		"Business Support / Administration",
	}
	if got := optionValues(resp.Categories); !slices.Equal(got, want) {
		t.Errorf("categories = %v, want %v", got, want)
	}
	// Country and experience values stay locale-independent codes; only their
	// labels translate.
	if got, want := optionValues(resp.Experiences), []string{"0", "1", "2", "3", "4"}; !slices.Equal(got, want) {
		t.Errorf("experiences = %v, want %v", got, want)
	}
	if got, want := optionLabels(resp.Experiences)[0], "Less than 2 Years"; got != want {
		t.Errorf("first experience label = %q, want %q", got, want)
	}
}

func optionValues(opts []FilterOption) []string {
	values := make([]string, len(opts))
	for i, o := range opts {
		values[i] = o.Value
	}
	return values
}

func optionLabels(opts []FilterOption) []string {
	labels := make([]string, len(opts))
	for i, o := range opts {
		labels[i] = o.Label
	}
	return labels
}

func TestParseSearchHTML_Filtered(t *testing.T) {
	resp, err := parseSearchHTML(bytes.NewReader(mockJobsFilteredRsp), "https://recruit.asus.com")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(resp.Jobs) == 0 {
		t.Fatal("expected filtered jobs, got 0")
	}
}

func TestParseSearchHTML_Empty(t *testing.T) {
	resp, err := parseSearchHTML(bytes.NewReader(mockJobsEmptyRsp), "https://recruit.asus.com")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(resp.Jobs) != 0 {
		t.Fatalf("expected 0 jobs for empty response, got %d", len(resp.Jobs))
	}
	if resp.TotalPages != 0 {
		t.Errorf("expected 0 total pages, got %d", resp.TotalPages)
	}
	if resp.CurrentPage != 0 {
		t.Errorf("expected 0 current page, got %d", resp.CurrentPage)
	}
	// A zero-result page still renders the search form, so callers can offer
	// the filter options that would widen the search.
	if len(resp.Categories) == 0 {
		t.Error("expected the empty result page to still carry the category options")
	}
}

func TestParseDetailHTML_Success(t *testing.T) {
	detail, err := parseDetailHTML(bytes.NewReader(mockJobDetailRsp), "762c08de-1daa-4aa8-9668-d8a746ce24a8", "https://recruit.asus.com")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if detail.ID != "762c08de-1daa-4aa8-9668-d8a746ce24a8" {
		t.Errorf("expected id %q, got %q", "762c08de-1daa-4aa8-9668-d8a746ce24a8", detail.ID)
	}
	if detail.Title == "" {
		t.Error("expected non-empty title")
	}
	if detail.Category == "" {
		t.Error("expected non-empty category")
	}
	if detail.Location == "" {
		t.Error("expected non-empty location")
	}
	if detail.Education == "" {
		t.Error("expected non-empty education")
	}
	if detail.Description == "" {
		t.Error("expected non-empty description")
	}
	if detail.Requirements == "" {
		t.Error("expected non-empty requirements")
	}
}

func TestParseDetailHTML_NotFound(t *testing.T) {
	_, err := parseDetailHTML(bytes.NewReader(mockJobNotFoundRsp), "invalid-id", "https://recruit.asus.com")
	if err == nil {
		t.Fatal("expected error parsing not found response, got nil")
	}
}
