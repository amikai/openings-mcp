package eightfold

import (
	"testing"

	"github.com/go-faster/jx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMergedFacetsCombinesBothLists proves smartFilters and allFilters are
// concatenated, covering tenants (Eaton, Infineon, Qualcomm) that populate
// only allFilters and would otherwise expose no facets at all.
func TestMergedFacetsCombinesBothLists(t *testing.T) {
	fd := FilterDef{
		SmartFilters: []SmartFilter{
			{FilterName: "businessarea", Title: "Business Area", Options: []SmartFilterOption{{Label: "Technology", Value: "technology"}}},
		},
		AllFilters: []AllFilter{
			{FilterName: "country", Title: "Country", Options: []AllFilterOption{{Label: jx.Raw(`"Taiwan"`), Value: "Taiwan"}}},
		},
	}
	facets := MergedFacets(fd)
	names := make([]string, len(facets))
	for i, f := range facets {
		names[i] = f.FilterName
	}
	assert.ElementsMatch(t, []string{"businessarea", "country"}, names)
}

// TestMergedFacetsDropsNonStringLabels covers a bug found by hitting the
// live API (not mock fixtures): Eaton's and Qualcomm's latlong_non_remote
// facet sends an integer nearby-postings count as label, e.g.
// {"label": 72, "value": "47.49,19.02"} — decoding that straight into a
// string field failed the entire search response, not just that one
// facet. MergedFacets must drop the bad option without erroring, and keep
// any well-formed ones alongside it.
func TestMergedFacetsDropsNonStringLabels(t *testing.T) {
	fd := FilterDef{
		AllFilters: []AllFilter{
			{
				FilterName: "latlong_non_remote",
				Title:      "LatLong",
				Options: []AllFilterOption{
					{Label: jx.Raw("72"), Value: "47.4941076,19.0247849"},
					{Label: jx.Raw(`"United States of America"`), Value: "United States of America"},
				},
			},
		},
	}
	facets := MergedFacets(fd)
	require.Len(t, facets, 1)
	require.Len(t, facets[0].Options, 1)
	assert.Equal(t, "United States of America", facets[0].Options[0].Label)
}

// TestMergedFacetsAllNonStringLabelsDrop covers the case where every option
// in a facet has a non-string label (e.g. Eaton's latlong_non_remote,
// where all 662 observed options are integer counts) — the whole facet
// must disappear, not surface as an empty, unusable entry.
func TestMergedFacetsAllNonStringLabelsDrop(t *testing.T) {
	fd := FilterDef{
		AllFilters: []AllFilter{
			{
				FilterName: "latlong_non_remote",
				Title:      "LatLong",
				Options: []AllFilterOption{
					{Label: jx.Raw("72"), Value: "47.4941076,19.0247849"},
					{Label: jx.Raw("67"), Value: "18.508934,73.92591019999999"},
				},
			},
		},
	}
	assert.Empty(t, MergedFacets(fd))
}
