package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	eightfold "github.com/amikai/openings-mcp/internal/provider/eightfold"
)

// TestPrintFiltersAllFiltersOnlyTenant is a regression test for a tenant
// (e.g. Eaton, Infineon, Qualcomm) that populates filterDef.allFilters but
// leaves smartFilters empty: the 'eightfold filters' subcommand used to
// read only res.Data.FilterDef.SmartFilters directly and print an empty
// "null" result for these tenants, even though internal/ats's unified
// adapter (via eightfold.MergedFacets) discovered facets fine.
func TestPrintFiltersAllFiltersOnlyTenant(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/pcsx/search", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"status": 200,
			"data": {
				"positions": [],
				"count": 0,
				"filterDef": {
					"smartFilters": [],
					"allFilters": [
						{"filterName": "work_type", "title": "Work Type", "options": [
							{"label": "Onsite", "value": "onsite"},
							{"label": "Remote", "value": "remote"}
						]},
						{"filterName": "latlong_non_remote", "title": "LatLong", "options": [
							{"label": 72, "value": "47.4941076,19.0247849"}
						]}
					]
				}
			}
		}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client, err := eightfold.NewClient(srv.URL, eightfold.WithClient(srv.Client()))
	require.NoError(t, err)

	var buf bytes.Buffer
	err = printFilters(t.Context(), client, "eaton.com", "json", &buf)
	require.NoError(t, err)

	require.NotEqual(t, "null\n", buf.String(), "allFilters-only tenant should not report zero filters")

	var out []struct {
		Name   string   `json:"name"`
		Title  string   `json:"title"`
		Values []string `json:"values"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out))
	require.Len(t, out, 1, "the latlong facet's non-string label should be dropped, not surfaced")
	assert.Equal(t, "work_type", out[0].Name)
	assert.ElementsMatch(t, []string{"onsite", "remote"}, out[0].Values)
}
