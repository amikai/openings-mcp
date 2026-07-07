# Job ID: 21826 (R&D Advanced Packaging Integration Engineer, Taiwan). Query
# param style detail page (see
# ../openapi.yaml paths./zh_TW/careers/JobDetail.get), with source filled in.
# http1.1 is forced because hurl's default HTTP/2 ALPN offer trips
# Cloudflare's bot challenge on this host (plain curl, which doesn't offer
# h2 by default, passes).
GET https://careers.tsmc.com/zh_TW/careers/JobDetail
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36
Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8
Accept-Language: zh-TW,zh;q=0.9,en;q=0.8
Referer: https://careers.tsmc.com/zh_TW/careers/SearchJobs
[Options]
http1.1: true
output: job_detail_rsp.html
[QueryStringParams]
jobId: 21826
source: External+Career+Site

HTTP 200
