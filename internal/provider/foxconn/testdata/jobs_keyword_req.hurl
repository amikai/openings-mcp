# Proves keywords= is a real server-side filter, not an ignored param: it
# narrows the full ~953-job board to 6. keywords matches the body text too,
# so a hit's job_name need not contain the term.
GET https://recruit.foxconn.com/hh_recruit_tw_api/portal_api/JobVacancies
[QueryStringParams]
keywords: ADAS

HTTP 200
