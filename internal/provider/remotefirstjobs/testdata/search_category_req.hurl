# Server-side category filter: every hit's category echoes the filter.
# The parameter accepts dash and underscore forms interchangeably
# (?category=software-development and ?category=software_development
# returned byte-identical bodies at capture time); responses always
# carry the underscore form. No paired _rsp.json on purpose — the body
# is another ~900KB page whose envelope search_rsp.json already covers.
GET https://remotefirstjobs.com/api/search-jobs
[QueryStringParams]
category: design

HTTP 200
[Asserts]
jsonpath "$.jobs" count == 100
# Every hit matches the filter: the design-only subset is the whole page.
jsonpath "$.jobs[?(@.category=='design')]" count == 100
