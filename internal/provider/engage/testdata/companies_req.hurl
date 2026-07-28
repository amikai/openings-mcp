# Pagination is a p_t/f_t stitch, not offset-based (see API.md): page=1
# needs f_t=0 but no meaningful p_t, while page=2 requires p_t to equal the
# exact totalCount observed on page 1 — a hardcoded p_t would break as
# totalCount drifts, so this captures it from page 1 and templates it into
# page 2.
GET https://en-gage.net/user/api/search/result_work_list/
[QueryStringParams]
p_t: 0
f_t: 0
page: 1
user-agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36
accept: application/json

HTTP 200
[Captures]
total: jsonpath "$.totalCount"
[Asserts]
jsonpath "$.result" == "success"
jsonpath "$.totalCount" isInteger
jsonpath "$.searchResult" count == 60
jsonpath "$.searchResult[0].company_url_root_dir" exists
jsonpath "$.searchResult[0].work_id" isInteger

GET https://en-gage.net/user/api/search/result_work_list/
[QueryStringParams]
p_t: {{total}}
f_t: 0
page: 2
user-agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36
accept: application/json

HTTP 200
[Asserts]
jsonpath "$.result" == "success"
jsonpath "$.searchResult" count == 60
jsonpath "$.fromCnt" == 61
