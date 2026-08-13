# Proves paginationStart is a real item offset: page 1 and page 2 return
# disjoint jobPostingId sets, and the API echoes paginationStart back as
# offset.
GET https://jobs.dayforcehcm.com/api/auth/csrf
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36

HTTP 200
[Captures]
csrf_token: jsonpath "$.csrfToken"

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
[Captures]
page1_first_id: jsonpath "$.jobPostings[0].jobPostingId"

POST https://jobs.dayforcehcm.com/api/geo/pca/jobposting/search
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36
Content-Type: application/json
X-CSRF-TOKEN: {{csrf_token}}
{
  "clientNamespace": "pca",
  "jobBoardCode": "CANDIDATEPORTAL",
  "cultureCode": "en-US",
  "paginationStart": 25
}

HTTP 200
[Asserts]
jsonpath "$.offset" == 25
jsonpath "$.count" == 25
jsonpath "$.jobPostings" count == 25
jsonpath "$.jobPostings[0].jobPostingId" != {{page1_first_id}}
