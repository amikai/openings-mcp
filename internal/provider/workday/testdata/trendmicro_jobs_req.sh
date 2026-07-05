#!/bin/bash
# Second-tenant capture (Trend Micro, wd3 pod, site "External") — verifies the
# CXS contract holds beyond NVIDIA's tenant.
BASE="https://trendmicro.wd3.myworkdayjobs.com/wday/cxs/trendmicro/External"
curl -s \
  -H "User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36" \
  -H "Accept: application/json" \
  -H "Content-Type: application/json" \
  -X POST \
  --data '{"appliedFacets":{},"limit":5,"offset":0,"searchText":"backend engineer"}' \
  "$BASE/jobs" \
  | jq . > trendmicro_jobs_rsp.json
