# hitsPerPage=0 + facets enumerates every filterable value with its job
# count without paying for any hits — this is how the client's Facets
# method discovers the searchFilter paths and region labels.
POST https://0586L1SOK8-dsn.algolia.net/1/indexes/jobPosts/query
X-Algolia-Application-Id: 0586L1SOK8
X-Algolia-API-Key: 8dacb58c6f375cba28e19ecf1f03e9e1
Referer: https://nodesk.co/
{
  "params": "query=&hitsPerPage=0&facets=%5B%22searchFilter%22%2C%22applicantLocationRegions%22%5D"
}

HTTP 200

[Asserts]
jsonpath "$.hits" count == 0
jsonpath "$.facets.searchFilter" exists
jsonpath "$.facets['applicantLocationRegions']" exists
