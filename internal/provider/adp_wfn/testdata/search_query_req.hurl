# userQuery is the one real query parameter, and it searches description text.
# Its total is a relevance tally, not a row count.
GET https://workforcenow.adp.com/mascsr/default/careercenter/public/events/staffing/v1/job-requisitions
[QueryStringParams]
cid: d16ba13d-474e-4326-b628-74c87f0b289a
locale: en_US
userQuery: welder

HTTP 200
[Asserts]
jsonpath "$.jobRequisitions" count > 0
