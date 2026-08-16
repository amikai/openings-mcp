GET https://recruit.asus.com/Jobs
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36
Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8
[QueryStringParams]
Keyword: xyz999foobar

HTTP 200
[Asserts]
header "Content-Type" contains "text/html"
xpath "//table[contains(@class, 'Rwd-Table')]" not exists
