# Server-side company_slug filter. Stripe is a known catalogue slug; the
# apply URL stays on the employer's ATS (here Greenhouse).
GET https://freehire.me/api/v1/jobs/search
User-Agent: amikai/openings-mcp (+https://github.com/amikai/openings-mcp)
[QueryStringParams]
company_slug: stripe
limit: 2

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.meta.limit" == 2
jsonpath "$.meta.total" isInteger
jsonpath "$.data" count == 2
jsonpath "$.data[0].company_slug" == "stripe"
jsonpath "$.data[0].public_slug" isString
jsonpath "$.data[0].source" isString
