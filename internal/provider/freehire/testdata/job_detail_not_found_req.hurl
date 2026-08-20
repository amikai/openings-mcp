# Unknown public_slug returns a JSON 404.
GET https://freehire.me/api/v1/jobs/does-not-exist-zzzz
User-Agent: amikai/openings-mcp (+https://github.com/amikai/openings-mcp)

HTTP 404
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.error" == "not found"
