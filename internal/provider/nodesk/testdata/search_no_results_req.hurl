# A query matching nothing is still HTTP 200 with empty hits.
POST https://0586L1SOK8-dsn.algolia.net/1/indexes/jobPosts/query
X-Algolia-Application-Id: 0586L1SOK8
X-Algolia-API-Key: 8dacb58c6f375cba28e19ecf1f03e9e1
Referer: https://nodesk.co/
{
  "params": "query=qqqzzzxx&hitsPerPage=20&page=0"
}

HTTP 200

[Asserts]
jsonpath "$.nbHits" == 0
jsonpath "$.hits" count == 0
