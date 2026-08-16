# Small board, unfiltered: the one case where meta.totalNumber is a row count.
GET https://workforcenow.adp.com/mascsr/default/careercenter/public/events/staffing/v1/job-requisitions
[QueryStringParams]
cid: 269ead05-4042-4cc5-b325-75a73a6e0439
locale: en_US
$skip: 1
$top: 20

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.meta.totalNumber" isInteger
jsonpath "$.jobRequisitions" count > 0
jsonpath "$.jobRequisitions[0].itemID" exists
jsonpath "$.jobRequisitions[0].requisitionTitle" exists
jsonpath "$.jobRequisitions[0].requisitionDescription" not exists
