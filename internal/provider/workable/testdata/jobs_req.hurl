POST https://apply.workable.com/api/v3/accounts/blueground/jobs
Content-Type: application/json
{
  "query": ""
}

HTTP 200
[Captures]
next_token: jsonpath "$.nextPage"
[Asserts]
jsonpath "$.total" isInteger
jsonpath "$.results" count == 10

# Pagination is a cursor: pass the previous response's nextPage back in the
# body as "token". The last page omits nextPage entirely. Page size is a
# fixed 10; a "limit" body field is rejected with "Not allowed".
POST https://apply.workable.com/api/v3/accounts/blueground/jobs
Content-Type: application/json
{
  "query": "",
  "token": "{{next_token}}"
}

HTTP 200
[Asserts]
jsonpath "$.total" isInteger
jsonpath "$.results" count > 0
