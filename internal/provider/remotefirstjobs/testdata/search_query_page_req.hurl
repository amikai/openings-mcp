# Full-text query and pagination compose: page 1 of the golang search
# shares zero ids with page 0 (verified at capture time). Pages run 0-4,
# 100 jobs each, so at most 500 jobs are reachable per query.
GET https://remotefirstjobs.com/api/search-jobs
[QueryStringParams]
query: golang
page: 1

HTTP 200
[Asserts]
jsonpath "$.page" == 1
jsonpath "$.jobs_count" == 100
jsonpath "$.jobs" count == 100
