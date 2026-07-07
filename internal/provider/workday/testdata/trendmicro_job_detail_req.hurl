# Second-tenant capture (Trend Micro, wd3 pod, site "External"). The titleSlug
# is deliberately one of Workday's "XMLNAME-" slugs — generated when a job
# title starts with a non-letter (here "(Sr.) Backend Engineer") — so the
# fixture pins that platform quirk. location/titleSlug taken from a
# trendmicro_jobs_rsp.json externalPath: /job/{location}/{titleSlug}
# (see ../openapi.yaml paths./job/{location}/{titleSlug}.get).
GET https://trendmicro.wd3.myworkdayjobs.com/wday/cxs/trendmicro/External/job/Taipei/XMLNAME--Sr--Backend-Engineer_R0006260-1
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36
Accept: application/json

HTTP 200
