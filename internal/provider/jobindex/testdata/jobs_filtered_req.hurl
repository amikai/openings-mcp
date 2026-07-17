# Jobindex search: python, last 7 days, sort by date.
GET https://www.jobindex.dk/jobsoegning?q=python&jobage=7&page=1&sort=date
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36
Accept: text/html,application/xhtml+xml
Accept-Language: da,en;q=0.9
HTTP 200
[Asserts]
body contains "var Stash ="
body contains "searchResponse"
