# Demonstrates every documented query parameter (see ../openapi.yaml
# paths./jobs-guest/jobs/api/seeMoreJobPostings/search.get). distance is
# included for completeness but is explicitly not implemented by this
# client — its behavior was never confirmed against the live site. Not
# wired to any committed fixture; illustrative only, and this exact filter
# combination is unlikely to return real results.
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
distance: 50
f_WT: 2
f_JT: F
pageNum: 0
start: 0
f_AL: true
f_C: 1441,162479
f_TPR: r604800

HTTP 200
