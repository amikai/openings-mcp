package jobmcp

import (
	"net/http"
	"testing"

	"github.com/amikai/job-mcp/internal/provider/tsmc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterTSMC(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)

	RegisterTSMC(server, tsmc.NewClient(http.DefaultClient))

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
	require.NoError(t, err)

	want := &tsmc.JobsRequest{
		Keyword:         "process engineer",
		Locations:       []string{tsmc.LocTaiwan, tsmc.LocJapanOsaka},
		Categories:      []string{tsmc.CatRD},
		JobTypes:        []string{tsmc.JobTypeEngineer},
		EmploymentTypes: []string{tsmc.EmployRegular},
		Page:            3,
	}
	assert.Equal(t, want, got)
}

func TestTSMCToRequestInvalidLocation(t *testing.T) {
	_, err := tsmcToRequest(tsmcSearchInput{Keyword: "x", Locations: []string{"mars"}})
	if err == nil {
		t.Fatal("expected error for invalid location, got nil")
	}
}
