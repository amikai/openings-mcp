# Search jobs — the exact request that captured jobs_rsp.html. See
# ../openapi.yaml paths./jobs/results.get for the full documented query
# parameter surface, including the multi-value filters (location,
# target_level, degree, employment_type, company) sent as repeated keys.
GET https://www.google.com/about/careers/applications/jobs/results
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36
Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8
Accept-Language: en-US,en;q=0.9
[QueryStringParams]
q: software engineer
location: Taiwan
employment_type: FULL_TIME
sort_by: date
page: 1

HTTP 200
