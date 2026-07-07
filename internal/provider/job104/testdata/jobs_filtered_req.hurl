# Demonstrates the full documented query parameter surface (see
# ../openapi.yaml paths./jobs/search/api/jobs.get) — see jobs_req.hurl for
# the exact minimal request that captured jobs_rsp.json. Not wired to any
# committed fixture; illustrative only. http1.1 is forced because hurl's
# default HTTP/2 ALPN offer trips Cloudflare's bot challenge on this host
# (plain curl, which doesn't offer h2 by default, passes).
GET https://www.104.com.tw/jobs/search/api/jobs
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36
Accept: application/json, text/plain, */*
Accept-Language: zh-TW,zh;q=0.9,en-US;q=0.8,en;q=0.7
Referer: https://www.104.com.tw/jobs/search/
[Options]
http1.1: true
[QueryStringParams]
keyword: Golang
excludeCompanyKeyword: true
area: 6001001000
ro: 1
order: 2
page: 1
edu: 4,5
remoteWork: 1
s9: 1,2

HTTP 200
