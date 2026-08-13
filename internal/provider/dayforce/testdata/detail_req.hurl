# No CSRF needed for GET endpoints. This posting had postingExpiryTimestampUTC:
# null when captured (2026-08-11); like the mynavi detail fixture, it may
# expire and need recapture against a different id from a fresh search.
GET https://jobs.dayforcehcm.com/api/geo/pca/jobposting/pca/en-US/1/62374
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.jobPostingId" == 62374
jsonpath "$.jobPostingAttributes" count > 0
jsonpath "$.jobPostingAttributes[?(@.name=='HiringMinRate')].value" isInteger
