# Unknown MyJobs slug — live API returns HTTP 400 (not 404) with Careersite not found.

GET https://myjobs.adp.com/public/staffing/v1/career-site/this-company-does-not-exist-xyz-openings-mcp
Accept: application/json

HTTP 400
[Asserts]
jsonpath "$.message" exists
