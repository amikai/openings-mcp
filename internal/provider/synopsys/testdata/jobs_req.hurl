# Filter/paginate job results (AJAX) — the exact request that captured
# jobs_rsp.json (RecordsPerPage: 2). See ../openapi.yaml
# paths./search-jobs/results.get for the full documented query parameter
# surface (resolved-location filtering, facet filter groups, etc.).
GET https://careers.synopsys.com/search-jobs/results
Accept: application/json
[QueryStringParams]
Keywords: software engineer
CurrentPage: 1
RecordsPerPage: 2
SortCriteria: 0
SortDirection: 0
Distance: 50
RadiusUnitType: 0
ShowRadius: False
IsPagination: False
SearchType: 5
ResultsType: 0
SearchResultsModuleName: Search Results
SearchFiltersModuleName: Search Filters

HTTP 200
