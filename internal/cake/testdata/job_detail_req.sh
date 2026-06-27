#!/bin/bash
# JOB_PATH from search_jobs_rsp.json first result.
JOB_PATH="senior-golang-web-backend-engineer-taoyuan"
curl -s \
  -H "User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36" \
  -H "Accept: application/json, text/plain, */*" \
  -H "Referer: https://www.cake.me/jobs" \
  "https://api.cake.me/api/client/v1/jobs/${JOB_PATH}" \
  | jq . > job_detail_rsp.json
