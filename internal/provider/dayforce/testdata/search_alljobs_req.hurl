# mydayforce's "alljobs" board is not the default CANDIDATEPORTAL, and its
# jobBoardId is 8, not 1 — proves jobBoardId must be resolved per board.
GET https://jobs.dayforcehcm.com/api/auth/csrf
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36

HTTP 200
[Captures]
csrf_token: jsonpath "$.csrfToken"

POST https://jobs.dayforcehcm.com/api/geo/mydayforce/jobposting/search
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36
Content-Type: application/json
X-CSRF-TOKEN: {{csrf_token}}
{
  "clientNamespace": "mydayforce",
  "jobBoardCode": "alljobs",
  "cultureCode": "en-US",
  "paginationStart": 0
}

HTTP 200
[Asserts]
jsonpath "$.maxCount" isInteger
jsonpath "$.jobPostings" count > 0
jsonpath "$.jobPostings[0].jobBoardId" == 8
