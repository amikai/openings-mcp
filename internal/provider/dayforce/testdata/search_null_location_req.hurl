# mymilacron's first search page includes jobPostingId 2640 ("Supply Chain
# Manager (Europe)"), a continent-level posting location where
# isoCountryCode, stateCode, and cityName are all null at once — the crash
# that motivated widening isoCountryCode to nullable.
GET https://jobs.dayforcehcm.com/api/auth/csrf
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36

HTTP 200
[Captures]
csrf_token: jsonpath "$.csrfToken"
[Asserts]
cookie "__Host-next-auth.csrf-token" exists

POST https://jobs.dayforcehcm.com/api/geo/mymilacron/jobposting/search
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36
Content-Type: application/json
X-CSRF-TOKEN: {{csrf_token}}
{
  "clientNamespace": "mymilacron",
  "jobBoardCode": "CANDIDATEPORTAL",
  "cultureCode": "en-US",
  "paginationStart": 0
}

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.jobPostings[?(@.jobPostingId==2640)].postingLocations[0].isoCountryCode" == null
jsonpath "$.jobPostings[?(@.jobPostingId==2640)].postingLocations[0].stateCode" == null
jsonpath "$.jobPostings[?(@.jobPostingId==2640)].postingLocations[0].cityName" == null
jsonpath "$.jobPostings[?(@.jobPostingId==2640)].postingLocations[0].formattedAddress" == "Europe"
