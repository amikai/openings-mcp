# Locations arrive already pair-formed and go on the wire as `value`, with
# `oid` as the display label. Job types invert that.
GET https://workforcenow.adp.com/mascsr/default/careercenter/public/events/staffing/v1/job-requisitions/getSearchFilters
[QueryStringParams]
cid: d16ba13d-474e-4326-b628-74c87f0b289a
locale: en_US

HTTP 200
[Asserts]
jsonpath "$.data" count > 0
jsonpath "$.data[?(@.filterType=='FILTER LOCATION')].searchFilterInfo[0].value" exists
