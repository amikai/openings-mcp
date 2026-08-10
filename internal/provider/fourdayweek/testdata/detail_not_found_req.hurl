# An unknown slug is a 404 with the same error envelope as the 400 case.
GET https://4dayweek.io/api/v2/jobs/no-such-job-xyz-000

HTTP 404
[Asserts]
jsonpath "$.error.code" == 404
jsonpath "$.error.message" == "Job not found"
