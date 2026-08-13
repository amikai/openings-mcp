GET https://jobs.dayforcehcm.com/api/auth/csrf
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36

HTTP 200
[Captures]
csrf_token: jsonpath "$.csrfToken"

POST https://jobs.dayforcehcm.com/api/geo/nosuchtenantxyz/jobposting/search
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36
Content-Type: application/json
X-CSRF-TOKEN: {{csrf_token}}
{
  "clientNamespace": "nosuchtenantxyz",
  "jobBoardCode": "CANDIDATEPORTAL",
  "cultureCode": "en-US",
  "paginationStart": 0
}

HTTP 404
[Asserts]
header "Content-Type" contains "application/problem+json"
jsonpath "$.status" == 404
jsonpath "$.traceId" exists
