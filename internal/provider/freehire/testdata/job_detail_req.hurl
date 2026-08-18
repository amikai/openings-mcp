# Full posting by public_slug. /jobs/{slug} takes no description_format,
# so the description is always the stored HTML.
GET https://freehire.me/api/v1/jobs/program-manager-intake-portfolio-management-stripe-plftlklg

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.data.public_slug" == "program-manager-intake-portfolio-management-stripe-plftlklg"
jsonpath "$.data.title" isString
jsonpath "$.data.company" == "Stripe"
jsonpath "$.data.company_slug" == "stripe"
jsonpath "$.data.description" isString
jsonpath "$.data.url" startsWith "https://"
