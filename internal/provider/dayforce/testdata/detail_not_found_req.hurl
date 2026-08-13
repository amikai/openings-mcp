GET https://jobs.dayforcehcm.com/api/geo/pca/jobposting/pca/en-US/1/99999999
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36

HTTP 404
[Asserts]
header "Content-Type" contains "application/problem+json"
jsonpath "$.status" == 404
jsonpath "$.traceId" exists
