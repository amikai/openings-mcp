# Search: keywords + location + start only, matching what actually captured
# jobs_rsp.html. See jobs_filtered_req.hurl for the full documented query
# parameter surface (see ../openapi.yaml
# paths./jobs-guest/jobs/api/seeMoreJobPostings/search.get) — most of those
# filters would narrow this specific "software engineer" + "Taiwan" result
# set differently, so they're kept out of the fixture-capturing request.
GET https://www.linkedin.com/jobs-guest/jobs/api/seeMoreJobPostings/search
authority: www.linkedin.com
accept: text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7
accept-language: en-US,en;q=0.9
cache-control: max-age=0
upgrade-insecure-requests: 1
user-agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36
[QueryStringParams]
keywords: software engineer
location: Taiwan
start: 0

HTTP 200
