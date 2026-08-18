# Live filter vocabulary for q=golang. Call before applying open-vocabulary
# filters such as skills, countries, source, or category.
GET https://freehire.me/api/v1/jobs/facets
[QueryStringParams]
q: golang

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.data.total" isInteger
jsonpath "$.data.facets.work_mode" exists
jsonpath "$.data.facets.seniority" exists
jsonpath "$.data.facets.skills" exists
