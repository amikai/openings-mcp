# Full-text search over the jobPosts index. The app ID and search-only API
# key are public: they ship in the site's own /js/search.min.js. The key is
# referer-locked to nodesk.co (see search_missing_referer_req.hurl).
POST https://0586L1SOK8-dsn.algolia.net/1/indexes/jobPosts/query
X-Algolia-Application-Id: 0586L1SOK8
X-Algolia-API-Key: 8dacb58c6f375cba28e19ecf1f03e9e1
Referer: https://nodesk.co/
{
  "params": "query=golang&hitsPerPage=20&page=0"
}

HTTP 200

[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.nbHits" >= 1
jsonpath "$.hits[0].objectID" exists
jsonpath "$.hits[0].title" exists
jsonpath "$.hits[0].permalink" startsWith "/remote-jobs/"
jsonpath "$.hits[0].company.name" exists
