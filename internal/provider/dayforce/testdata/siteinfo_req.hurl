# Not part of openapi.yaml: this is the SSR board page, hand-parsed by
# siteinfo.go for the jobBoardId that never appears in the human-facing URL.
GET https://jobs.dayforcehcm.com/en-US/pca/CANDIDATEPORTAL
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36
Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8

HTTP 200
[Asserts]
header "Content-Type" contains "text/html"
body contains "__NEXT_DATA__"
body contains "\"site-info\""
body contains "\"jobBoardId\":1"
