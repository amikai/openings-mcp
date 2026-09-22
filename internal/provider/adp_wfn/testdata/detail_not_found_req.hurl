# An unknown posting id is answered with HTTP 200 and a record stripped of
# its itemID. That absence is the only not-found signal this endpoint gives.
GET https://workforcenow.adp.com/mascsr/default/careercenter/public/events/staffing/v1/job-requisitions/99999999
[QueryStringParams]
cid: 269ead05-4042-4cc5-b325-75a73a6e0439
locale: en_US

HTTP 200
[Asserts]
jsonpath "$.itemID" not exists
