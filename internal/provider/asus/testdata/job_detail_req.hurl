GET https://recruit.asus.com/Jobs/Detail?sn=762c08de-1daa-4aa8-9668-d8a746ce24a8
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36
Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8

HTTP 200
[Asserts]
header "Content-Type" contains "text/html"
xpath "//div[contains(@class, 'Detail-Title')]/h1" exists
