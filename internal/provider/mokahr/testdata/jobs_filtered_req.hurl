# Server-side location filtering. Ulanqab's ids narrow the 35-posting board to
# two, so a client that drops locationIds gets a visibly different set.
POST https://app.mokahr.com/api/outer/ats-apply/website/jobs/v2
Content-Type: application/json
{
  "orgId": "high-flyer",
  "siteId": "140576",
  "limit": 5,
  "offset": 0,
  "needStat": true,
  "locale": "en-US",
  "locationIds": [
    655446
  ]
}

HTTP 200
[Asserts]
jsonpath "$.necromancer" matches "^[0-9a-f]{16}$"
jsonpath "$.data" matches "^[A-Za-z0-9+/]+={0,2}$"
