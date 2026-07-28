# Job detail by has_id — the public hashed id carried on every list row.
# Returns the full posting: HTML description, company profile, employment
# types, office location, and the company's own career-page apply URL
# (unlike the list, whose urlJobApplay points at flowxtra.com/apply/...).
# Note date_share serializes as null here — posted-at only comes from the
# list. Captured 2026-07-26; like the other providers' detail fixtures
# this pins a point-in-time job that will eventually leave the board.
GET https://app.flowxtra.com/api/candidate/jobs/M88PB

HTTP 200
[Asserts]
jsonpath "$.success" == true
jsonpath "$.data.has_id" == "M88PB"
jsonpath "$.data.title" isString
jsonpath "$.data.description" isString
jsonpath "$.data.company.name" isString
jsonpath "$.data.urlJobApplay" isString
