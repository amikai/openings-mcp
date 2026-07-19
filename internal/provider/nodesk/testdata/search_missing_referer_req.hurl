# The search-only API key is referer-locked: the same request without a
# nodesk.co Referer header is rejected. The client always sends the header.
POST https://0586L1SOK8-dsn.algolia.net/1/indexes/jobPosts/query
X-Algolia-Application-Id: 0586L1SOK8
X-Algolia-API-Key: 8dacb58c6f375cba28e19ecf1f03e9e1
{
  "params": "query=golang&hitsPerPage=20&page=0"
}

HTTP 403

[Asserts]
jsonpath "$.message" contains "referer"
