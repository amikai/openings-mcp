# jobId here is the ExternalJobID custom field, not the itemID. Detail adds
# exactly one field over the list row: requisitionDescription.
GET https://workforcenow.adp.com/mascsr/default/careercenter/public/events/staffing/v1/job-requisitions/584587
[QueryStringParams]
cid: 269ead05-4042-4cc5-b325-75a73a6e0439
locale: en_US

HTTP 200
[Asserts]
jsonpath "$.itemID" exists
jsonpath "$.requisitionTitle" exists
jsonpath "$.requisitionDescription" exists
