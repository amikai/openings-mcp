package jobmcp

import (
	"testing"

	job104 "github.com/amikai/job-mcp/internal/104"
)

func TestTW104ToRequest(t *testing.T) {
	in := tw104SearchInput{
		Keyword: "golang",
		Area:    "taipei",
		JobType: "part",
		Sort:    "newest",
		Remote:  "full",
		Page:    2,
	}
	got, err := tw104ToRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Keyword != "golang" {
		t.Errorf("Keyword = %q, want golang", got.Keyword)
	}
	if got.Area != job104.AreaTaipei {
		t.Errorf("Area = %q, want %q", got.Area, job104.AreaTaipei)
	}
	if got.RO == nil || *got.RO != 1 {
		t.Errorf("RO = %v, want 1", got.RO)
	}
	if got.Order == nil || *got.Order != 15 {
		t.Errorf("Order = %v, want 15", got.Order)
	}
	if got.RemoteWork == nil || *got.RemoteWork != 2 {
		t.Errorf("RemoteWork = %v, want 2", got.RemoteWork)
	}
	if got.Page == nil || *got.Page != 2 {
		t.Errorf("Page = %v, want 2", got.Page)
	}
}

func TestTW104ToRequestInvalidArea(t *testing.T) {
	_, err := tw104ToRequest(tw104SearchInput{Keyword: "x", Area: "atlantis"})
	if err == nil {
		t.Fatal("expected error for invalid area, got nil")
	}
}
