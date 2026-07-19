# Only pages 0-4 exist; anything outside is a 400, same error shape as
# an invalid category.
GET https://remotefirstjobs.com/api/search-jobs
[QueryStringParams]
page: 5

HTTP 400
[Asserts]
jsonpath "$.kind" == "invalid_argument"
jsonpath "$.message" contains "page must be between 0 and 4"
