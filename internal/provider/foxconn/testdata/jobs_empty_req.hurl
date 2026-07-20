# No-404 quirk: an unknown/invalid workplaceCode is HTTP 200 with an empty
# array, not an error.
GET https://recruit.foxconn.com/hh_recruit_tw_api/portal_api/JobVacancies
[QueryStringParams]
workplaceCode: ZZZINVALID

HTTP 200
[Asserts]
jsonpath "$" count == 0
