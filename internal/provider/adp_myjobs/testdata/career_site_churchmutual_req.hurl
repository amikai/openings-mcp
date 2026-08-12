# Second tenant career-site config (public).

GET https://myjobs.adp.com/public/staffing/v1/career-site/churchmutual
Accept: application/json

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.clientName" exists
jsonpath "$.myJobsToken" exists
jsonpath "$.domain" == "churchmutual"
