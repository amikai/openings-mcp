# Default search: the newest 100 jobs (one full page), no parameters.
# Every hit carries the complete HTML description inline — there is no
# per-job detail endpoint (/api/jobs/<id> and variants 404), so detail
# lookup means finding the id in these search pages. Jobs reach the API
# 24 hours after publication; upstream refreshes every 10 minutes.
GET https://remotefirstjobs.com/api/search-jobs

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$._README_" isString
jsonpath "$.page" == 0
jsonpath "$.jobs_count" == 100
jsonpath "$.jobs" count == 100
jsonpath "$.jobs[0].id" isString
jsonpath "$.jobs[0].url" isString
jsonpath "$.jobs[0].company_name" isString
jsonpath "$.jobs[0].title" isString
jsonpath "$.jobs[0].category" isString
jsonpath "$.jobs[0].seniority" isString
jsonpath "$.jobs[0].description" isString
jsonpath "$.jobs[0].locations" isCollection
jsonpath "$.jobs[0].published_at" isString
