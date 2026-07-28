# Unknown jobId: HTTP 200 again, with code 703015 (您访问的职位已不存在) inside
# the obfuscated body.
POST https://app.mokahr.com/api/outer/ats-apply/website/job
Content-Type: application/json
{
  "orgId": "high-flyer",
  "siteId": "140576",
  "jobId": "00000000-0000-0000-0000-000000000000",
  "locale": "en-US"
}

HTTP 200
[Asserts]
jsonpath "$.necromancer" matches "^[0-9a-f]{16}$"
jsonpath "$.data" matches "^[A-Za-z0-9+/]+={0,2}$"
