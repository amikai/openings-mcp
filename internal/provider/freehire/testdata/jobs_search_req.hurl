# Full-text search. limit=2 keeps the fixture small. This endpoint serves
# the search index's preview rather than the stored body: it is cut near
# 1000 characters on a word boundary WITHOUT closing the tags it opened,
# so a consumer has to run it through an HTML converter.
GET https://freehire.me/api/v1/jobs/search
User-Agent: amikai/openings-mcp (+https://github.com/amikai/openings-mcp)
[QueryStringParams]
q: golang
limit: 2

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.meta.limit" == 2
jsonpath "$.meta.offset" == 0
jsonpath "$.meta.total" isInteger
# Absent on a clean request rather than empty, so its presence is the signal.
jsonpath "$.meta.ignored_params" not exists
jsonpath "$.data" count == 2
jsonpath "$.data[0].public_slug" isString
jsonpath "$.data[0].title" isString
jsonpath "$.data[0].company" isString
jsonpath "$.data[0].company_slug" isString
jsonpath "$.data[0].url" startsWith "https://"
jsonpath "$.data[0].posted_at" isString
jsonpath "$.data[0].description" contains "<"
# reality rides on every posting; ghost is raised only when evidence fires
# and is null across the live catalogue today.
jsonpath "$.data[0].reality.class" isString
