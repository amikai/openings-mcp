#!/bin/bash
# The guest jobs/view/{id} endpoint 999-blocks (authwall redirect) a cold
# request with no session. It works when the cookies set by a prior request
# to the same host (jobs-guest search or the public jobs/search page) are
# carried along, same as a browser would. Capture jobs_rsp.html first with a
# cookie jar, then reuse the jar here:
#   curl -sS -c cookies.txt ... "https://www.linkedin.com/jobs-guest/jobs/api/seeMoreJobPostings/search?..." -o /dev/null
#   curl -sS -b cookies.txt ... "https://www.linkedin.com/jobs/view/{id}"
curl -sS \
  -H "authority: www.linkedin.com" \
  -H "accept: text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7" \
  -H "accept-language: en-US,en;q=0.9" \
  -H "cache-control: max-age=0" \
  -H "upgrade-insecure-requests: 1" \
  -H "referer: https://www.linkedin.com/jobs/search?keywords=software%20engineer&location=Taiwan" \
  -H "user-agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36" \
  -b cookies.txt \
  -L "https://www.linkedin.com/jobs/view/4422697744"
