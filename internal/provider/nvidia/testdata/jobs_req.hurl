# Keyword search — the exact request that captured jobs_rsp.json (unfiltered
# appliedFacets); job_detail_req.hurl's externalPath is taken from this
# fixture. See ../openapi.yaml paths./jobs.post, examples.filtered for the
# documented facet vocabulary (jobFamilyGroup, locationHierarchy1/2, ...).
POST https://nvidia.wd5.myworkdayjobs.com/wday/cxs/nvidia/NVIDIAExternalCareerSite/jobs
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36
Accept: application/json
Content-Type: application/json
{
  "appliedFacets": {},
  "limit": 20,
  "offset": 0,
  "searchText": "golang"
}

HTTP 200
