#!/bin/bash
BASE="https://careers.synopsys.com"
curl -s \
  -H "Accept: application/json" \
  "$BASE/search-jobs/results?Keywords=software+engineer&CurrentPage=1&RecordsPerPage=2&SortCriteria=0&SortDirection=0&Distance=50&RadiusUnitType=0&ShowRadius=False&IsPagination=False&SearchType=5&ResultsType=0&SearchResultsModuleName=Search+Results&SearchFiltersModuleName=Search+Filters" \
  > search_jobs_rsp.json
