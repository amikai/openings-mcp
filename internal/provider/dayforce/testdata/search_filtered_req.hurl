# Proves searchText is a real server-side filter: pca's board is ~350
# postings, and this narrows to ~115 (see search_req.hurl for the base count).
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
  "searchText": "electrical engineer",
  "paginationStart": 0
}

HTTP 200
[Asserts]
jsonpath "$.maxCount" isInteger
jsonpath "$.maxCount" < 300
jsonpath "$.jobPostings" count > 0
