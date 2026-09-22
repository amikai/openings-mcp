# Every filter-shaped parameter other than userQuery is ignored and answers
# the whole board. An invented parameter behaves identically, which is the
# point: there is no way to tell a supported filter from an unsupported one
# by the response.
GET https://workforcenow.adp.com/mascsr/default/careercenter/public/events/staffing/v1/job-requisitions
[QueryStringParams]
cid: 269ead05-4042-4cc5-b325-75a73a6e0439
locale: en_US
$search: welder
searchText: welder
fooBarBaz: quux

HTTP 200
[Asserts]
jsonpath "$.meta.totalNumber" isInteger
jsonpath "$.jobRequisitions" count > 0
