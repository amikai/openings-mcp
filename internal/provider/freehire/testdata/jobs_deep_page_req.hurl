# Deep paging is capped: offset+limit may reach 10000 but not exceed it.
# openapi.yaml states the ceiling in the offset parameter's prose only —
# there is no maximum in the schema — so it stays observable only here.
GET https://freehire.me/api/v1/jobs/search
User-Agent: amikai/openings-mcp (+https://github.com/amikai/openings-mcp)
[QueryStringParams]
q: golang
limit: 1
offset: 9999

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.meta.offset" == 9999

GET https://freehire.me/api/v1/jobs/search
User-Agent: amikai/openings-mcp (+https://github.com/amikai/openings-mcp)
[QueryStringParams]
q: golang
limit: 100
offset: 9999

HTTP 400
[Asserts]
jsonpath "$.error" == "pagination too deep"

GET https://freehire.me/api/v1/jobs/search
User-Agent: amikai/openings-mcp (+https://github.com/amikai/openings-mcp)
[QueryStringParams]
q: golang
limit: 1
offset: 10000

HTTP 400
[Asserts]
jsonpath "$.error" == "pagination too deep"
