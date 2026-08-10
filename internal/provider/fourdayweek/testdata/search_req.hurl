# Unfiltered board, and the baseline `total` that search_filtered_req.hurl
# narrows against.
GET https://4dayweek.io/api/v2/jobs
[QueryStringParams]
limit: 2

HTTP 200
[Asserts]
jsonpath "$.page" == 1
jsonpath "$.limit" == 2
jsonpath "$.data" count == 2
jsonpath "$.has_more" == true
jsonpath "$.total" > 1000
# work_arrangement is the only remoteness signal; the spec's is_remote is never sent.
jsonpath "$.data[0].work_arrangement" isString
jsonpath "$.data[0].is_remote" not exists
jsonpath "$.data[0].expires_at" not exists
