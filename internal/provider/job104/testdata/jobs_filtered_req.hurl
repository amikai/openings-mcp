# HTTP/1.1 avoids Cloudflare's HTTP/2 bot challenge.
GET https://www.104.com.tw/jobs/search/api/jobs
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36
Accept: application/json, text/plain, */*
Accept-Language: zh-TW,zh;q=0.9,en-US;q=0.8,en;q=0.7
Referer: https://www.104.com.tw/jobs/search/
[Options]
http1.1: true
[QueryStringParams]
keyword: Golang
excludeCompanyKeyword: true # without it, a keyword 104 recognizes as a company name gets the pagination-less companyKeyword response instead of a job search (issue #94; see issue_94_req.hurl)
area: 6001001000
ro: 1
order: 2
page: 1
edu: 4,5
jobexp: 1,3 # comma-joined brackets union like edu; any value outside 1/3/5/10/99 is a 422 (issue #98)
remoteWork: 1
s9: 1,2

HTTP 200
