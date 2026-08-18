# City typeahead. The returned value is exactly what the cities filter
# expects; the filter holds display names and matches nothing on a near
# miss, so it has to be resolved here first. City names are not unique,
# and the cities facet carries no country qualifier — hence the country
# parameter on this endpoint but not on the filter.
GET https://freehire.me/api/v1/geo/cities
[QueryStringParams]
q: lond
country: gb

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.data" count > 0
jsonpath "$.data[0].value" isString
jsonpath "$.data[0].country" == "gb"
# This response carries no meta block at all.
jsonpath "$.meta" not exists
