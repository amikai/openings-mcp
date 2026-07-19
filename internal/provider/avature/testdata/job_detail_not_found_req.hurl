GET https://bloomberg.avature.net/careers/JobDetail/Nonexistent-Job/999999999
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36
Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8
Accept-Language: en-US,en;q=0.9

HTTP 302
[Asserts]
header "Location" contains "/careers/Error"
