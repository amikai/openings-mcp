# Company profile plus a page of its open jobs. The profile is the only
# place the industries, maturity, and yc_* vocabularies can be read, and
# their real spelling is not what openapi.yaml's examples suggest:
# yc_batch is written out in full ("Summer 2009"), so the documented W21
# form matches nothing.
#
# limit=1 keeps the fixture small on purpose: the jobs it returns carry
# their FULL descriptions, so the upstream default of 20 is ~125 KB.
GET https://freehire.me/api/v1/companies/stripe
[QueryStringParams]
limit: 1

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.data.company.slug" == "stripe"
jsonpath "$.data.company.name" == "Stripe"
jsonpath "$.data.company.job_count" isInteger
jsonpath "$.data.company.maturity" isString
jsonpath "$.data.company.yc_batch" count == 1
jsonpath "$.data.company.yc_batch[0]" == "Summer 2009"
jsonpath "$.data.company.industries" count > 0
jsonpath "$.data.jobs" count == 1
# my_vote is documented as always null on this unauthenticated schema,
# but comes back as 0.
jsonpath "$.data.company.my_vote" == 0
