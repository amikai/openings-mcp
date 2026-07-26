# Without needStat the board still returns a full page of jobs, but jobStats
# degrades to total 0 rather than being omitted. A client that took that total
# as the board size would stop reading after one page, so this capture is what
# the "unknown total" path is tested against.
POST https://app.mokahr.com/api/outer/ats-apply/website/jobs/v2
Content-Type: application/json
{
  "orgId": "high-flyer",
  "siteId": "140576",
  "limit": 5,
  "offset": 0,
  "locale": "en-US"
}

HTTP 200
[Asserts]
jsonpath "$.necromancer" matches "^[0-9a-f]{16}$"
jsonpath "$.data" matches "^[A-Za-z0-9+/]+={0,2}$"
