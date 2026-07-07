# https://github.com/amikai/openings-mcp/issues/94 — when the keyword is one
# 104 recognizes as a company name (e.g. 聯發科), the API abandons the job
# search and replies with this pagination-less companyKeyword shape unless
# excludeCompanyKeyword=true is sent. excludeCompanyKeyword is deliberately
# omitted here to reproduce that bug — see jobs_req.hurl for the normal,
# fully-parameterized search.
GET https://www.104.com.tw/jobs/search/api/jobs
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36
Accept: application/json, text/plain, */*
Accept-Language: zh-TW,zh;q=0.9,en-US;q=0.8,en;q=0.7
Referer: https://www.104.com.tw/jobs/search/
[Options]
http1.1: true
output: issue_94_rsp.json
[QueryStringParams]
keyword: 聯發科
area: 6001006000

HTTP 200
