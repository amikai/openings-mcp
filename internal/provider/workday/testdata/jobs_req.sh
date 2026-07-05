#!/bin/bash
BASE="https://nvidia.wd5.myworkdayjobs.com/wday/cxs/nvidia/NVIDIAExternalCareerSite"
curl -s \
  -H "User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36" \
  -H "Accept: application/json" \
  -H "Content-Type: application/json" \
  -X POST \
  --data '{"appliedFacets":{"jobFamilyGroup":["0c40f6bd1d8f10ae43ffaefd46dc7e78"]},"limit":20,"offset":0,"searchText":"golang"}' \
  "$BASE/jobs" \
  | jq . > jobs_rsp.json
