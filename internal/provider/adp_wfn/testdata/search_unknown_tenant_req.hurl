# The only loud failure on this surface, and it is not JSON.
GET https://workforcenow.adp.com/mascsr/default/careercenter/public/events/staffing/v1/job-requisitions
[QueryStringParams]
cid: 00000000-0000-0000-0000-000000000000
locale: en_US

HTTP 404
[Asserts]
header "Content-Type" contains "text/html"
