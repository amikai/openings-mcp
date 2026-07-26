# The one endpoint that answers in the clear — no data/necromancer envelope —
# so its payload can be asserted directly.
POST https://app.mokahr.com/api/outer/ats-apply/website/jobs/v2/filterFieldsAggregations
Content-Type: application/json
{
  "orgId": "high-flyer",
  "siteId": "140576",
  "locale": "en-US"
}

HTTP 200
[Asserts]
jsonpath "$.success" == true
jsonpath "$.code" == 0
jsonpath "$.data.systemFieldsAggregations.locationAggregation.locationList" count > 0
jsonpath "$.data.systemFieldsAggregations.locationAggregation.locationList[0].label" exists
jsonpath "$.data.systemFieldsAggregations.locationAggregation.locationList[0].locationRows[0].id" isInteger
jsonpath "$.data.systemFieldsAggregations.zhinengAggregation.zhinengList" count > 0
jsonpath "$.data.systemFieldsAggregations.zhinengAggregation.zhinengList[0].id" isInteger
jsonpath "$.data.systemFieldsAggregations.zhinengAggregation.zhinengList[0].label" exists
