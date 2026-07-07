package ats

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/amikai/openings-mcp/internal/provider/lever"
)

func testLeverAdapter(t *testing.T) *LeverAdapter {
	t.Helper()
	srv := lever.NewMockServer()
	t.Cleanup(srv.Close)
	a, err := NewLeverAdapter(srv.URL, &http.Client{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestLeverRoster(t *testing.T) {
	a := testLeverAdapter(t)
	roster := a.Roster()
	if len(roster) != len(lever.Companies) {
		t.Fatalf("roster len = %d, want %d", len(roster), len(lever.Companies))
	}
	for _, c := range roster {
		if c.Slug == "" || c.Name == "" {
			t.Fatalf("roster entry with empty field: %+v", c)
		}
	}
}

func TestLeverSearchAll(t *testing.T) {
	a := testLeverAdapter(t)
	res, err := a.Search(context.Background(), "leverdemo", SearchParams{})
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalCount != 3 || len(res.Jobs) != 3 {
		t.Fatalf("got %d/%d jobs, want 3", len(res.Jobs), res.TotalCount)
	}
	for _, j := range res.Jobs {
		if j.JobID == "" || j.Title == "" || j.URL == "" || j.PostedAt == "" {
			t.Fatalf("summary with empty field: %+v", j)
		}
	}
}

func TestLeverSearchQuery(t *testing.T) {
	a := testLeverAdapter(t)
	res, err := a.Search(context.Background(), "leverdemo", SearchParams{Query: "AbelsonTaylor"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Jobs) < 1 || res.Jobs[0].Title != "AbelsonTaylor Writer" {
		t.Fatalf("got %+v, want AbelsonTaylor Writer first", res.Jobs)
	}
}

func TestLeverSearchFilters(t *testing.T) {
	a := testLeverAdapter(t)
	res, err := a.Search(context.Background(), "leverdemo", SearchParams{
		Filters: map[string][]string{"team": {"Professional Services"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Jobs) == 0 {
		t.Fatal("want at least one Professional Services job")
	}
}

func TestLeverFilters(t *testing.T) {
	a := testLeverAdapter(t)
	fs, err := a.Filters(context.Background(), "leverdemo")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, v := range fs["team"] {
		if v == "Professional Services" {
			found = true
		}
	}
	if !found {
		t.Fatalf(`fs["team"] = %v, want it to contain "Professional Services"`, fs["team"])
	}
}

func TestLeverDetail(t *testing.T) {
	a := testLeverAdapter(t)
	d, err := a.Detail(context.Background(), "leverdemo", "33538a2f-d27d-4a96-8f05-fa4b0e4d940e")
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "AbelsonTaylor Writer" {
		t.Errorf("Title = %q", d.Title)
	}
	if !strings.Contains(d.Description, "Welcome to the Demo") {
		t.Errorf("Description should contain the fixture opening, got %.80q", d.Description)
	}
	if strings.Contains(d.Description, "<") {
		t.Errorf("Description should be plain text, got %.80q", d.Description)
	}
}

func TestLeverDetailNotFound(t *testing.T) {
	a := testLeverAdapter(t)
	if _, err := a.Detail(context.Background(), "leverdemo", lever.MockNotFoundPostingID); err == nil {
		t.Fatal("want error for unknown posting id")
	}
}
