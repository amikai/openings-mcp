package avature

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSearch(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL+"/careers", nil)

	res, err := c.Search(context.Background(), nil)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res.Jobs) != 12 {
		t.Fatalf("got %d jobs, want 12", len(res.Jobs))
	}
	if res.Total != 436 {
		t.Errorf("Total = %d, want 436", res.Total)
	}
	if !res.HasNext {
		t.Error("HasNext = false, want true")
	}
	first := res.Jobs[0]
	if first.ID != "20873" {
		t.Errorf("first job ID = %q, want 20873", first.ID)
	}
	if want := "Enterprise Services - FXGO Tradedesk, Client Services Specialist - Singapore"; first.Title != want {
		t.Errorf("first job Title = %q, want %q", first.Title, want)
	}
	if first.Location != "Singapore, Singapore" {
		t.Errorf("first job Location = %q, want Singapore, Singapore", first.Location)
	}
	if !strings.Contains(first.URL, "/JobDetail/") {
		t.Errorf("first job URL = %q, want a JobDetail link", first.URL)
	}
}

func TestSearchFiltered(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL+"/careers", nil)

	res, err := c.Search(context.Background(), &SearchRequest{Search: "engineer"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res.Total != 281 {
		t.Errorf("Total = %d, want 281", res.Total)
	}
	if len(res.Jobs) != 12 {
		t.Errorf("got %d jobs, want 12", len(res.Jobs))
	}
	if res.Jobs[0].ID != "18312" {
		t.Errorf("first job ID = %q, want 18312", res.Jobs[0].ID)
	}
}

func TestSearchOffset(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL+"/careers", nil)

	res, err := c.Search(context.Background(), &SearchRequest{Offset: 12})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res.Total != 436 {
		t.Errorf("Total = %d, want 436", res.Total)
	}
	if res.Jobs[0].ID != "20826" {
		t.Errorf("first job ID = %q, want 20826", res.Jobs[0].ID)
	}
}

func TestSearchNoResults(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL+"/careers", nil)

	res, err := c.Search(context.Background(), &SearchRequest{Search: "zzzznonexistentkeyword12345"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res.Jobs) != 0 {
		t.Errorf("got %d jobs, want 0", len(res.Jobs))
	}
	// The zero-result legend has empty text but keeps aria-label="0 results".
	if res.Total != 0 {
		t.Errorf("Total = %d, want 0", res.Total)
	}
	if res.HasNext {
		t.Error("HasNext = true, want false")
	}
}

// TestSearchNoLegend covers portals that hide the results legend (Koch):
// Total is unknowable from one page and reported as -1.
func TestSearchNoLegend(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL+"/nolegend", nil)

	res, err := c.Search(context.Background(), nil)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res.Jobs) != 6 {
		t.Fatalf("got %d jobs, want 6", len(res.Jobs))
	}
	if res.Total != -1 {
		t.Errorf("Total = %d, want -1", res.Total)
	}
	if !res.HasNext {
		t.Error("HasNext = false, want true")
	}
	first := res.Jobs[0]
	if first.ID != "189618" {
		t.Errorf("first job ID = %q, want 189618", first.ID)
	}
	// Koch renders location as a label/value card field.
	if first.Location != "Atlanta, Georgia" {
		t.Errorf("first job Location = %q, want Atlanta, Georgia", first.Location)
	}
}

// TestSearchJobsTheme covers the article--jobs theme (Avature's own portal)
// where location is tagged with an address icon in the item footer.
func TestSearchJobsTheme(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL+"/jobs-theme", nil)

	res, err := c.Search(context.Background(), nil)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res.Jobs) != 6 {
		t.Fatalf("got %d jobs, want 6", len(res.Jobs))
	}
	if res.Total != 27 {
		t.Errorf("Total = %d, want 27", res.Total)
	}
	first := res.Jobs[0]
	if first.ID != "9173" {
		t.Errorf("first job ID = %q, want 9173", first.ID)
	}
	if first.Location != "Argentina" {
		t.Errorf("first job Location = %q, want Argentina", first.Location)
	}
}

func TestJobDetail(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL+"/careers", nil)

	d, err := c.JobDetail(context.Background(), "20873")
	if err != nil {
		t.Fatalf("JobDetail: %v", err)
	}
	if want := "Enterprise Services - FXGO Tradedesk, Client Services Specialist - Singapore"; d.Title != want {
		t.Errorf("Title = %q, want %q", d.Title, want)
	}
	if d.Location() != "Singapore" {
		t.Errorf("Location() = %q, want Singapore", d.Location())
	}
	var refFound bool
	for _, f := range d.Fields {
		if f.Label == "Ref #" && f.Value == "10052667" {
			refFound = true
		}
	}
	if !refFound {
		t.Errorf("Fields = %v, want a Ref # 10052667 entry", d.Fields)
	}
	if !strings.Contains(d.DescriptionHTML, "FXGO") {
		t.Error("DescriptionHTML does not mention FXGO")
	}
	if want := srv.URL + "/careers/JobDetail/job/20873"; d.URL != want {
		t.Errorf("URL = %q, want %q", d.URL, want)
	}
}

// TestJobDetailFieldsTheme covers a portal (Koch) whose description sits
// directly in a details section without a rich-text field wrapper, with
// tenant-specific labels like "Location(s)" and a brand "Company" field.
func TestJobDetailFieldsTheme(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL+"/careers", nil)

	d, err := c.JobDetail(context.Background(), "161128")
	if err != nil {
		t.Fatalf("JobDetail: %v", err)
	}
	if want := "Operational Technology (OT) Security Engineer"; d.Title != want {
		t.Errorf("Title = %q, want %q", d.Title, want)
	}
	if d.Location() != "Zhuhai, Guangdong" {
		t.Errorf("Location() = %q, want Zhuhai, Guangdong", d.Location())
	}
	if d.Company() != "Molex" {
		t.Errorf("Company() = %q, want Molex", d.Company())
	}
	if !strings.Contains(d.DescriptionHTML, "OT Security Engineer") {
		t.Error("DescriptionHTML does not contain the role text")
	}
	// Metadata removed from the description must still be in Fields only.
	if strings.Contains(d.DescriptionHTML, "Career Field") {
		t.Error("DescriptionHTML still contains the Career Field metadata")
	}
}

func TestJobDetailNotFound(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL+"/careers", nil)

	_, err := c.JobDetail(context.Background(), "999999999")
	if !errors.Is(err, ErrJobNotFound) {
		t.Fatalf("err = %v, want ErrJobNotFound", err)
	}
}

func TestJobDetailRejectsNonNumericID(t *testing.T) {
	c := NewClient("https://example.avature.net/careers", nil)
	if _, err := c.JobDetail(context.Background(), "abc"); err == nil {
		t.Fatal("want error for non-numeric id")
	}
	if _, err := c.JobDetail(context.Background(), ""); err == nil {
		t.Fatal("want error for empty id")
	}
}
