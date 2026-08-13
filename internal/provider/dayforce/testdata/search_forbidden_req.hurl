# No prior getCsrf call in this file, so neither the cookie nor the header
# is present — the gate itself, so a transport regression fails loudly.
POST https://jobs.dayforcehcm.com/api/geo/pca/jobposting/search
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36
Content-Type: application/json
{
  "clientNamespace": "pca",
  "jobBoardCode": "CANDIDATEPORTAL",
  "cultureCode": "en-US",
  "paginationStart": 0
}

HTTP 403
[Asserts]
bytes count == 9
body == "Forbidden"
