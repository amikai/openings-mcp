# $skip is 1-based. Asking for offset 0 silently drops a row: the same $top
# returns one fewer item. Nothing in the response says so.
GET https://workforcenow.adp.com/mascsr/default/careercenter/public/events/staffing/v1/job-requisitions
[QueryStringParams]
cid: d16ba13d-474e-4326-b628-74c87f0b289a
locale: en_US
$skip: 1
$top: 5

HTTP 200
[Asserts]
jsonpath "$.jobRequisitions" count == 5

GET https://workforcenow.adp.com/mascsr/default/careercenter/public/events/staffing/v1/job-requisitions
[QueryStringParams]
cid: d16ba13d-474e-4326-b628-74c87f0b289a
locale: en_US
$skip: 0
$top: 5

HTTP 200
[Asserts]
jsonpath "$.jobRequisitions" count == 4
