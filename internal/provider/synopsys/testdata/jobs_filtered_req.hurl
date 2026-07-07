# Demonstrates the full documented query parameter surface (see
# ../openapi.yaml paths./search-jobs/results.get): a resolved location
# (Latitude/Longitude/LocationType/LocationPath — required together for
# location filtering to take effect) plus one active facet filter group
# (Category = Engineering). See jobs_req.hurl for the exact minimal request
# that captured jobs_rsp.json. Not wired to any committed fixture;
# illustrative only.
GET https://careers.synopsys.com/search-jobs/results
Accept: application/json
[QueryStringParams]
Keywords: verification engineer
Location: Hsinchu, Taiwan
Latitude: 24.80361
Longitude: 120.96861
LocationType: 4
LocationPath: 1668284-7280291-1675103-1675151
CurrentPage: 1
RecordsPerPage: 15
SortCriteria: 0
SortDirection: 0
Distance: 50
RadiusUnitType: 0
PostalCode: 95051
ShowRadius: False
IsPagination: False
SearchType: 5
ResultsType: 0
ActiveFacetID: 8675488
SearchResultsModuleName: Search Results
SearchFiltersModuleName: Search Filters
FacetFilters[0].ID: 8675488
FacetFilters[0].FacetType: 1
FacetFilters[0].Display: Engineering
FacetFilters[0].Count: 533
FacetFilters[0].IsApplied: true
FacetFilters[0].FieldName:

HTTP 200
