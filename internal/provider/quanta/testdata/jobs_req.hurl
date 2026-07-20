# Full dump of Quanta's open vacancies: one anonymous GET returns the
# whole board (239 rows at capture time, 2026-07-21) with descriptions,
# requirements, and salary inline. No pagination, no query parameters,
# no cookies or User-Agent required, and no detail endpoint — the site's
# own JS fetches this once and does all searching/filtering client-side;
# detail lookup means finding the `serial` in this dump. The page script
# also reads `data.category` / `data.location` from this response, but
# the live endpoint has never returned those keys — the filter option
# lists are server-rendered into the /QuantaRecruit/Home/Job page HTML
# instead.
GET https://hr.quantatw.com/QuantaRecruit/Home/QueryJob

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.jobResult" count > 0
jsonpath "$.jobResult[0].serial" isString
jsonpath "$.jobResult[0].jobdid" isString
jsonpath "$.jobResult[0].jobdnm" isString
jsonpath "$.jobResult[0].caponm" isString
jsonpath "$.jobResult[0].locatm" isString
jsonpath "$.jobResult[0].descnm" isString
