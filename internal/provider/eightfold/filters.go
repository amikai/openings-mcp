package eightfold

import "encoding/json"

// MergedFacets concatenates FilterDef.SmartFilters and FilterDef.AllFilters
// into one facet list, so every caller (the unified adapter in internal/ats
// and the debug CLI in cmd/eightfold) discovers the same facets the same
// way. AllFilters is a second, usually-disjoint facet list some tenants
// (observed: Eaton, Infineon, Qualcomm) populate instead of smartFilters —
// normalized here to the SmartFilter shape.
func MergedFacets(fd FilterDef) []SmartFilter {
	facets := make([]SmartFilter, 0, len(fd.SmartFilters)+len(fd.AllFilters))
	facets = append(facets, fd.SmartFilters...)
	for _, af := range fd.AllFilters {
		if sf, ok := normalizeAllFilter(af); ok {
			facets = append(facets, sf)
		}
	}
	return facets
}

// normalizeAllFilter converts one allFilters entry to the SmartFilter shape,
// dropping any option whose label isn't a JSON string. Live-verified on
// 2026-07-14: Eaton's and Qualcomm's "latlong" facets (latlong_non_remote)
// send an integer nearby-postings count as label instead of a name — not a
// pickable filter value, and AllFilterOption leaves label untyped
// specifically so decoding the rest of the response doesn't fail because
// of it. Reports ok=false when every option was dropped this way (or
// options was null to begin with), same as SmartFilter's null-options case.
func normalizeAllFilter(af AllFilter) (SmartFilter, bool) {
	if len(af.Options) == 0 {
		return SmartFilter{}, false
	}
	opts := make([]SmartFilterOption, 0, len(af.Options))
	for _, o := range af.Options {
		var label string
		if err := json.Unmarshal(o.Label, &label); err != nil {
			continue
		}
		opts = append(opts, SmartFilterOption{Label: label, Value: o.Value})
	}
	if len(opts) == 0 {
		return SmartFilter{}, false
	}
	return SmartFilter{FilterName: af.FilterName, Title: af.Title, Options: opts}, true
}
