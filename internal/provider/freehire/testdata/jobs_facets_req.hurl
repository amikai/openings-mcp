# Live filter vocabulary for q=golang. Call it before applying any open or
# generated vocabulary: skills, category, role, source, collections,
# countries, posting_language. A distribution is counted per facet and
# cities, role, and skills carry ~1200 values each, so the unnarrowed
# answer runs to thousands of values.
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
jsonpath "$.data.facets.reality" exists
jsonpath "$.data.facets.role" exists
jsonpath "$.data.facets.cities" exists
# company_slug has no distribution — searchCompanies is its typeahead.
jsonpath "$.data.facets.company_slug" not exists
# Numeric facets come back as ranges instead.
jsonpath "$.data.stats.salary_min.min" isNumber
jsonpath "$.data.stats.experience_years_min.max" isNumber
jsonpath "$.data.meta" not exists

# facets= narrows which distributions are counted.
GET https://freehire.me/api/v1/jobs/facets
[QueryStringParams]
q: golang
facets: skills,seniority

HTTP 200
[Asserts]
jsonpath "$.data.facets.skills" exists
jsonpath "$.data.facets.seniority" exists
jsonpath "$.data.facets.work_mode" not exists

# Without disjunctive, a facet you filtered on collapses to the one value
# you picked, which says nothing about what the alternatives would return.
GET https://freehire.me/api/v1/jobs/facets
[QueryStringParams]
seniority: senior

HTTP 200
[Asserts]
jsonpath "$.data.facets.seniority.senior" isInteger
jsonpath "$.data.facets.seniority.lead" not exists

# disjunctive counts each facet under the filter MINUS its own selection,
# so all eight seniority levels come back. It forces every facet to be
# counted, which is why it cannot be combined with facets=.
GET https://freehire.me/api/v1/jobs/facets
[QueryStringParams]
seniority: senior
disjunctive: true

HTTP 200
[Asserts]
jsonpath "$.data.facets.seniority.senior" isInteger
jsonpath "$.data.facets.seniority.lead" isInteger
jsonpath "$.data.facets.seniority.intern" isInteger

GET https://freehire.me/api/v1/jobs/facets
[QueryStringParams]
seniority: senior
facets: skills
disjunctive: true

HTTP 400
[Asserts]
jsonpath "$.error" == "facets= cannot be combined with disjunctive"
