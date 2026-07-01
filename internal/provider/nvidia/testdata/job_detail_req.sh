#!/bin/bash
BASE="https://nvidia.wd5.myworkdayjobs.com/wday/cxs/nvidia/NVIDIAExternalCareerSite"
# location/titleSlug taken from a jobs_rsp.json externalPath: /job/{location}/{titleSlug}
curl -s \
  -H "User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36" \
  -H "Accept: application/json" \
  "$BASE/job/Israel-Yokneam/Senior-Software-Golang-Kubernetes-Engineer_JR2015916" \
  | jq . > job_detail_rsp.json
