# Deep paging is capped: offset+limit may reach MaxResultWindow but not
# exceed it. openapi.yaml puts no maximum on offset, so the ceiling is
# only observable here.
GET https://freehire.me/api/v1/agent/jobs/search
[QueryStringParams]
q: golang
limit: 1
offset: 9999

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.meta.offset" == 9999

GET https://freehire.me/api/v1/agent/jobs/search
[QueryStringParams]
q: golang
limit: 100
offset: 9999

HTTP 400
[Asserts]
jsonpath "$.error" == "pagination too deep"

GET https://freehire.me/api/v1/agent/jobs/search
[QueryStringParams]
q: golang
limit: 1
offset: 10000

HTTP 400
[Asserts]
jsonpath "$.error" == "pagination too deep"
