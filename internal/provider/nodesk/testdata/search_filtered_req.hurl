# Server-side facet filtering: facetFilters is a URL-encoded JSON array
# inside params. Top-level entries AND together; searchFilter values are
# the site's category paths and applicantLocationRegions holds region
# labels (both enumerated by facets_req.hurl).
POST https://0586L1SOK8-dsn.algolia.net/1/indexes/jobPosts/query
X-Algolia-Application-Id: 0586L1SOK8
X-Algolia-API-Key: 8dacb58c6f375cba28e19ecf1f03e9e1
Referer: https://nodesk.co/
{
  "params": "query=&hitsPerPage=20&page=0&facetFilters=%5B%22searchFilter%3Aremote-jobs%2Fengineering%22%2C%22applicantLocationRegions%3ARemote%20-%20Europe%22%5D"
}

HTTP 200

[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.nbHits" >= 1
jsonpath "$.hits[0].applicantLocationRegions" contains "Remote - Europe"
