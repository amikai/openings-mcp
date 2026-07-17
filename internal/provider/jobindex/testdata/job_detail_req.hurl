# Jobindex detail page stays on-site (unlike /jobannonce/ which may 302 off-site).
GET https://www.jobindex.dk/vis-job/h1683131
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36
Accept: text/html,application/xhtml+xml
Accept-Language: da,en;q=0.9
HTTP 200
[Asserts]
body contains "Senior Backend Engineer"
body contains "Whiteaway Group"
body contains "og:title"
