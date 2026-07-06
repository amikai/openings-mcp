package openingsmcp

import (
	"context"
	"strings"
	"testing"

	"github.com/amikai/openings-mcp/internal/ats"
)

// stubAdapter returns canned results so tests exercise only the MCP
// translation layer.
type stubAdapter struct {
	searchResult *ats.SearchResult
	filterSet    ats.FilterSet
	detail       *ats.JobDetail
	gotParams    ats.SearchParams
}

func (s *stubAdapter) Name() string { return "stub" }
func (s *stubAdapter) Roster() []ats.CompanyInfo {
	return []ats.CompanyInfo{{Slug: "acme", Name: "Acme Corp"}}
}
func (s *stubAdapter) Search(_ context.Context, _ string, p ats.SearchParams) (*ats.SearchResult, error) {
	s.gotParams = p
	return s.searchResult, nil
}
func (s *stubAdapter) Filters(context.Context, string) (ats.FilterSet, error) {
	return s.filterSet, nil
}
func (s *stubAdapter) Detail(context.Context, string, string) (*ats.JobDetail, error) {
	return s.detail, nil
}

func testCompanyRegistry(t *testing.T, stub *stubAdapter) *ats.Registry {
	t.Helper()
	r, err := ats.NewRegistry(stub)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestCompanySearchMapsParamsAndResult(t *testing.T) {
	stub := &stubAdapter{searchResult: &ats.SearchResult{
		Jobs: []ats.JobSummary{{
			JobID: "j1", Title: "Engineer", Location: "Taipei", PostedAt: "2026-07-01", URL: "https://x/j1",
		}},
		TotalCount: 41, Page: 2, TotalPages: 3,
	}}
	reg := testCompanyRegistry(t, stub)

	out, err := companySearch(context.Background(), reg, &companySearchInput{
		Company:  "Acme Corp",
		Query:    "golang",
		Location: "taipei",
		Filters:  map[string][]string{"team": {"Platform"}},
		Page:     2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if stub.gotParams.Query != "golang" || stub.gotParams.Page != 2 || stub.gotParams.Filters["team"][0] != "Platform" {
		t.Errorf("params not forwarded: %+v", stub.gotParams)
	}
	if out.TotalCount != 41 || out.Page != 2 || out.TotalPages != 3 || len(out.Data) != 1 {
		t.Errorf("result not mapped: %+v", out)
	}
	if out.Data[0].JobID != "j1" || out.Data[0].URL != "https://x/j1" {
		t.Errorf("summary not mapped: %+v", out.Data[0])
	}
}

func TestCompanySearchUnknownCompanyTeaches(t *testing.T) {
	reg := testCompanyRegistry(t, &stubAdapter{})
	_, err := companySearch(context.Background(), reg, &companySearchInput{Company: "acme corp intl"})
	if err == nil {
		t.Fatal("want teaching error")
	}
	if !strings.Contains(err.Error(), "acme") {
		t.Errorf("error should suggest acme, got: %v", err)
	}
}

func TestCompanyFilters(t *testing.T) {
	stub := &stubAdapter{filterSet: ats.FilterSet{"team": {"ML", "Web"}}}
	reg := testCompanyRegistry(t, stub)
	out, err := companyFilters(context.Background(), reg, &companyFiltersInput{Company: "acme"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Filters["team"]) != 2 {
		t.Errorf("filters not mapped: %+v", out)
	}
}

func TestCompanyDetail(t *testing.T) {
	stub := &stubAdapter{detail: &ats.JobDetail{JobID: "j1", Title: "Engineer", Company: "Acme Corp", Description: "plain text"}}
	reg := testCompanyRegistry(t, stub)
	out, err := companyDetail(context.Background(), reg, &companyDetailInput{Company: "acme", JobID: "j1"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Title != "Engineer" || out.Description != "plain text" {
		t.Errorf("detail not mapped: %+v", out)
	}
}
