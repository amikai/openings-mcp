# Cross-tenant list: one Laravel-paginated board covering every company
# hosted on Flowxtra, newest-first by date_share. per_page=3 keeps the
# fixture small. The server hardcodes the visibility window (status Live,
# admin-approved, shared within the last 90 days, not expired), so there
# is no status parameter to send.
GET https://app.flowxtra.com/api/central/jobs
[QueryStringParams]
per_page: 3

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.success" == true
jsonpath "$.data.current_page" == 1
jsonpath "$.data.total" isInteger
jsonpath "$.data.last_page" isInteger
jsonpath "$.data.data" count == 3
jsonpath "$.data.data[0].has_id" isString
jsonpath "$.data.data[0].title" isString
jsonpath "$.data.data[0].name_company" isString
jsonpath "$.data.data[0].country_company" isString
jsonpath "$.data.data[0].date_share" isString
jsonpath "$.data.data[0].urlJobApplay" startsWith "https://flowxtra.com/apply/"
