# MokaHR obfuscates job payloads: the body is {"data": <base64 AES-128-CBC
# ciphertext>, "necromancer": <the key for this very response>}, and the key
# is fresh per response. Asserts therefore cover the envelope's shape, and
# the decoded payload is exercised in client_test.go against the captured
# fixture.
POST https://app.mokahr.com/api/outer/ats-apply/website/jobs/v2
Content-Type: application/json
{
  "orgId": "high-flyer",
  "siteId": "140576",
  "limit": 5,
  "offset": 0,
  "needStat": true,
  "locale": "en-US"
}

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.necromancer" matches "^[0-9a-f]{16}$"
jsonpath "$.data" matches "^[A-Za-z0-9+/]+={0,2}$"
