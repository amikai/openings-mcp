package taiwanjobs

import (
	"context"
	"testing"
)

func TestJobs(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	resp, err := c.Jobs(context.Background(), &JobsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Fetched != 3 || len(resp.Jobs) != 3 {
		t.Fatalf("got fetched=%d jobs=%d, want 3 and 3", resp.Fetched, len(resp.Jobs))
	}
	j := resp.Jobs[0]
	if j.Title != "家庭幫傭" {
		t.Errorf("title = %q", j.Title)
	}
	if j.UpdatedAt != "2026-07-16" {
		t.Errorf("updated_at = %q, want ISO date", j.UpdatedAt)
	}
	if j.URL == "" || j.Description == "" || j.Location == "" {
		t.Errorf("expected url/description/location populated, got %+v", j)
	}
}

func TestJobsKeywordFilter(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	resp, err := c.Jobs(context.Background(), &JobsRequest{Keyword: "java"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Fetched != 3 {
		t.Errorf("fetched = %d, want 3 (pre-filter count)", resp.Fetched)
	}
	if len(resp.Jobs) != 1 {
		t.Fatalf("got %d jobs after keyword filter, want 1", len(resp.Jobs))
	}
	if got := resp.Jobs[0].Company; got != "指南動力有限公司" {
		t.Errorf("company = %q", got)
	}
}

func TestJobsCountTooLarge(t *testing.T) {
	c := NewClient("http://unused", nil)
	if _, err := c.Jobs(context.Background(), &JobsRequest{Count: MaxCount + 1}); err == nil {
		t.Fatal("expected error for count over upstream max")
	}
}
