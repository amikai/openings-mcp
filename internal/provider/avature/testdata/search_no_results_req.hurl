GET https://bloomberg.avature.net/careers/SearchJobs/
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36
Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8
Accept-Language: en-US,en;q=0.9
[QueryStringParams]
search: zzzznonexistentkeyword12345

HTTP 200
[Asserts]
header "Content-Type" contains "text/html"
xpath "//a[contains(@href,'/JobDetail/')]" count == 0
