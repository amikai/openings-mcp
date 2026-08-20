# Company-slug resolver for the company_slug job filter, which takes an
# exact slug. Matching is fuzzy despite the spec calling it a substring:
# "strpie" finds stripe and "altinty" finds altinity-inc. Results come
# back exact-match first, then by job_count descending.
GET https://freehire.me/api/v1/companies
User-Agent: amikai/openings-mcp (+https://github.com/amikai/openings-mcp)
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

# The curated company facets filter without a q. yc_flags is one of the
# four whose vocabulary no facets endpoint covers.
GET https://freehire.me/api/v1/companies
User-Agent: amikai/openings-mcp (+https://github.com/amikai/openings-mcp)
[QueryStringParams]
collections: yc
yc_flags: top_company
limit: 2

HTTP 200
[Asserts]
jsonpath "$.meta.total" isInteger
jsonpath "$.meta.ignored_params" not exists
jsonpath "$.data" count == 2
jsonpath "$.data[0].collections" contains "yc"

# Company search checks against its OWN vocabulary, so a real JOB facet
# sent here is dropped and reported.
GET https://freehire.me/api/v1/companies
User-Agent: amikai/openings-mcp (+https://github.com/amikai/openings-mcp)
[QueryStringParams]
seniority: senior
limit: 1

HTTP 200
[Asserts]
jsonpath "$.meta.ignored_params" count == 1
jsonpath "$.meta.ignored_params[0].param" == "seniority"
