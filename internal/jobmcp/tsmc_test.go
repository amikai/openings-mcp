package jobmcp

import (
	"net/http"
	"testing"

	"github.com/amikai/job-mcp/internal/provider/tsmc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRegisterTSMC(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)

	RegisterTSMC(server, tsmc.NewClient(tsmc.Config{HTTPClient: http.DefaultClient}))

	assertTools(t, server, "tsmc_search_jobs", "tsmc_get_job_detail")
}

func TestTSMCToRequest(t *testing.T) {
	in := tsmcSearchInput{
		Keyword:         "process engineer",
		Locations:       []string{"taiwan", "japan_osaka"},
		Categories:      []string{"rd"},
		JobTypes:        []string{"engineer"},
		EmploymentTypes: []string{"regular"},
		Page:            3,
	}
	got, err := tsmcToRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Keyword != "process engineer" || got.Page != 3 {
		t.Errorf("Keyword/Page = %q/%d", got.Keyword, got.Page)
	}
	wantLoc := []string{tsmc.LocTaiwan, tsmc.LocJapanOsaka}
	if len(got.Locations) != 2 || got.Locations[0] != wantLoc[0] || got.Locations[1] != wantLoc[1] {
		t.Errorf("Locations = %v, want %v", got.Locations, wantLoc)
	}
	if len(got.Categories) != 1 || got.Categories[0] != tsmc.CatRD {
		t.Errorf("Categories = %v, want [%s]", got.Categories, tsmc.CatRD)
	}
	if len(got.JobTypes) != 1 || got.JobTypes[0] != tsmc.JobTypeEngineer {
		t.Errorf("JobTypes = %v, want [%s]", got.JobTypes, tsmc.JobTypeEngineer)
	}
	if len(got.EmploymentTypes) != 1 || got.EmploymentTypes[0] != tsmc.EmployRegular {
		t.Errorf("EmploymentTypes = %v", got.EmploymentTypes)
	}
}

func TestTSMCToRequestInvalidLocation(t *testing.T) {
	_, err := tsmcToRequest(tsmcSearchInput{Keyword: "x", Locations: []string{"mars"}})
	if err == nil {
		t.Fatal("expected error for invalid location, got nil")
	}
}
