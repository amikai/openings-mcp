GET https://jobs.dayforcehcm.com/api/auth/csrf
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36

HTTP 200
[Captures]
csrf_token: jsonpath "$.csrfToken"
[Asserts]
cookie "__Host-next-auth.csrf-token" exists

POST https://jobs.dayforcehcm.com/api/geo/pca/jobposting/search
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36
Content-Type: application/json
X-CSRF-TOKEN: {{csrf_token}}
{
  "clientNamespace": "pca",
  "jobBoardCode": "CANDIDATEPORTAL",
  "cultureCode": "en-US",
  "paginationStart": 0
}

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.maxCount" isInteger
jsonpath "$.offset" == 0
jsonpath "$.count" == 25
jsonpath "$.jobPostings" count == 25
jsonpath "$.jobPostings[0].jobBoardId" exists
