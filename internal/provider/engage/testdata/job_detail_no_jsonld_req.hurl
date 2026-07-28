# Detail page rendered without any application/ld+json block. A minority of
# postings ship this way (~1 in 34 sampled across tenants) while rendering the
# normal layout, so the parser falls back to the page's own markup.
GET https://en-gage.net/aspark-tokyo/work_17068421/
user-agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36
accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8

HTTP 200
[Asserts]
header "Content-Type" contains "text/html"
xpath "count(//script[@type='application/ld+json'])" == 0
xpath "count(//h1)" == 1
xpath "count(//dl[@class='dataSet']/h2[@class='item'])" > 0
