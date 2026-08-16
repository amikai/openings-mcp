package asus

import (
	"bytes"
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
