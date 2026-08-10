# An unrecognized sort is a hard 400, not a silent fallback to the default.
GET https://4dayweek.io/api/v2/jobs
[QueryStringParams]
sort: bogus

HTTP 400
[Asserts]
jsonpath "$.error.code" == 400
jsonpath "$.error.message" isString
