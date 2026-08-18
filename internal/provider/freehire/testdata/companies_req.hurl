# Company-slug resolver for freehire_search_jobs's company filter, which
# takes an exact slug. Matching is fuzzy despite the spec calling it a
# substring: "strpie" finds stripe and "altinty" finds altinity-inc.
# Results come back exact-match first, then by job_count descending.
GET https://freehire.me/api/v1/companies
[QueryStringParams]
q: adria
limit: 3

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.meta.total" isInteger
jsonpath "$.data" count == 3
# Two distinct companies share this name, which is why the tool returns a
# list rather than resolving to one slug.
jsonpath "$.data[0].slug" == "adria-solutions"
jsonpath "$.data[1].slug" == "adria-solutions-ltd"
jsonpath "$.data[0].job_count" isInteger
