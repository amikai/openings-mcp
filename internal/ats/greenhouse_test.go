package ats

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/amikai/openings-mcp/internal/provider/greenhouse"
)

// mockGreenhouseBoard is the board token the provider mock server serves a
// content=true dump for (5 hand-crafted jobs; see the provider's
// testdata/jobs_content_rsp.json).
const mockGreenhouseBoard = "safariai"

func testGreenhouseAdapter(t *testing.T) *GreenhouseAdapter {
	t.Helper()
	srv := greenhouse.NewMockServer()
	t.Cleanup(srv.Close)
	a, err := NewGreenhouseAdapter(srv.URL, &http.Client{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestGreenhouseRoster(t *testing.T) {
	a := testGreenhouseAdapter(t)
	if got, want := len(a.Roster()), len(greenhouse.Companies); got != want {
		t.Fatalf("roster len = %d, want %d", got, want)
	}
	for _, c := range a.Roster() {
		if c.Slug == "" || c.Name == "" {
			t.Fatalf("roster entry with empty field: %+v", c)
		}
	}
}

func TestGreenhouseSearchAll(t *testing.T) {
	a := testGreenhouseAdapter(t)
	res, err := a.Search(t.Context(), mockGreenhouseBoard, SearchParams{})
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalCount != 5 {
		t.Fatalf("TotalCount = %d, want 5", res.TotalCount)
	}
	if res.Jobs[0].JobID != "6100001004" {
		t.Errorf("newest job should sort first, got %+v", res.Jobs[0])
	}
	for _, j := range res.Jobs {
		if j.JobID == "" || j.Title == "" || j.URL == "" {
			t.Fatalf("summary with empty field: %+v", j)
		}
		if !strings.HasPrefix(j.PostedAt, "20") {
			t.Errorf("PostedAt should be an ISO date, got %q", j.PostedAt)
		}
	}
	// 5 jobs < PageSize, so page 2 is empty but the envelope stays sane.
	page2, err := a.Search(t.Context(), mockGreenhouseBoard, SearchParams{Page: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page2.Jobs) != 0 || page2.Page != 2 || page2.TotalPages != 1 {
		t.Errorf("page 2 = %+v, want empty page with TotalPages 1", page2)
	}
}

func TestGreenhouseSearchQueryRanksTitleFirst(t *testing.T) {
	a := testGreenhouseAdapter(t)
	res, err := a.Search(t.Context(), mockGreenhouseBoard, SearchParams{Query: "agent platform"})
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalCount != 2 {
		t.Fatalf("TotalCount = %d, want 2 (title hit + JD-body hit)", res.TotalCount)
	}
	if res.Jobs[0].JobID != "6100001002" || res.Jobs[1].JobID != "6100001004" {
		t.Fatalf("want title hit before JD-body hit, got %+v", res.Jobs)
	}
}

func TestGreenhouseSearchQueryMatchesJDBody(t *testing.T) {
	a := testGreenhouseAdapter(t)
	res, err := a.Search(t.Context(), mockGreenhouseBoard, SearchParams{Query: "kubernetes"})
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalCount != 1 || res.Jobs[0].Title != "Senior Backend Engineer" {
		t.Fatalf("query hitting only entity-encoded JD content should match, got %+v", res.Jobs)
	}
}

func TestGreenhouseSearchQueryMatchesOrgUnit(t *testing.T) {
	a := testGreenhouseAdapter(t)
	// "people" appears only in the Technical Recruiter job's department.
	res, err := a.Search(t.Context(), mockGreenhouseBoard, SearchParams{Query: "people"})
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalCount != 1 || res.Jobs[0].Title != "Technical Recruiter" {
		t.Fatalf("query hitting only the department name should match, got %+v", res.Jobs)
	}
}

func TestGreenhouseSearchLocation(t *testing.T) {
	a := testGreenhouseAdapter(t)
	remote, err := a.Search(t.Context(), mockGreenhouseBoard, SearchParams{Location: "remote"})
	if err != nil {
		t.Fatal(err)
	}
	if remote.TotalCount != 1 || remote.Jobs[0].Title != "Product Designer" {
		t.Fatalf(`Location "remote" should fall back to location-text match, got %+v`, remote.Jobs)
	}
	london, err := a.Search(t.Context(), mockGreenhouseBoard, SearchParams{Location: "london"})
	if err != nil {
		t.Fatal(err)
	}
	if london.TotalCount != 1 || london.Jobs[0].Title != "Data Scientist" {
		t.Fatalf(`Location "london" should fuzzy-match, got %+v`, london.Jobs)
	}
}

func TestGreenhouseSearchFilterDepartment(t *testing.T) {
	a := testGreenhouseAdapter(t)
	res, err := a.Search(t.Context(), mockGreenhouseBoard, SearchParams{
		Filters: map[string][]string{"department": {"Engineering"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalCount != 2 {
		t.Fatalf("Engineering filter: TotalCount = %d, want 2", res.TotalCount)
	}
}

func TestGreenhouseFilters(t *testing.T) {
	a := testGreenhouseAdapter(t)
	fs, err := a.Filters(t.Context(), mockGreenhouseBoard)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"department", "office"} {
		if len(fs[key]) == 0 {
			t.Errorf("FilterSet missing %q: %v", key, fs)
		}
	}
	wantDepts := []string{"Data", "Design", "Engineering", "People"}
	if got := strings.Join(fs["department"], ","); got != strings.Join(wantDepts, ",") {
		t.Errorf("department values = %v, want %v", fs["department"], wantDepts)
	}
}

func TestGreenhouseDetail(t *testing.T) {
	a := testGreenhouseAdapter(t)
	d, err := a.Detail(t.Context(), "anthropic", "4461450008")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(d.Title, "Account Executive") {
		t.Errorf("Title = %q", d.Title)
	}
	if d.Company != "Anthropic" {
		t.Errorf("Company = %q, want display name from the roster", d.Company)
	}
	if !strings.Contains(d.Description, "About Anthropic") {
		t.Errorf("Description should carry decoded JD text, got %.80q", d.Description)
	}
	if strings.Contains(d.Description, "&lt;") || strings.Contains(d.Description, "<div") {
		t.Errorf("Description should be plain text, got %.80q", d.Description)
	}
	if d.URL == "" || d.JobID != "4461450008" {
		t.Errorf("unexpected detail envelope: %+v", d)
	}
}

func TestGreenhouseDetailBadID(t *testing.T) {
	a := testGreenhouseAdapter(t)
	if _, err := a.Detail(t.Context(), "anthropic", "not-a-number"); err == nil {
		t.Fatal("want teaching error for non-numeric job id")
	}
}

func TestGreenhouseDetailNotFound(t *testing.T) {
	a := testGreenhouseAdapter(t)
	if _, err := a.Detail(t.Context(), "anthropic", "999999999999"); err == nil {
		t.Fatal("want error for unknown job id")
	}
}

func TestGreenhouseUnknownBoardUpstream(t *testing.T) {
	a := testGreenhouseAdapter(t)
	if _, err := a.Search(t.Context(), "doesnotexist", SearchParams{}); err == nil {
		t.Fatal("want error when upstream returns 404")
	}
}
