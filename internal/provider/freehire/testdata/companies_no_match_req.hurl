# Fuzzy matching still bottoms out: a query with no near neighbour is an
# empty page, so "not found" stays distinguishable from a typo.
GET https://freehire.me/api/v1/companies
[QueryStringParams]
q: zzzzzzzzzz
limit: 1

HTTP 200
[Asserts]
jsonpath "$.meta.total" == 0
jsonpath "$.data" count == 0
