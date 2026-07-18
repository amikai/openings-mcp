package nodesk

import (
	"context"
	"strings"
	"testing"
	"time"
)

func testClient(t *testing.T) *Client {
	t.Helper()
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, srv.URL, srv.Client())
}

func TestClient_Search(t *testing.T) {
	c := testClient(t)
	res, err := c.Search(context.Background(), SearchOptions{Query: "golang"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res.Jobs) != 2 {
		t.Fatalf("got %d jobs, want 2 (the captured fixture's hit count)", len(res.Jobs))
	}
	if res.NbHits != 2 || res.NbPages != 1 {
		t.Errorf("NbHits/NbPages = %d/%d, want 2/1", res.NbHits, res.NbPages)
	}

	j := res.Jobs[0]
	if j.ID != "proxify-senior-golang-developer-fullstack-be-heavy" {
		t.Errorf("ID = %q, want the permalink slug", j.ID)
	}
	if j.ObjectID != "2f67e64031540e79ed83630faece1c53" {
		t.Errorf("ObjectID = %q", j.ObjectID)
	}
	if j.Title != "Senior Golang Developer (Fullstack, BE-Heavy)" {
		t.Errorf("Title = %q", j.Title)
	}
	if j.Company != "Proxify" {
		t.Errorf("Company = %q", j.Company)
	}
	if !strings.HasSuffix(j.URL, "/remote-jobs/proxify-senior-golang-developer-fullstack-be-heavy/") ||
		!strings.HasPrefix(j.URL, "http") {
		t.Errorf("URL = %q, want the absolute job page URL", j.URL)
	}
	if j.Role != "Engineering" {
		t.Errorf("Role = %q", j.Role)
	}
	if len(j.Types) != 2 || j.Types[0] != "Contract" || j.Types[1] != "Full-Time" {
		t.Errorf("Types = %v", j.Types)
	}
	if len(j.Keywords) == 0 || j.Keywords[len(j.Keywords)-1] != "Golang" {
		t.Errorf("Keywords = %v, want Golang last", j.Keywords)
	}
	if len(j.Regions) != 1 || j.Regions[0] != "Worldwide" {
		t.Errorf("Regions = %v", j.Regions)
	}
	if j.BaseSalary != "" {
		t.Errorf("BaseSalary = %q, want empty (fixture publishes literal false)", j.BaseSalary)
	}
	if j.DateLabel != "Featured" || !j.Featured {
		t.Errorf("DateLabel/Featured = %q/%v, want Featured/true", j.DateLabel, j.Featured)
	}
	want := time.Date(2026, 6, 19, 14, 15, 26, 0, time.UTC)
	if !j.PublishedAt.Equal(want) {
		t.Errorf("PublishedAt = %v, want %v", j.PublishedAt, want)
	}
	if j.LogoURL == "" || !strings.HasPrefix(j.LogoURL, "http") {
		t.Errorf("LogoURL = %q, want absolute", j.LogoURL)
	}
}

func TestClient_Search_emptyQueryDropsAdRecord(t *testing.T) {
	c := testClient(t)
	res, err := c.Search(context.Background(), SearchOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	// The captured first page has 20 hits, one of which is the injected
	// advertisement record.
	if len(res.Jobs) != 19 {
		t.Fatalf("got %d jobs, want 19 (20 hits minus the ad record)", len(res.Jobs))
	}
	for _, j := range res.Jobs {
		if j.Company == "NoGigiddy" {
			t.Errorf("the ad record leaked into results: %+v", j)
		}
		if j.ID == "" || strings.Contains(j.ID, "/") {
			t.Errorf("job ID %q is not a bare slug", j.ID)
		}
	}
	if res.NbHits != 186 || res.NbPages != 10 {
		t.Errorf("NbHits/NbPages = %d/%d, want the fixture's 186/10", res.NbHits, res.NbPages)
	}
}

func TestClient_Search_filtered(t *testing.T) {
	c := testClient(t)
	// The mock rejects any facetFilters encoding other than the one the
	// fixture was captured with, so this also pins the client's encoding.
	res, err := c.Search(context.Background(), SearchOptions{
		Filter: "remote-jobs/engineering",
		Region: "Remote - Europe",
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res.Jobs) != 10 {
		t.Fatalf("got %d jobs, want the fixture's 10", len(res.Jobs))
	}
	for _, j := range res.Jobs {
		found := false
		for _, r := range j.Regions {
			if r == "Remote - Europe" {
				found = true
			}
		}
		if !found {
			t.Errorf("job %q Regions = %v, want to include Remote - Europe", j.ID, j.Regions)
		}
	}
}

func TestClient_Search_noResults(t *testing.T) {
	c := testClient(t)
	res, err := c.Search(context.Background(), SearchOptions{Query: "qqqzzzxx"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res.Jobs) != 0 || res.NbHits != 0 {
		t.Fatalf("got %d jobs, NbHits %d, want 0/0", len(res.Jobs), res.NbHits)
	}
}

func TestClient_Facets(t *testing.T) {
	c := testClient(t)
	f, err := c.Facets(context.Background())
	if err != nil {
		t.Fatalf("Facets: %v", err)
	}
	if got := f.SearchFilters["remote-jobs/engineering"]; got != 62 {
		t.Errorf("SearchFilters[remote-jobs/engineering] = %d, want the fixture's 62", got)
	}
	if len(f.Regions) != 22 {
		t.Errorf("got %d regions, want the fixture's 22", len(f.Regions))
	}
}

func TestClient_Detail(t *testing.T) {
	c := testClient(t)
	d, err := c.Detail(context.Background(), "sticker-mule-software-engineer")
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if d.ID != "sticker-mule-software-engineer" {
		t.Errorf("ID = %q", d.ID)
	}
	if d.Title != "Software engineer" {
		t.Errorf("Title = %q", d.Title)
	}
	if d.Company != "Sticker Mule" {
		t.Errorf("Company = %q", d.Company)
	}
	if len(d.CompanyLinks) != 2 {
		t.Errorf("CompanyLinks = %v, want the fixture's 2 sameAs URLs", d.CompanyLinks)
	}
	if !strings.Contains(d.DescriptionHTML, "<p>") {
		t.Errorf("DescriptionHTML = %.60q…, want HTML", d.DescriptionHTML)
	}
	if len(d.Types) != 1 || d.Types[0] != "FULL_TIME" {
		t.Errorf("Types = %v", d.Types)
	}
	if d.LocationType != "TELECOMMUTE" {
		t.Errorf("LocationType = %q", d.LocationType)
	}
	if len(d.Locations) != 1 || d.Locations[0] != "Anywhere" {
		t.Errorf("Locations = %v", d.Locations)
	}
	if d.Salary == nil {
		t.Fatal("Salary is nil, want the fixture's range")
	}
	if d.Salary.Currency != "USD" || d.Salary.Min != 150000 || d.Salary.Max != 250000 || d.Salary.Unit != "YEAR" {
		t.Errorf("Salary = %+v", *d.Salary)
	}
	if d.DatePosted != time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC) {
		t.Errorf("DatePosted = %v", d.DatePosted)
	}
	if d.ValidThrough != time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC) {
		t.Errorf("ValidThrough = %v", d.ValidThrough)
	}
	if !strings.HasPrefix(d.ApplyURL, "https://jobs.ashbyhq.com/stickermule/") {
		t.Errorf("ApplyURL = %q, want the outbound employer link", d.ApplyURL)
	}
}

func TestClient_Detail_notFound(t *testing.T) {
	c := testClient(t)
	_, err := c.Detail(context.Background(), "no-such-job-slug")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("err = %v, want a not-found error", err)
	}
}
