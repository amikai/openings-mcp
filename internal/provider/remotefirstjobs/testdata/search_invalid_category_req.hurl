# Unknown category is a hard 400 with the API's uniform error shape,
# unlike the zero-hit query case (200 with jobs null).
GET https://remotefirstjobs.com/api/search-jobs
[QueryStringParams]
category: doesnotexist

HTTP 400
[Asserts]
jsonpath "$.kind" == "invalid_argument"
jsonpath "$.message" contains "invalid category"
