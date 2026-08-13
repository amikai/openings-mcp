# No CSRF needed for GET endpoints (see detail_req.hurl). mymilacron 2684
# carries a jobPostingAttributes entry (TravelRequired) whose value is a
# JSON boolean with type "bool" — a shape JobPostingAttributeValue (modeled
# as string|number only) missed. Found live during this fix's mandated
# page-walk, not in the initial null-field sweep.
GET https://jobs.dayforcehcm.com/api/geo/mymilacron/jobposting/mymilacron/en-US/1/2684
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.jobPostingId" == 2684
jsonpath "$.jobPostingAttributes[?(@.name=='TravelRequired')].value" == true
jsonpath "$.jobPostingAttributes[?(@.name=='TravelRequired')].type" == "bool"
