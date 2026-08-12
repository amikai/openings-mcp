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

# The tenant's own filter dimensions, in one request. Slot codes are positional
# per tenant, so the codes below are Guitar Center's and mean other things
# elsewhere: FIELD2 "Area of Interest", FIELD3 "Full-Time/Part-Time",
# FIELD4 "Brand". A failure here means the tenant reconfigured its fields.
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
jsonpath "$.filterList[0].category" matches /^FIELD[0-9]+$/
jsonpath "$.filterList[0].categoryLabel" exists
jsonpath "$.filterList[0].filterList[0].value" exists

# Applying one narrows the board.
GET https://my.adp.com/myadp_prefix/mycareer/public/staffing/v1/job-requisitions/apply-custom-filters?$select=reqId&$top=1&tz=America/New_York&$filter=FIELD3%20eq%20%27Full-time%27
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

# A value containing spaces filters too, once the spaces are %20.
GET https://my.adp.com/myadp_prefix/mycareer/public/staffing/v1/job-requisitions/apply-custom-filters?$select=reqId&$top=1&tz=America/New_York&$filter=FIELD2%20eq%20%27Customer%20Service%20and%20Sales%27
Accept: application/json
Origin: https://myjobs.adp.com
Referer: https://myjobs.adp.com/
myjobstoken: {{myjobs_token}}
rolecode: manager
orgoid: {{orgoid}}

HTTP 200
[Captures]
aoi_count: jsonpath "$.count"
[Asserts]
jsonpath "$.count" > 0
jsonpath "$.count" < {{baseline_count}}

# Clauses AND with && , and compose with $search, in one request.
GET https://my.adp.com/myadp_prefix/mycareer/public/staffing/v1/job-requisitions/apply-custom-filters?$select=reqId&$top=1&tz=America/New_York&$search=teacher&$filter=FIELD2%20eq%20%27Customer%20Service%20and%20Sales%27%20%26%26%20FIELD3%20eq%20%27Full-time%27
Accept: application/json
Origin: https://myjobs.adp.com
Referer: https://myjobs.adp.com/
myjobstoken: {{myjobs_token}}
rolecode: manager
orgoid: {{orgoid}}

HTTP 200
[Asserts]
jsonpath "$.count" isInteger
jsonpath "$.count" < {{aoi_count}}

# Regression guards for the three ways a wrong filter goes unnoticed: each
# returns the FULL board with HTTP 200 rather than an error, which is why the
# adapter never forwards an unvalidated field, a label, or a "+" for a space.

# (1) a slot code the board has not configured
GET https://my.adp.com/myadp_prefix/mycareer/public/staffing/v1/job-requisitions/apply-custom-filters?$select=reqId&$top=1&tz=America/New_York&$filter=FIELD9%20eq%20%27x%27
Accept: application/json
Origin: https://myjobs.adp.com
Referer: https://myjobs.adp.com/
myjobstoken: {{myjobs_token}}
rolecode: manager
orgoid: {{orgoid}}

HTTP 200
[Asserts]
jsonpath "$.count" == {{baseline_count}}

# (2) the display label instead of the slot code
GET https://my.adp.com/myadp_prefix/mycareer/public/staffing/v1/job-requisitions/apply-custom-filters?$select=reqId&$top=1&tz=America/New_York&$filter=Brand%20eq%20%27AVDG%27
Accept: application/json
Origin: https://myjobs.adp.com
Referer: https://myjobs.adp.com/
myjobstoken: {{myjobs_token}}
rolecode: manager
orgoid: {{orgoid}}

HTTP 200
[Asserts]
jsonpath "$.count" == {{baseline_count}}

# (3) spaces encoded as "+" rather than %20 — same filter as the AoI one above
GET https://my.adp.com/myadp_prefix/mycareer/public/staffing/v1/job-requisitions/apply-custom-filters?$select=reqId&$top=1&tz=America/New_York&$filter=FIELD2+eq+%27Customer+Service+and+Sales%27
Accept: application/json
Origin: https://myjobs.adp.com
Referer: https://myjobs.adp.com/
myjobstoken: {{myjobs_token}}
rolecode: manager
orgoid: {{orgoid}}

HTTP 200
[Asserts]
jsonpath "$.count" == {{baseline_count}}
