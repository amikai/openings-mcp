# jobId is a live posting and will 703015 once DeepSeek closes it; recapture
# both files from a current jobs/v2 response when that happens.
POST https://app.mokahr.com/api/outer/ats-apply/website/job
Content-Type: application/json
{
  "orgId": "high-flyer",
  "siteId": "140576",
  "jobId": "7dcd6fde-84f1-4deb-890c-f1f275df0efc",
  "locale": "en-US"
}

HTTP 200
[Asserts]
jsonpath "$.necromancer" matches "^[0-9a-f]{16}$"
jsonpath "$.data" matches "^[A-Za-z0-9+/]+={0,2}$"
