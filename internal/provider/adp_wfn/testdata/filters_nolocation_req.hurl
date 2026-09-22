# This tenant publishes no FILTER LOCATION block at all, yet filters by city
# correctly — see search_locale_req.hurl for its board. Absence of the
# dimension means there is no list to validate against, not that location
# filtering is unavailable.
GET https://workforcenow.adp.com/mascsr/default/careercenter/public/events/staffing/v1/job-requisitions/getSearchFilters
[QueryStringParams]
cid: 5a312740-8c7b-44b7-80ad-50a35fbb25d3
locale: en_CA

HTTP 200
[Asserts]
jsonpath "$.data" count == 1
jsonpath "$.data[0].filterType" == "FILTER JOB TYPE"
