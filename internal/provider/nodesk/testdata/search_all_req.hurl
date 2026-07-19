# Empty query: pages through the whole board, featured postings first.
# The first page also carries the index's injected advertisement record
# (is_ad: true) that the client drops from results.
POST https://0586L1SOK8-dsn.algolia.net/1/indexes/jobPosts/query
X-Algolia-Application-Id: 0586L1SOK8
X-Algolia-API-Key: 8dacb58c6f375cba28e19ecf1f03e9e1
Referer: https://nodesk.co/
{
  "params": "query=&hitsPerPage=20&page=0"
}

HTTP 200

[Asserts]
jsonpath "$.nbHits" >= 20
jsonpath "$.nbPages" >= 2
jsonpath "$.hits" count == 20
