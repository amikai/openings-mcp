package ats

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/amikai/openings-mcp/internal/provider/ashby"
)

func testAshbyAdapter(t *testing.T) *AshbyAdapter {
	t.Helper()
	srv := ashby.NewMockServer()
	t.Cleanup(srv.Close)
	a, err := NewAshbyAdapter(srv.URL, &http.Client{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestAshbyRoster(t *testing.T) {
	a := testAshbyAdapter(t)
	if got, want := len(a.Roster()), len(ashby.Companies); got != want {
		t.Fatalf("roster len = %d, want %d", got, want)
	}
}

func TestAshbySearchAll(t *testing.T) {
	a := testAshbyAdapter(t)
	res, err := a.Search(context.Background(), ashby.MockBoardName, SearchParams{})
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalCount != 5 {
		t.Fatalf("TotalCount = %d, want 5 (all fixture jobs are listed)", res.TotalCount)
	}
	for _, j := range res.Jobs {
		if j.JobID == "" || j.Title == "" || j.URL == "" {
			t.Fatalf("summary with empty field: %+v", j)
		}
	}
}

func TestAshbySearchQueryAndFilters(t *testing.T) {
	a := testAshbyAdapter(t)
	res, err := a.Search(context.Background(), ashby.MockBoardName, SearchParams{Query: "agent platform"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Jobs) == 0 || res.Jobs[0].Title != "Software Engineer (Agent Platform)" {
		t.Fatalf("got %+v, want the Agent Platform job first", res.Jobs)
	}

	filtered, err := a.Search(context.Background(), ashby.MockBoardName, SearchParams{
		Filters: map[string][]string{"employmentType": {"FullTime"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Jobs) == 0 {
		t.Fatal("want at least one FullTime job")
	}
}

func TestAshbyFilters(t *testing.T) {
	a := testAshbyAdapter(t)
	fs, err := a.Filters(context.Background(), ashby.MockBoardName)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"department", "employmentType", "workplaceType"} {
		if len(fs[key]) == 0 {
			t.Errorf("FilterSet missing %q: %v", key, fs)
		}
	}
}

func TestAshbyDetailRefetchesBoard(t *testing.T) {
	a := testAshbyAdapter(t)
	d, err := a.Detail(context.Background(), ashby.MockBoardName, "7724fbe3-6a27-4418-9705-2dcc40751a16")
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "Software Engineer (Agent Platform)" {
		t.Errorf("Title = %q", d.Title)
	}
	if d.Description == "" {
		t.Error("Description should be non-empty plain text")
	}
}

func TestAshbyDetailNotFound(t *testing.T) {
	a := testAshbyAdapter(t)
	if _, err := a.Detail(context.Background(), ashby.MockBoardName, "no-such-id"); err == nil {
		t.Fatal("want error for unknown job id")
	}
}

func TestAshbyUnknownBoardUpstream(t *testing.T) {
	a := testAshbyAdapter(t)
	if _, err := a.Search(context.Background(), "not-in-mock", SearchParams{}); err == nil {
		t.Fatal("want error when upstream returns 404")
	}
}

func TestAshbySearchIsDeterministic(t *testing.T) {
	a := testAshbyAdapter(t)
	r1, _ := a.Search(context.Background(), ashby.MockBoardName, SearchParams{})
	r2, _ := a.Search(context.Background(), ashby.MockBoardName, SearchParams{})
	for i := range r1.Jobs {
		if r1.Jobs[i].JobID != r2.Jobs[i].JobID {
			t.Fatal("search order is not deterministic")
		}
	}
	if !strings.HasPrefix(r1.Jobs[0].PostedAt, "20") {
		t.Errorf("PostedAt should be an ISO date, got %q", r1.Jobs[0].PostedAt)
	}
}
