# Multi-category board: nova_career carries 中途採用 at the 100/category cap
# plus アルバイト・パート採用 with 7 jobs (see CategoryCap in doc.go). Each
# category renders as its own dl.jobList (see doc.go), so two categories
# means two sibling dl.jobList elements, not one dl with two dt.category
# children.
GET https://en-gage.net/nova_career/
user-agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36
accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8

HTTP 200
[Asserts]
header "Content-Type" contains "text/html"
xpath "count(//dl[@class='jobList'])" == 2
xpath "count(//dd[@class='dataArea'])" == 107
xpath "count(//dt[@class='category'])" == 2
