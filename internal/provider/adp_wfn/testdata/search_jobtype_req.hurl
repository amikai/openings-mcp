# A published job-type oid filters correctly. An unpublished one does not
# error, does not return zero, and does not return the whole board — it
# returns a large subset that looks like a filter that worked, which is why
# oids are validated against getSearchFilters before being sent.
GET https://workforcenow.adp.com/mascsr/default/careercenter/public/events/staffing/v1/job-requisitions
workerCategoriesList: 7:132
[QueryStringParams]
cid: 55a9d987-168e-4cd9-9f4d-bc3c10fb6e90
locale: en_US

HTTP 200
[Asserts]
jsonpath "$.jobRequisitions" count > 0

GET https://workforcenow.adp.com/mascsr/default/careercenter/public/events/staffing/v1/job-requisitions
workerCategoriesList: 9:999
[QueryStringParams]
cid: 55a9d987-168e-4cd9-9f4d-bc3c10fb6e90
locale: en_US

HTTP 200
[Asserts]
jsonpath "$.jobRequisitions" count > 0
