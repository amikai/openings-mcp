# Job detail page: the search hit's permalink under nodesk.co. The page
# embeds a schema.org JobPosting JSON-LD block (full HTML description,
# salary, locations) and the outbound apply URL in data-apply-url.
# This posting's JSON-LD validThrough is 2026-08-25; recapture with a
# fresh permalink from search_req.hurl once it expires live.
GET https://nodesk.co/remote-jobs/sticker-mule-software-engineer/
User-Agent: Mozilla/5.0

HTTP 200

[Asserts]
header "Content-Type" contains "text/html"
body contains "application/ld+json"
body contains "\"@type\":\"JobPosting\""
body contains "data-apply-url"
