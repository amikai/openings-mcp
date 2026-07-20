# Unlike the list endpoint, an unknown detail id 404s with an RFC 7807
# problem+json body. traceId is per-request, so it is not asserted.
GET https://recruit.foxconn.com/hh_recruit_tw_api/portal_api/JobVacancies/00000000000000000000000000000000

HTTP 404
[Asserts]
jsonpath "$.status" == 404
jsonpath "$.title" == "Not Found"
