# List published postings (see ../openapi.yaml paths./v0/postings/{site}.get).
# mode is required; skip/limit paginate. The repeatable filters (location,
# commitment, team, department, level) are tenant-specific and omitted here
# — leverdemo's small demo dataset would likely zero out under an arbitrary
# filter value, breaking recapture.
GET https://api.lever.co/v0/postings/leverdemo
[QueryStringParams]
mode: json
skip: 0
limit: 3

HTTP 200
