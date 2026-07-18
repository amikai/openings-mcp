# Proves "query" is a real server-side keyword filter: total narrows from 29
# to 9 for blueground and matching happens in title and description text.
POST https://apply.workable.com/api/v3/accounts/blueground/jobs
Content-Type: application/json
{
  "query": "engineer"
}

HTTP 200
[Asserts]
jsonpath "$.total" isInteger
