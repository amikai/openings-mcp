# The same tenant publishes no filter dimensions: systemFieldsAggregations is
# an empty object rather than absent, so get_filters legitimately has nothing
# to offer.
POST https://app.mokahr.com/api/outer/ats-apply/website/jobs/v2/filterFieldsAggregations
Content-Type: application/json
{
  "orgId": "vanke",
  "siteId": "36266",
  "locale": "en-US"
}

HTTP 200
[Asserts]
jsonpath "$.success" == true
jsonpath "$.data.systemFieldsAggregations" exists
jsonpath "$.data.systemFieldsAggregations.locationAggregation" not exists
