# Minimal search: query/sort_by/filters are the only required body fields
# (see ../openapi.yaml paths./api/client/v1/jobs/search.post). filters is
# left {} here to preserve fixture fidelity — this exact body is what
# captured jobs_rsp.json, and client_test.go asserts the Go client sends it
# verbatim. The full filter vocabulary (locations, professions, job_types,
# seniority_levels, remote, salary, page, ...) is documented under
# components.schemas.JobSearchFilters in openapi.yaml.
POST https://api.cake.me/api/client/v1/jobs/search
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36
Accept: application/json, text/plain, */*
Content-Type: application/json
Referer: https://www.cake.me/jobs
{
  "query": "Golang",
  "sort_by": "popularity",
  "filters": {}
}

HTTP 200
