package weworkremotely

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testClient(t *testing.T) *Client {
	t.Helper()
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, srv.Client())
}

// testClientWithFailingCategory behaves like testClient, except the given
// category's feed answers HTTP 500 instead of its fixture — for exercising
// AllJobs/Search/Detail's partial-failure handling.
func testClientWithFailingCategory(t *testing.T, failSlug string) *Client {
	t.Helper()
	fixtures := map[string][]byte{
		"remote-full-stack-programming-jobs": mockFullStackRsp,
		"remote-design-jobs":                 mockDesignRsp,
		"remote-back-end-programming-jobs":   mockBackEndRsp,
	}

	mux := http.NewServeMux()
	for _, cat := range Categories {
		if cat.Slug == failSlug {
			mux.HandleFunc("/categories/"+cat.Slug+".rss", func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "boom", http.StatusInternalServerError)
			})
			continue
		}
		body, ok := fixtures[cat.Slug]
		if !ok {
			body = []byte(emptyFeed)
		}
		mux.HandleFunc("/categories/"+cat.Slug+".rss", serveRSS(body))
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, srv.Client())
}

func TestClient_Jobs(t *testing.T) {
	c := testClient(t)
	fullStack := Categories[5] // Full-Stack Programming
	if fullStack.Slug != "remote-full-stack-programming-jobs" {
		t.Fatalf("test assumes Categories[5] is Full-Stack Programming, got %+v", fullStack)
	}

	jobs, err := c.Jobs(context.Background(), fullStack)
	if err != nil {
		t.Fatalf("Jobs: %v", err)
	}
	if len(jobs) != 42 {
		t.Fatalf("got %d jobs, want 42 (the captured fixture's item count)", len(jobs))
	}

	j := jobs[0]
	if j.ID == "" {
		t.Error("job ID is empty")
	}
	if j.Company == "" {
		t.Error("job Company is empty")
	}
	if j.Title == "" {
		t.Error("job Title is empty")
	}
	if j.PostedAt.IsZero() {
		t.Error("job PostedAt did not parse")
	}
	if j.URL == "" {
		t.Error("job URL is empty")
	}
}

func TestClient_Jobs_titleWithMultipleColons(t *testing.T) {
	c := testClient(t)
	jobs, err := c.Jobs(context.Background(), Categories[2]) // Design
	if err != nil {
		t.Fatalf("Jobs: %v", err)
	}
	found := false
	for _, j := range jobs {
		if j.Company == "Education Sub Saharan Africa" {
			found = true
			if j.Title != "Consultancy: Design and Implementation of ACSL Impact Studies" {
				t.Errorf("Title = %q, want the text after the first colon only", j.Title)
			}
		}
	}
	if !found {
		t.Fatal("expected the multi-colon-title fixture job in the Design feed")
	}
}

func TestClient_AllJobs_mergesEveryCategory(t *testing.T) {
	c := testClient(t)
	jobs, err := c.AllJobs(context.Background())
	if err != nil {
		t.Fatalf("AllJobs: %v", err)
	}
	// 42 (full-stack) + 23 (design) + 7 (back-end) minus 1: the captured
	// full-stack fixture itself contains one job listed twice (a real WWR
	// duplicate-listing quirk, not a fixture-capture error — see
	// TestClient_Jobs which confirms the raw per-feed count is 42).
	// Every other category serves the mock's empty feed.
	if want := 42 + 23 + 7 - 1; len(jobs) != want {
		t.Fatalf("got %d jobs, want %d", len(jobs), want)
	}

	seen := make(map[string]bool)
	for _, j := range jobs {
		if seen[j.ID] {
			t.Fatalf("duplicate job ID %q in merged AllJobs result", j.ID)
		}
		seen[j.ID] = true
	}
}

func TestClient_Search_categoryFetchesOnlyThatFeed(t *testing.T) {
	c := testClient(t)
	jobs, err := c.Search(context.Background(), FilterOptions{Category: "Design"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(jobs) != 23 {
		t.Fatalf("got %d jobs, want 23 (the Design fixture's full count, unfiltered further)", len(jobs))
	}
	for _, j := range jobs {
		if j.Category != "Design" {
			t.Errorf("job %q has Category %q, want Design", j.ID, j.Category)
		}
	}
}

func TestClient_Search_categoryPathDedupesLikeFullDump(t *testing.T) {
	c := testClient(t)
	jobs, err := c.Search(context.Background(), FilterOptions{Category: "Full-Stack Programming"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	// The captured fixture has 42 raw items but one duplicate (see
	// TestClient_Jobs and TestClient_AllJobs_mergesEveryCategory) —
	// Search's recognized-category fast path must dedupe the same way the
	// AllJobs-backed full-dump path does, not just return the raw feed.
	if want := 42 - 1; len(jobs) != want {
		t.Fatalf("got %d jobs, want %d", len(jobs), want)
	}
	seen := make(map[string]bool)
	for _, j := range jobs {
		if seen[j.ID] {
			t.Fatalf("duplicate job ID %q in Search result", j.ID)
		}
		seen[j.ID] = true
	}
}

func TestClient_Search_unrecognizedCategoryFallsBackToFullDump(t *testing.T) {
	c := testClient(t)
	jobs, err := c.Search(context.Background(), FilterOptions{Category: "Not A Real Category"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("got %d jobs, want 0 — no job's Category equals the unrecognized filter", len(jobs))
	}
}

func TestClient_Detail(t *testing.T) {
	c := testClient(t)
	jobs, err := c.Jobs(context.Background(), Categories[5]) // Full-Stack Programming
	if err != nil {
		t.Fatalf("Jobs: %v", err)
	}
	want := jobs[0]

	got, err := c.Detail(context.Background(), want.ID)
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if got.ID != want.ID || got.Title != want.Title || got.Description != want.Description {
		t.Fatalf("Detail returned a different job than Jobs: got %+v, want %+v", got.ID, want.ID)
	}
}

func TestClient_Detail_notFound(t *testing.T) {
	c := testClient(t)
	_, err := c.Detail(context.Background(), "no-such-job-slug")
	if err == nil {
		t.Fatal("expected an error for an unknown job ID")
	}
}

func TestClient_AllJobs_partialFailureReturnsWhatSucceeded(t *testing.T) {
	c := testClientWithFailingCategory(t, "remote-full-stack-programming-jobs")
	jobs, err := c.AllJobs(context.Background())
	if err == nil {
		t.Fatal("expected a non-nil error reporting the failed category")
	}
	// Design (23) + Back-End (7); Full-Stack's feed failed and every other
	// category serves the mock's empty feed.
	if want := 23 + 7; len(jobs) != want {
		t.Fatalf("got %d jobs, want %d — a failing category should not discard jobs from feeds that succeeded", len(jobs), want)
	}
}

func TestClient_Search_partialFailureReturnsWhatSucceeded(t *testing.T) {
	c := testClientWithFailingCategory(t, "remote-full-stack-programming-jobs")
	jobs, err := c.Search(context.Background(), FilterOptions{})
	if err == nil {
		t.Fatal("expected a non-nil error reporting the failed category")
	}
	if want := 23 + 7; len(jobs) != want {
		t.Fatalf("got %d jobs, want %d", len(jobs), want)
	}
}

func TestClient_Detail_succeedsDespitePartialFailure(t *testing.T) {
	// The wanted job lives in Design, which stays healthy; only Full-Stack's
	// feed fails.
	healthy := testClient(t)
	designJobs, err := healthy.Jobs(context.Background(), Categories[2]) // Design
	if err != nil {
		t.Fatalf("Jobs: %v", err)
	}
	want := designJobs[0]

	c := testClientWithFailingCategory(t, "remote-full-stack-programming-jobs")
	got, err := c.Detail(context.Background(), want.ID)
	if err != nil {
		t.Fatalf("Detail: %v (should succeed — the job's own feed did not fail)", err)
	}
	if got.ID != want.ID {
		t.Fatalf("Detail returned job %q, want %q", got.ID, want.ID)
	}
}

func TestClient_Detail_notFoundMentionsPartialFailure(t *testing.T) {
	c := testClientWithFailingCategory(t, "remote-full-stack-programming-jobs")
	_, err := c.Detail(context.Background(), "no-such-job-slug")
	if err == nil {
		t.Fatal("expected an error for an unknown job ID")
	}
	if !strings.Contains(err.Error(), "some category feeds failed") {
		t.Errorf("error %q does not mention the underlying category failure", err)
	}
}
