# Filters are omitted because their values are tenant-specific and could zero
# out the small leverdemo dataset.
GET https://api.lever.co/v0/postings/leverdemo
[QueryStringParams]
mode: json
skip: 0
limit: 3

HTTP 200
