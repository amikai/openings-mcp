#!/bin/bash
curl -s \
  -A "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36" \
  -H "Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8" \
  -H "Accept-Language: en-US,en;q=0.9" \
  "https://www.google.com/about/careers/applications/jobs/results?q=software+engineer&location=Taiwan&employment_type=FULL_TIME&sort_by=date&page=1" \
  > search_jobs_rsp.html
