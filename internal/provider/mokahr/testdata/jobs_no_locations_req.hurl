# Some tenants keep workplaces off the listing: every posting comes back with
# "locations": null even though getJob reports a real one. Captured from a
# tenant that also publishes no filter facets at all, which is the pairing
# that makes a location search unanswerable from the listing alone.
POST https://app.mokahr.com/api/outer/ats-apply/website/jobs/v2
Content-Type: application/json
{
  "orgId": "vanke",
  "siteId": "36266",
  "limit": 5,
  "offset": 0,
  "needStat": true,
  "locale": "en-US"
}

HTTP 200
[Asserts]
jsonpath "$.necromancer" matches "^[0-9a-f]{16}$"
jsonpath "$.data" matches "^[A-Za-z0-9+/]+={0,2}$"
