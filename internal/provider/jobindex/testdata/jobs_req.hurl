# Jobindex search: keyword backend, last 14 days.
GET https://www.jobindex.dk/jobsoegning?q=backend&jobage=14&page=1
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36
Accept: text/html,application/xhtml+xml
Accept-Language: da,en;q=0.9
HTTP 200
[Asserts]
body contains "var Stash ="
body contains "searchResponse"
body contains "hitcount"
