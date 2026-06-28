#!/bin/bash
curl -s \
  -H "User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36" \
  -H "Accept: application/json, text/plain, */*" \
  -H "Content-Type: application/json" \
  -H "Referer: https://www.cake.me/jobs" \
  -X POST \
  --data '{"query":"Golang","sort_by":"popularity","filters":{}}' \
  "https://api.cake.me/api/client/v1/jobs/search" \
  | jq . > search_jobs_rsp.json
