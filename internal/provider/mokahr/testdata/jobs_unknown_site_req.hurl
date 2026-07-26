# An orgId/siteId pair that names no careers site still answers HTTP 200; the
# failure lives in the obfuscated body as code 703002, 未找到对应的官网.
POST https://app.mokahr.com/api/outer/ats-apply/website/jobs/v2
Content-Type: application/json
{
  "orgId": "openings-mcp-no-such-org",
  "siteId": "1",
  "limit": 5,
  "offset": 0,
  "needStat": true,
  "locale": "en-US"
}

HTTP 200
[Asserts]
jsonpath "$.necromancer" matches "^[0-9a-f]{16}$"
jsonpath "$.data" matches "^[A-Za-z0-9+/]+={0,2}$"
