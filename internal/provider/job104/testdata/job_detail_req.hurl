# Job code taken from a search result's link.job URL trailing segment
# (see ../openapi.yaml paths./job/ajax/content/{jobCode}.get). http1.1 is
# forced because hurl's default HTTP/2 ALPN offer trips Cloudflare's bot
# challenge on this host (plain curl, which doesn't offer h2 by default,
# passes).
GET https://www.104.com.tw/job/ajax/content/8uz4j
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36
Accept: application/json, text/plain, */*
Accept-Language: zh-TW,zh;q=0.9,en-US;q=0.8,en;q=0.7
Referer: https://www.104.com.tw/job/8uz4j
[Options]
http1.1: true
output: job_detail_rsp.json

HTTP 200
