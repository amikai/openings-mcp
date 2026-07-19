# Zero hits: still HTTP 200, but jobs is null — not an empty array —
# with jobs_count 0. The generated schema marks jobs nullable for this.
GET https://remotefirstjobs.com/api/search-jobs
[QueryStringParams]
query: zzzzqqqqxxxx

HTTP 200
[Asserts]
jsonpath "$.jobs_count" == 0
jsonpath "$.jobs" == null
