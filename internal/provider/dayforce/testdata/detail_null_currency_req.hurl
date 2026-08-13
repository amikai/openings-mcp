# No CSRF needed for GET endpoints (see detail_req.hurl). jdemea 35762 has
# isoCurrencyRegion: null (9 of 10 surveyed tenants; PCA is the only one that
# returns a value) and hasVirtualLocation: false with zero postingLocations —
# a posting with no location signal at all, distinct from the other
# null-postingLocations postings, which are all remote.
GET https://jobs.dayforcehcm.com/api/geo/jdemea/jobposting/jdemea/en-US/1/35762
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.jobPostingId" == 35762
jsonpath "$.isoCurrencyRegion" == null
jsonpath "$.hasVirtualLocation" == false
jsonpath "$.postingLocations" count == 0
