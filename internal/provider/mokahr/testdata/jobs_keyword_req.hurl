# keyword matches titles only, never descriptions: "运维" returns the one
# posting with it in the title, though five carry it in the body.
POST https://app.mokahr.com/api/outer/ats-apply/website/jobs/v2
Content-Type: application/json
{
  "orgId": "high-flyer",
  "siteId": "140576",
  "limit": 5,
  "offset": 0,
  "needStat": true,
  "locale": "en-US",
  "keyword": "运维"
}

HTTP 200
[Asserts]
jsonpath "$.necromancer" matches "^[0-9a-f]{16}$"
jsonpath "$.data" matches "^[A-Za-z0-9+/]+={0,2}$"
