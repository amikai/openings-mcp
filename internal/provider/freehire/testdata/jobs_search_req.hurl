# Full-text search via the official ChatGPT Actions path. limit=2 keeps
# the fixture small. Unknown filters return 200 with total=0, not 400.
# Every row carries its full description whatever description_format
# says; text is the cheapest of the three.
GET https://freehire.me/api/v1/agent/jobs/search
[QueryStringParams]
q: golang
limit: 2
description_format: text

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.meta.limit" == 2
jsonpath "$.meta.offset" == 0
jsonpath "$.meta.total" isInteger
jsonpath "$.data" count == 2
jsonpath "$.data[0].public_slug" isString
jsonpath "$.data[0].title" isString
jsonpath "$.data[0].company" isString
jsonpath "$.data[0].company_slug" isString
jsonpath "$.data[0].url" startsWith "https://"
jsonpath "$.data[0].posted_at" isString
jsonpath "$.data[0].description" not contains "<p>"
