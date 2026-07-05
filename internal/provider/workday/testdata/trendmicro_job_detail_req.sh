#!/bin/bash
# Second-tenant capture (Trend Micro, wd3 pod, site "External"). The titleSlug
# is deliberately one of Workday's "XMLNAME-" slugs — generated when a job
# title starts with a non-letter (here "(Sr.) Backend Engineer") — so the
# fixture pins that platform quirk.
BASE="https://trendmicro.wd3.myworkdayjobs.com/wday/cxs/trendmicro/External"
# location/titleSlug taken from a trendmicro_jobs_rsp.json externalPath: /job/{location}/{titleSlug}
curl -s \
  -H "User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36" \
  -H "Accept: application/json" \
  "$BASE/job/Taipei/XMLNAME--Sr--Backend-Engineer_R0006260-1" \
  | jq . > trendmicro_job_detail_rsp.json
