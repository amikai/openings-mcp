# The guest jobs/view/{id} endpoint 999-blocks (authwall redirect) a cold
# request with no session. It works when the cookies set by a prior request
# to the same host (jobs-guest search or the public jobs/search page) are
# carried along, same as a browser would — hurl shares cookie state across
# entries within one file automatically, so entry 1 below is a warm-up
# search (same request as jobs_req.hurl) purely to collect that session
# before entry 2 reuses it for the job detail fetch. Splitting the warm-up
# into its own file and stitching cookies back together with
# `hurl --cookie-jar` does NOT work here — a separate cookie-jar file
# round-trip still 999s, only same-file entry chaining does.
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

GET https://www.linkedin.com/jobs/view/4422697744
authority: www.linkedin.com
accept: text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7
accept-language: en-US,en;q=0.9
cache-control: max-age=0
upgrade-insecure-requests: 1
referer: https://www.linkedin.com/jobs/search?keywords=software%20engineer&location=Taiwan
user-agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36
[Options]
location: true

HTTP 200
