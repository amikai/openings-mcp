# Server-side search: search-key narrows on the job title (LIKE), and it
# is real narrowing — total drops and every returned title matches.
# location (city / state / country LIKE), company_name (LIKE), and
# workplace (exact "On-site" / "Hybrid" / "Remote") behave the same way.
# The upstream employment_type parameter is a live no-op (the backing
# JSON column stores objects, the filter compares plain strings — every
# value returns zero rows) and is deliberately not modeled.
GET https://app.flowxtra.com/api/central/jobs
[QueryStringParams]
per_page: 3
search-key: sales

HTTP 200
[Asserts]
jsonpath "$.success" == true
jsonpath "$.data.data" count > 0
jsonpath "$.data.data[0].title" contains "ales"
