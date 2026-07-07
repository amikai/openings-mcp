# Demonstrates the full documented facet set (see ../openapi.yaml
# paths./jobs.post, examples.filtered) — Engineering (jobFamilyGroup) +
# Remote (locationHierarchy2) + United States (locationHierarchy1) on top of
# a keyword search. See jobs_req.hurl for the exact minimal request that
# captured jobs_rsp.json. Not wired to any committed fixture; illustrative
# only.
POST https://nvidia.wd5.myworkdayjobs.com/wday/cxs/nvidia/NVIDIAExternalCareerSite/jobs
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36
Accept: application/json
Content-Type: application/json
{
  "appliedFacets": {
    "jobFamilyGroup": ["0c40f6bd1d8f10ae43ffaefd46dc7e78"],
    "locationHierarchy2": ["0c3f5f117e9a0101f63dc469c3010000"],
    "locationHierarchy1": ["2fcb99c455831013ea52fb338f2932d8"]
  },
  "limit": 20,
  "offset": 0,
  "searchText": "golang"
}

HTTP 200
