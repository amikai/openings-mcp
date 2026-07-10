package ats

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/amikai/openings-mcp/internal/provider/workday"
)

// recordingProxy forwards every request to inner and keeps the bodies, so
// tests can assert how many upstream calls a Search made and what the real
// (second) search request contained.
func recordingProxy(t *testing.T, inner string) (*httptest.Server, *[][]byte) {
	t.Helper()
	var bodies [][]byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodies = append(bodies, body)
		req, err := http.NewRequestWithContext(r.Context(), r.Method, inner+r.URL.Path, strings.NewReader(string(body)))
		if err != nil {
			t.Errorf("proxy: %v", err)
			return
		}
		req.Header = r.Header.Clone()
		rsp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Errorf("proxy: %v", err)
			return
		}
		defer rsp.Body.Close()
		w.Header().Set("Content-Type", rsp.Header.Get("Content-Type"))
		w.WriteHeader(rsp.StatusCode)
		io.Copy(w, rsp.Body)
	}))
	t.Cleanup(srv.Close)
	return srv, &bodies
}

func testWorkdayAdapter(t *testing.T) (*WorkdayAdapter, *[][]byte) {
	t.Helper()
	mock := workday.NewMockServer(workday.MockNvidiaJobsRsp, workday.MockNvidiaJobDetailRsp)
	t.Cleanup(mock.Close)
	proxy, bodies := recordingProxy(t, mock.URL)
	a := NewWorkdayAdapter(&http.Client{Timeout: 5 * time.Second})
	a.baseURL = func(workday.Company) string { return proxy.URL }
	return a, bodies
}

func TestWorkdayRosterDedupesShareClasses(t *testing.T) {
	a := NewWorkdayAdapter(http.DefaultClient)
	seen := map[string]bool{}
	for _, c := range a.Roster() {
		if seen[c.Slug] {
			t.Fatalf("duplicate slug %q in roster", c.Slug)
		}
		seen[c.Slug] = true
	}
	// fox and dowjones each occupy two share-class rows in companies.yaml
	// sharing one tenant; the roster must carry each slug once.
	if !seen["fox"] || !seen["dowjones"] {
		t.Fatal("expected fox and dowjones slugs present exactly once")
	}
}

func TestWorkdaySearchPlainIsOneRequest(t *testing.T) {
	a, bodies := testWorkdayAdapter(t)
	res, err := a.Search(t.Context(), "nvidia", SearchParams{Query: "golang"})
	if err != nil {
		t.Fatal(err)
	}
	if len(*bodies) != 1 {
		t.Fatalf("plain search should be 1 upstream request, got %d", len(*bodies))
	}
	if res.TotalCount != 27 || res.TotalPages != 2 || len(res.Jobs) != 20 {
		t.Fatalf("got {total %d, pages %d, len %d}, want {27, 2, 20}", res.TotalCount, res.TotalPages, len(res.Jobs))
	}
	first := res.Jobs[0]
	if first.Title != "Software Golang Kubernetes Engineer" {
		t.Errorf("Title = %q", first.Title)
	}
	if first.JobID != "/job/Israel-Tel-Aviv/Software-Golang-Kubernetes-Engineer_JR2020442" {
		t.Errorf("JobID = %q", first.JobID)
	}
}

func TestWorkdaySearchWithFiltersIsTwoRequests(t *testing.T) {
	a, bodies := testWorkdayAdapter(t)
	_, err := a.Search(t.Context(), "nvidia", SearchParams{
		Filters: map[string][]string{"timeType": {"Full time"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(*bodies) != 2 {
		t.Fatalf("filtered search should probe then search (2 requests), got %d", len(*bodies))
	}
	var real struct {
		AppliedFacets map[string][]string `json:"appliedFacets"`
	}
	if err := json.Unmarshal((*bodies)[1], &real); err != nil {
		t.Fatal(err)
	}
	if got := real.AppliedFacets["timeType"]; len(got) != 1 || got[0] != "5509c0b5959810ac0029943377d47364" {
		t.Fatalf("appliedFacets[timeType] = %v, want the Full time GUID", got)
	}
}

func TestWorkdaySearchLocationResolvesToFacet(t *testing.T) {
	a, bodies := testWorkdayAdapter(t)
	_, err := a.Search(t.Context(), "nvidia", SearchParams{Location: "Tel Aviv"})
	if err != nil {
		t.Fatal(err)
	}
	var real struct {
		AppliedFacets map[string][]string `json:"appliedFacets"`
	}
	if err := json.Unmarshal((*bodies)[1], &real); err != nil {
		t.Fatal(err)
	}
	if got := real.AppliedFacets["locations"]; len(got) != 1 || got[0] != "c7769ee377291036b08490819096b8bf" {
		t.Fatalf(`appliedFacets[locations] = %v, want the "Israel, Tel Aviv" GUID`, got)
	}
}

func TestWorkdayFilterValueNotFoundTeaches(t *testing.T) {
	a, _ := testWorkdayAdapter(t)
	_, err := a.Search(t.Context(), "nvidia", SearchParams{
		Filters: map[string][]string{"timeType": {"Part time"}},
	})
	if err == nil {
		t.Fatal("want error for unknown facet value")
	}
	if !strings.Contains(err.Error(), "Full time") {
		t.Errorf("error should list available values, got: %v", err)
	}
}

func TestWorkdayFilterKeyNotFoundTeaches(t *testing.T) {
	a, _ := testWorkdayAdapter(t)
	_, err := a.Search(t.Context(), "nvidia", SearchParams{
		Filters: map[string][]string{"bogus": {"x"}},
	})
	if err == nil {
		t.Fatal("want error for unknown facet key")
	}
	if !strings.Contains(err.Error(), "jobFamilyGroup") {
		t.Errorf("error should list valid keys, got: %v", err)
	}
}

func TestWorkdayFilters(t *testing.T) {
	a, _ := testWorkdayAdapter(t)
	fs, err := a.Filters(t.Context(), "nvidia")
	if err != nil {
		t.Fatal(err)
	}
	if len(fs["jobFamilyGroup"]) == 0 || len(fs["timeType"]) == 0 {
		t.Fatalf("FilterSet missing expected dimensions: %v", fs)
	}
	if !slices.Contains(fs["jobFamilyGroup"], "Engineering") {
		t.Errorf(`fs["jobFamilyGroup"] = %v, want it to contain "Engineering"`, fs["jobFamilyGroup"])
	}
}

func TestWorkdayDetail(t *testing.T) {
	a, _ := testWorkdayAdapter(t)
	d, err := a.Detail(t.Context(), "nvidia", "/job/Israel-Tel-Aviv/Software-Golang-Kubernetes-Engineer_JR2020442")
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "Senior Software Golang Kubernetes Engineer" {
		t.Errorf("Title = %q", d.Title)
	}
	if strings.Contains(d.Description, "<p>") {
		t.Errorf("Description should be converted from HTML, got %.80q", d.Description)
	}
	if !strings.Contains(d.Description, "NVIDIA Networking") {
		t.Errorf("Description should carry the fixture text, got %.80q", d.Description)
	}
}

func TestWorkdayDetailRejectsMalformedJobID(t *testing.T) {
	a, _ := testWorkdayAdapter(t)
	if _, err := a.Detail(t.Context(), "nvidia", "garbage"); err == nil {
		t.Fatal("want error for malformed job_id")
	}
}
