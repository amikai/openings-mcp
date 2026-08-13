# mymilacron's second search page (paginationStart 25) includes
# jobPostingId 2418 ("Field Sales Representative - Extrusion"), whose
# postingLocations array itself is null (not just a field within a
# location) — distinct from search_null_location's continent-level-fields
# case, and the shape the earlier fix (widening only isoCountryCode) missed.
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
  "paginationStart": 25
}

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.jobPostings[?(@.jobPostingId==2418)].postingLocations" == null
jsonpath "$.jobPostings[?(@.jobPostingId==2418)].hasVirtualLocation" == true
