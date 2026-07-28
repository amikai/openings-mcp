# Detail happy path: one application/ld+json schema.org JobPosting.
GET https://en-gage.net/nova_career/work_17046487/
user-agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36
accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8

HTTP 200
[Asserts]
header "Content-Type" contains "text/html"
xpath "count(//script[@type='application/ld+json'])" == 1
