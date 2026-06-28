#!/bin/bash
# Job ID: 106863362666570438 (Software Engineer, GPU System Software, Taipei, Taiwan)
JOB_ID="106863362666570438"
curl -s \
  -A "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36" \
  -H "Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8" \
  -H "Accept-Language: en-US,en;q=0.9" \
  -H "Referer: https://www.google.com/about/careers/applications/jobs/results" \
  "https://www.google.com/about/careers/applications/jobs/results/${JOB_ID}" \
  > job_detail_rsp.html
