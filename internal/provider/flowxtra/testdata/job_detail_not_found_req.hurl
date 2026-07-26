# Unknown has_id: a clean JSON 404 envelope, no HTML error page.
GET https://app.flowxtra.com/api/candidate/jobs/ZZZZZ99

HTTP 404
[Asserts]
jsonpath "$.success" == false
jsonpath "$.message" == "Central job not found"
