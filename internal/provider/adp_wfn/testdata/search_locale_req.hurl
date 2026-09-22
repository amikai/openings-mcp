# This tenant publishes postings only under en_CA. Under en_US — which is
# also what omitting locale falls back to — the board is empty, with no
# error and no meta object to hint at why.
GET https://workforcenow.adp.com/mascsr/default/careercenter/public/events/staffing/v1/job-requisitions
[QueryStringParams]
cid: 5a312740-8c7b-44b7-80ad-50a35fbb25d3
locale: en_CA

HTTP 200
[Asserts]
jsonpath "$.jobRequisitions" count > 0

GET https://workforcenow.adp.com/mascsr/default/careercenter/public/events/staffing/v1/job-requisitions
[QueryStringParams]
cid: 5a312740-8c7b-44b7-80ad-50a35fbb25d3
locale: en_US

HTTP 200
[Asserts]
jsonpath "$.jobRequisitions" count == 0
