# Keyword + facet-filtered search: all four required body fields filled in
# (see ../openapi.yaml paths./jobs.post — appliedFacets/limit/offset/
# searchText are all required even when unused).
POST https://nvidia.wd5.myworkdayjobs.com/wday/cxs/nvidia/NVIDIAExternalCareerSite/jobs
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36
Accept: application/json
Content-Type: application/json
{
  "appliedFacets": {
    "jobFamilyGroup": ["0c40f6bd1d8f10ae43ffaefd46dc7e78"]
  },
  "limit": 20,
  "offset": 0,
  "searchText": "golang"
}

HTTP 200
