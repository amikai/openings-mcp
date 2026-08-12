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

# Location via $filter: a bounding box over workLocations.geoLocation, boxed
# around the Hollywood store (34.1147313 / -118.3637264). The literal
# "undefined" tokens are required — a well-formed POLYGON is answered with 500.
GET https://my.adp.com/myadp_prefix/mycareer/public/staffing/v1/job-requisitions/apply-custom-filters?$orderby=postingDate%20desc&$select=reqId,jobTitle,requisitionLocations&$top=20&$skip=0&tz=America/New_York&$filter=geo.intersects(workLocations.geoLocation,%20geography%27POLYGON((undefined,%20-118.4%2034.09,%20undefined,%20-118.33%2034.14))%27)
Accept: application/json
Origin: https://myjobs.adp.com
Referer: https://myjobs.adp.com/
myjobstoken: {{myjobs_token}}
rolecode: manager
orgoid: {{orgoid}}

HTTP 200
[Asserts]
jsonpath "$.count" isInteger
jsonpath "$.count" > 0
jsonpath "$.count" < {{baseline_count}}
jsonpath "$.jobRequisitions[0].requisitionLocations[0].address.countrySubdivisionLevel1.codeValue" == "CA"
# The address field is "country", not "countryCode".
jsonpath "$.jobRequisitions[0].requisitionLocations[0].address.country.codeValue" == "USA"
jsonpath "$.jobRequisitions[0].requisitionLocations[0].address.geoCoordinate.latitude" exists

# Box AND keyword are both applied server-side in one request.
GET https://my.adp.com/myadp_prefix/mycareer/public/staffing/v1/job-requisitions/apply-custom-filters?$orderby=postingDate%20desc&$select=reqId,jobTitle&$top=20&$skip=0&tz=America/New_York&$search=teacher&$filter=geo.intersects(workLocations.geoLocation,%20geography%27POLYGON((undefined,%20-118.98%2033.45,%20undefined,%20-117.84%2034.6))%27)
Accept: application/json
Origin: https://myjobs.adp.com
Referer: https://myjobs.adp.com/
myjobstoken: {{myjobs_token}}
rolecode: manager
orgoid: {{orgoid}}

HTTP 200
[Captures]
la_teacher_count: jsonpath "$.count"
[Asserts]
jsonpath "$.count" > 0
jsonpath "$.count" < {{baseline_count}}

# A zero-area box is rejected, which is why resolved boxes are padded.
GET https://my.adp.com/myadp_prefix/mycareer/public/staffing/v1/job-requisitions/apply-custom-filters?$select=reqId&$top=1&tz=America/New_York&$filter=geo.intersects(workLocations.geoLocation,%20geography%27POLYGON((undefined,%20-118.3637264%2034.1147313,%20undefined,%20-118.3637264%2034.1147313))%27)
Accept: application/json
Origin: https://myjobs.adp.com
Referer: https://myjobs.adp.com/
myjobstoken: {{myjobs_token}}
rolecode: manager
orgoid: {{orgoid}}

HTTP 500

# Regression guard for the trap that makes a wrong location filter invisible:
# a $filter on any non-geo field returns HTTP 200 and the UNFILTERED board.
# If either assert ever fails, the endpoint gained real field filtering and the
# adapter can stop going through coordinates.
GET https://my.adp.com/myadp_prefix/mycareer/public/staffing/v1/job-requisitions/apply-custom-filters?$select=reqId&$top=1&tz=America/New_York&$filter=requisitionLocations/itemID%20eq%205000010689906
Accept: application/json
Origin: https://myjobs.adp.com
Referer: https://myjobs.adp.com/
myjobstoken: {{myjobs_token}}
rolecode: manager
orgoid: {{orgoid}}

HTTP 200
[Asserts]
jsonpath "$.count" == {{baseline_count}}

GET https://my.adp.com/myadp_prefix/mycareer/public/staffing/v1/job-requisitions/apply-custom-filters?$select=reqId&$top=1&tz=America/New_York&$filter=totallyBogusField%20eq%20%27zzz%27
Accept: application/json
Origin: https://myjobs.adp.com
Referer: https://myjobs.adp.com/
myjobstoken: {{myjobs_token}}
rolecode: manager
orgoid: {{orgoid}}

HTTP 200
[Asserts]
jsonpath "$.count" == {{baseline_count}}

# The facet catalog behind the board's "All Filters" dialog: one request,
# tenant-defined categories. Labels carry no coordinates, so the adapter builds
# its location catalog from the board dump instead.
GET https://my.adp.com/myadp_prefix/mycareer/public/staffing/v1/job-requisitions/search-custom-filters?tz=America/New_York
Accept: application/json
Origin: https://myjobs.adp.com
Referer: https://myjobs.adp.com/
myjobstoken: {{myjobs_token}}
rolecode: manager
orgoid: {{orgoid}}

HTTP 200
[Asserts]
jsonpath "$.filterList" count > 0
jsonpath "$.filterList[0].categoryLabel" exists
