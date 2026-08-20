# An unknown company_slug is a successful empty page, not an error — a
# display name such as "Altinity, Inc." lands here rather than erroring.
# Only a bad parameter NAME is reported; a bad VALUE is silent.
GET https://freehire.me/api/v1/jobs/search
User-Agent: amikai/openings-mcp (+https://github.com/amikai/openings-mcp)
[QueryStringParams]
company_slug: definitely-not-a-real-company-zzzz
limit: 1

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.meta.total" == 0
jsonpath "$.meta.ignored_params" not exists
jsonpath "$.data" count == 0
