# Search: engineer keyword, with every documented filter field filled in —
# organization=TSMC Group (4177=5410262), Taiwan (1277=13209), R&D
# (558=38617), Engineer/Admin (147=5709), Regular (542=5701) — see
# ../openapi.yaml paths./zh_TW/careers/SearchJobs/{keyword}.get and
# components.parameters. http1.1 is forced because hurl's default HTTP/2
# ALPN offer trips Cloudflare's bot challenge on this host (plain curl,
# which doesn't offer h2 by default, passes).
GET https://careers.tsmc.com/zh_TW/careers/SearchJobs/engineer
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36
Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8
Accept-Language: zh-TW,zh;q=0.9,en;q=0.8
[Options]
http1.1: true
output: jobs_rsp.html
[QueryStringParams]
listFilterMode: 1
jobRecordsPerPage: 10
jobOffset: 0
4177: 5410262
1277: 13209
558: 38617
147: 5709
542: 5701
jobSort:
jobSortDirection:
source: External+Career+Site

HTTP 200
