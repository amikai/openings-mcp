# Live recon: public MyJobs career-site config then listings.
# Captures myJobsToken/orgoid in-session (public short-lived session; not partner API).
# Do not commit real myJobsToken values; fixtures use TEST_MYJOBS_TOKEN only.

GET https://myjobs.adp.com/public/staffing/v1/career-site/guitarcenterexternal
Accept: application/json

HTTP 200
[Captures]
myjobs_token: jsonpath "$.myJobsToken"
orgoid: jsonpath "$.orgoid"
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.clientName" exists
jsonpath "$.myJobsToken" exists
jsonpath "$.orgoid" exists
jsonpath "$.domain" == "guitarcenterexternal"

GET https://my.adp.com/myadp_prefix/mycareer/public/staffing/v1/job-requisitions/apply-custom-filters?$orderby=postingDate%20desc&$select=reqId,jobTitle,publishedJobTitle,type,jobDescription,jobQualifications,workLevelCode,clientRequisitionID,postingDate,requisitionLocations&$top=5&$skip=0&tz=America/New_York
Accept: application/json
Origin: https://myjobs.adp.com
Referer: https://myjobs.adp.com/
myjobstoken: {{myjobs_token}}
rolecode: manager
orgoid: {{orgoid}}

HTTP 200
[Captures]
baseline_count: jsonpath "$.count"
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.count" isInteger
jsonpath "$.count" > 0
jsonpath "$.jobRequisitions" count > 0
jsonpath "$.jobRequisitions[0].reqId" exists
jsonpath "$.jobRequisitions[0].jobTitle" exists

# Server-side free-text keyword via $search (must include the $ prefix).
GET https://my.adp.com/myadp_prefix/mycareer/public/staffing/v1/job-requisitions/apply-custom-filters?$orderby=postingDate%20desc&$select=reqId,jobTitle,publishedJobTitle&$top=5&$skip=0&tz=America/New_York&$search=teacher
Accept: application/json
Origin: https://myjobs.adp.com
Referer: https://myjobs.adp.com/
myjobstoken: {{myjobs_token}}
rolecode: manager
orgoid: {{orgoid}}

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.count" isInteger
jsonpath "$.count" > 0
jsonpath "$.count" < {{baseline_count}}
jsonpath "$.jobRequisitions" count > 0
