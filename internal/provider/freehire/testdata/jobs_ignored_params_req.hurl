# A parameter no filter reads is dropped rather than refused, and the
# answer is the whole catalogue. "country" is the trap: every geography
# facet is plural, so the singular spelling filters nothing while looking
# exactly like a search that legitimately matched everything.
#
# meta.ignored_params is the only way to tell the two apart, which is why
# both the MCP tools and cmd/freehire turn its presence into an error.
GET https://freehire.me/api/v1/jobs/search
[QueryStringParams]
country: it
limit: 1

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.meta.ignored_params" count == 1
jsonpath "$.meta.ignored_params[0].param" == "country"
jsonpath "$.meta.ignored_params[0].did_you_mean" == "countries"

# The plural spelling filters, and returns a far smaller total.
GET https://freehire.me/api/v1/jobs/search
[QueryStringParams]
countries: it
limit: 1

HTTP 200
[Asserts]
jsonpath "$.meta.ignored_params" not exists
jsonpath "$.meta.total" isInteger
