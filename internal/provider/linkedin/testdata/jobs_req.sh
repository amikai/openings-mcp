#!/bin/bash
curl -sS \
  -H "authority: www.linkedin.com" \
  -H "accept: text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7" \
  -H "accept-language: en-US,en;q=0.9" \
  -H "cache-control: max-age=0" \
  -H "upgrade-insecure-requests: 1" \
  -H "user-agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36" \
  -G "https://www.linkedin.com/jobs-guest/jobs/api/seeMoreJobPostings/search" \
  --data-urlencode "keywords=software engineer" \
  --data-urlencode "location=Taiwan" \
  --data-urlencode "start=0"
