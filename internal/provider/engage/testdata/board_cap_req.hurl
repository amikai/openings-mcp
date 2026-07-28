# Single-category board at exactly the 100/category cap.
GET https://en-gage.net/cookbiz_jobs/
user-agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36
accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8

HTTP 200
[Asserts]
header "Content-Type" contains "text/html"
xpath "count(//dl[@class='jobList'])" == 1
xpath "count(//dd[@class='dataArea'])" == 100
